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
