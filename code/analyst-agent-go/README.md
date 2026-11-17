# ResearchAgent — backend (Go)

The Go service behind ResearchAgent. Give it a company name or ticker and it
runs a small research agent: an LLM (Gemini) plans, calls tools for live
financials, news and web search, and writes a structured six-section
investment brief. If the LLM is unavailable it still returns a report, built
only from the data the tools retrieved.

Standard library only — there are no third-party Go dependencies.

## Quick start

Requires Go 1.22 or newer.

```bash
cp .env.example .env            # add your GEMINI_API_KEY
set -a; source .env; set +a     # load it into this shell
go run ./cmd/server             # -> http://localhost:8080
```

Check it is up: `curl localhost:8080/healthz` → `ok`.

Try a run (takes up to about a minute; the reply arrives when it finishes):

```bash
curl -s -X POST localhost:8080/api/agent \
  -H 'Content-Type: application/json' \
  -d '{"company":"Apple"}'
```

No API keys? It still works — you get the deterministic fallback report
(see [How the agent works](#how-the-agent-works)).

## Configuration

All configuration is by environment variable (see `.env.example`).

| Variable | Default | What it does |
| --- | --- | --- |
| `GEMINI_API_KEY` | — | Enables the AI-written report. Without it the fallback report is used. |
| `GEMINI_MODEL` | `gemini-3.5-flash-lite` | Gemini model to call. Models get retired — see Troubleshooting. |
| `OPENROUTER_API_KEY` | — | Backup LLM, tried once if the Gemini call fails. |
| `OPENROUTER_MODEL` | `google/gemini-3.5-flash-lite` | Model id used for the backup call. |
| `TWELVEDATA_API_KEY` | — | Preferred live-price source. |
| `EXA_API_KEY` | — | Preferred search/news source. |
| `PORT` | `8080` | Port to listen on (Render sets this for you). |
| `ALLOWED_ORIGIN` | any (`*`) | Frontend origin(s) allowed to call the API. Comma-separated; trailing slashes ignored. **Set this in production.** |
| `MAX_CONCURRENT_RUNS` | `4` | Max research runs at once; extra requests get `503`. |
| `RUN_BUDGET_SECONDS` | `90` | Time a run may spend researching before it writes up what it has. |
| `ALLOW_SAMPLE_DATA` | off | `1` allows labelled, hard-coded sample figures for AAPL / INFY.NS when all live sources fail. Demo only. |

The server logs a warning at startup if `GEMINI_API_KEY` or `ALLOWED_ORIGIN`
is missing. Never commit `.env` — it is in `.gitignore`.

## API

All request bodies are JSON; all responses are JSON (except `/healthz`).
Inputs are trimmed and limited to 100 characters.

| Method & path | Body | Purpose |
| --- | --- | --- |
| `GET /healthz` | — | Liveness check, returns `ok`. |
| `POST /api/agent` | `{"company": "..."}` | Run the full agent. |
| `POST /api/tools/search` | `{"query": "..."}` | Web search only. |
| `POST /api/tools/financials` | `{"ticker": "..."}` | Price and fundamentals only. |
| `POST /api/tools/news` | `{"company": "..."}` | Recent headlines only. |

### `POST /api/agent`

Returns every step the agent took, then the report:

```json
{
  "messages": [
    { "type": "step",   "step": { "id": 1, "type": "plan", "content": "Starting research on Apple...", "timestamp": 1760000000000 } },
    { "type": "report", "report": {
        "company": "Apple Inc.",
        "generatedAt": "2026-11-16T12:00:00Z",
        "overallSentiment": "Neutral",
        "sections": [ { "title": "Company Overview", "icon": "OVERVIEW", "content": "- point\n- point" } ]
    } },
    { "type": "done" }
  ]
}
```

The report always has six sections: Company Overview, Financial Snapshot,
Recent News, Competitive Landscape, Risk Factors, Investment Summary.
Anything the retrieved data can't support says so instead of guessing.
A run that fails outright ends with a message of `"type": "error"`.

### Errors

Errors are always `{"error": "readable message"}`.

| Status | When |
| --- | --- |
| `400` | Body is not JSON, the field is missing/blank/not a string, or it is over 100 characters. |
| `405` | Wrong HTTP method (these endpoints are POST-only). |
| `502` | A direct `/api/tools/*` call where the data source failed. |
| `503` | Too many runs in flight. Has a `Retry-After` header; retry shortly. |
| `500` | Unexpected internal error (logged server-side). |

## How the agent works

`internal/agent` runs a loop of at most 8 steps: the model states a plan,
picks a tool, reads the result, and decides whether it has enough to write.

- **Grounded:** the model may not write a report until a tool has returned
  real data, and is told to use only figures that appear in tool results.
- **Fallback:** if the model is unavailable, returns unparseable output, or
  no tool returns data, a deterministic report is built from whatever was
  retrieved. Sections that need judgment are marked "Not available…".
- **Time budget:** after `RUN_BUDGET_SECONDS` it stops researching and
  writes up what it has.

Data sources, in order of preference (`internal/tools`):

| Tool | Sources |
| --- | --- |
| financials | Twelve Data (if keyed) → Yahoo Finance → (Indian tickers) Screener.in for fundamentals → sample data only if `ALLOW_SAMPLE_DATA=1`. Every result carries a `source`. |
| news | Exa (if keyed) → Google News RSS |
| search | Exa (if keyed) → DuckDuckGo HTML → Google News RSS |

### What you can type

The financials lookup accepts more than plain tickers (`internal/tools/ticker.go`):

| Input | Handled as |
| --- | --- |
| `AAPL`, `aapl`, `$AAPL` | ticker |
| `BRK-B`, `BAJAJ-AUTO.NS`, `M&M.NS`, `^GSPC`, `EURUSD=X` | ticker, punctuation kept |
| `NASDAQ:AAPL`, `NSE:TCS` | exchange prefix removed (`NSE`/`BSE` become `.NS`/`.BO`) |
| `Apple Inc.`, `Berkshire Hathaway`, `Infosys` | a built-in table of well-known companies |
| `Nvidia Corporation`, `Nestlé`, `Zomato Limited` | symbol found through Yahoo Finance's search, then quoted |

A symbol found by name search is *labelled* in the result's `source`
(`matched "Zomato Limited" to ZOMATO.NS`), because a name match can be wrong
for ambiguous names such as "Delta". Typing the ticker avoids the guess.
Input containing only symbols (`???`) is rejected with a 400, and
newlines/control characters are replaced with spaces before the text reaches
a prompt or a log.

The DuckDuckGo and Screener.in paths scrape HTML, which can break when
those sites change their markup; that is why they are fallbacks.

## Project layout

```
cmd/server/          entry point: routes, timeouts, graceful shutdown
internal/httpapi/    handlers, input validation, CORS, concurrency cap
internal/agent/      the research loop and the fallback report
internal/llm/        Gemini client with OpenRouter backup
internal/tools/      search, news, financials (+ Twelve Data, Exa)
internal/types/      shared data shapes (mirrored in the frontend)
```

## Testing

```bash
go vet ./... && go test ./...      # unit tests, no network needed
../scripts/integration_test.sh     # builds and starts the server, calls it over HTTP
```

Unit tests swap the LLM, the tools and the data sources for fakes, so they
run offline. The integration script passes without any API keys.

## Deploying to Render

See the repository's top-level `README.md` — `render.yaml` defines this
service as a Go web service with `/healthz` as its health check. Set
`GEMINI_API_KEY` and `ALLOWED_ORIGIN` (your frontend's URL) in the
dashboard.

On Render's free plan the service sleeps when idle, so the first request
after a quiet period can take about a minute while it wakes up.

## Troubleshooting

- **Browser says "could not reach the backend" / CORS error** — set
  `ALLOWED_ORIGIN` to the exact frontend URL (scheme and host, no path).
- **Reports say "Not available… needs the AI model"** — the model wasn't
  used, so you are looking at the data-only report. The step log (and the
  server log) now says why, e.g. `Gemini API error (404): … is no longer
  available`. Usual causes: `GEMINI_API_KEY` missing or invalid; quota
  exhausted; or **the model has been retired** — Google shut down the
  `gemini-2.0-*` models on 1 June 2026. Set `GEMINI_MODEL` to a current model
  from <https://ai.google.dev/gemini-api/docs/deprecations> and restart.
- **Price is missing or marked "Sample data"** — every live source failed
  for that ticker. Try the exchange-suffixed form (`INFY.NS`, `RELIANCE.BO`).
- **"No financial data found for …"** — the name did not match a listed
  company. Use its ticker symbol.
