# ResearchAgent

An autonomous research agent. Enter a company name or stock ticker and it
produces a structured investment brief — company overview, financial
snapshot, recent news, competitive landscape, risks and a summary — built
from live financial data, news and web search.

> Informational research aid only — **not financial advice.** Figures come
> from free public sources that can be delayed, incomplete or wrong. Anything
> the retrieved data can't support is marked "not available" instead of
> guessed.

```
 Browser ──► React app (analyst-agent-web) ──► Go API (analyst-agent-go) ──► Gemini (plans, writes)
                                                       │
                                                       └─► tools: financials · news · web search
```

| Folder | What it is | Docs |
| --- | --- | --- |
| `analyst-agent-go/` | Backend: agent loop, tools, HTTP API (Go, no dependencies) | [README](analyst-agent-go/README.md) |
| `analyst-agent-web/` | Frontend: React 19 + Vite + TypeScript + Tailwind | [README](analyst-agent-web/README.md) |
| `scripts/` | End-to-end smoke test for the backend | below |
| `render.yaml` | Render Blueprint that deploys both services | [Deploying](#deploying-to-render) |
| `PROGRESS.md` | Week-by-week build log | |

## Run it locally

You need Go 1.22+ and Node 20+. Two terminals:

```bash
# 1. backend  ->  http://localhost:8080
cd analyst-agent-go
cp .env.example .env              # add your GEMINI_API_KEY
set -a; source .env; set +a
go run ./cmd/server
```

```bash
# 2. frontend ->  http://localhost:5173
cd analyst-agent-web
npm install
npm run dev
```

Open http://localhost:5173, enter a company or ticker (`Apple`, `AAPL`,
`BRK-B`, `INFY.NS`, `Nvidia`…) and wait up to about a minute.

Without a `GEMINI_API_KEY` everything still runs; the report is then built
only from retrieved data, with the sections that need the AI model marked
unavailable.

## Checks

```bash
cd analyst-agent-go && go vet ./... && go test ./...   # unit tests, offline
./scripts/integration_test.sh                          # starts the server, calls it over HTTP
cd analyst-agent-web && npm run build                  # type-check + production build
```

## Deploying to Render

Both services deploy from this repo with the Blueprint in `render.yaml`: a
Go web service (`researchagent-api`) and a static site (`researchagent-web`).
Each needs the *other's* URL, so the order matters.

1. **Push this repo to GitHub** (Render deploys from a Git repository).
2. In the Render dashboard choose **New → Blueprint**, select the repo, and
   apply. Render prompts for the variables marked `sync: false`:
   - `GEMINI_API_KEY` — your Gemini key (leave the optional ones blank).
   - `ALLOWED_ORIGIN` — the frontend's URL. Render URLs are normally
     `https://<service-name>.onrender.com`, so enter
     `https://researchagent-web.onrender.com`. If Render gives either
     service a different name or suffix, you'll fix it in step 4.
   - `VITE_API_URL` — the backend's URL, normally
     `https://researchagent-api.onrender.com`.
3. Wait for both deploys. Check the backend first:
   `https://<api-url>/healthz` should show `ok`.
4. **Confirm the URLs match what Render actually assigned** (shown at the top
   of each service). If either was different:
   - fix `ALLOWED_ORIGIN` on the API service (it restarts automatically), and
   - fix `VITE_API_URL` on the web service, then **Manual Deploy → Deploy
     latest commit** — it is baked into the site at build time, so a
     restart is not enough.
5. Open the frontend URL and run a report (`AAPL`).

Notes for the free plan: services sleep when idle, so the first request after
a quiet period can take about a minute (the app warms the backend while you
type and explains the wait). To avoid it, use a paid instance for the API.

If it doesn't work:

| Symptom | Fix |
| --- | --- |
| "Could not reach the backend" | `VITE_API_URL` wrong (rebuild after fixing), or `ALLOWED_ORIGIN` not exactly the frontend URL |
| 404 when refreshing `/report/AAPL` | the SPA rewrite in `render.yaml` didn't apply — check Redirects/Rewrites on the static site |
| Reports say "needs the AI model" | `GEMINI_API_KEY` missing, invalid, or out of quota |
| Backend build fails on Go version | set a Go version Render supports ≥ 1.22 (`go.mod` requires 1.22) |

## Releasing

After the deploy works, tag the commit that is live:

```bash
git tag -a v1.0.0 -m "ResearchAgent v1.0"
git push origin v1.0.0
```

## Known limitations

- `/api/agent` is synchronous: the page shows a timer, not live steps, until
  the whole run finishes. Streaming the steps is the natural next improvement.
- Data comes from free sources, two of which are scraped (DuckDuckGo
  HTML, Screener.in) and can break when those sites change. Twelve Data and
  Exa keys make it sturdier.
- Looking a company up by name (rather than ticker) can pick the wrong
  listing for ambiguous names; the result says when it matched by name.
- Abuse protection is a global cap on simultaneous runs, not per-user rate
  limiting or authentication — anyone with the URL can use your API keys'
  quota, up to that cap.
