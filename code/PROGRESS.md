# Progress Log

## Week 1 — Scaffolding

**Goal:** get both projects initialized and running, no real functionality yet.

- Initialized Go module and a minimal HTTP server with a `/healthz` route.
- Initialized the Vite + React + TypeScript project.
- Defined the first shared data types (`SearchResult`, `FinancialData`,
  `NewsItem`) that the backend tools will return and the frontend will
  consume — no logic yet, just the shapes.
- Confirmed `go run ./cmd/server` and `npm run dev` both start cleanly.

**Next up (Week 2):** first real data source — a web search tool on the
backend, wired through a dispatcher.

## Week 2 — First real data source + frontend types

**Goal:** one working tool end-to-end, frontend mirrors the data shape.

- Added the tools dispatcher (`internal/tools/tools.go`) and the first real
  tool, web search (`internal/tools/search.go`) — scrapes DuckDuckGo's HTML
  endpoint, with a Google News RSS fallback if that fails.
- Added `ToolName` and `ToolResult` to the shared types so every tool
  reports success/failure the same way.
- Added a basic Gemini API client (`internal/llm/llm.go`) — a single
  request/response call, no retries or backup provider yet.
- Added the matching frontend types (`src/types/index.ts`) and an API
  client skeleton (`src/lib/api.ts`) — not wired into the UI yet, since the
  backend doesn't expose an HTTP route for search yet either (still just
  `/healthz`).
- Confirmed the search tool returns real results when called directly;
  `npm run dev` still shows the same placeholder page as Week 1.

**Next up (Week 3):** the news and financials tools, plus an OpenRouter
fallback for the LLM client.

## Week 3 — News, financials, OpenRouter fallback, and the landing page

**Goal:** round out the tool set, harden the LLM client, and get a real
page on screen.

- Added the news tool (`internal/tools/news.go`) — Google News RSS only
  for now.
- Added the financials tool (`internal/tools/financials.go`) — first pass:
  Yahoo Finance's public chart endpoint, with a Screener.in scrape as a
  fallback for Indian tickers, and a small hard-coded mock-data safety net
  for two well-known tickers if both live sources fail.
- Extended `FinancialData` with the fields the financials tool actually
  needs (market cap, P/E, dividend yield, revenue), and wired both new
  tools into the dispatcher.
- Added an OpenRouter fallback to the LLM client — if the direct Gemini
  call fails and `OPENROUTER_API_KEY` is set, it retries once through
  OpenRouter before giving up.
- Added the real landing page (`src/pages/Home.tsx`), Tailwind v4 styling
  (`src/index.css`), and routing (`src/App.tsx`) — the search box doesn't
  do anything live yet, since it navigates to a `/report/:company` route
  that doesn't exist until the report page is built.

**Next up (Week 4):** the agent loop itself, plus Twelve Data/Exa as
optional extra data sources.

## Week 4 — The agent loop, optional data sources, and agent UI

**Goal:** the research loop actually runs end-to-end, with a safety net if
the LLM is unavailable.

- Added the agent loop (`internal/agent/agent.go`) — plan, call tools,
  reflect, and synthesize a final report, driven by Gemini. If the LLM
  fails at any point (missing credentials, rate limited, etc.), it falls
  back to a deterministic report assembled directly from the tools' raw
  data instead of failing outright.
- Added the agent-loop types to the shared types file (`ToolCall`,
  `AgentStep`, `ResearchReport`, `AgentMessage`) — these were deliberately
  left out until now, since nothing needed them before the agent existed.
- Added Twelve Data (`internal/tools/twelvedata.go`) as an optional,
  preferred financial data source, and Exa (`internal/tools/exa.go`) as an
  optional, preferred search/news source — both fall through to the
  existing chains if unconfigured or if they fail.
- Hardened the Yahoo Finance and Twelve Data paths: a "successful" response
  with a zero price now counts as a failure, so the fallback chain
  continues instead of returning a useless empty quote.
- Added the agent status UI (`src/components/AgentThinking.tsx`,
  `src/components/ToolCallBadge.tsx`) and brought the frontend's shared
  types back in sync with everything the backend has added since Week 2.
  Neither component is wired into a page yet — that's part of building the
  report page next week.
- Ran the full agent loop end-to-end locally: confirmed the deterministic
  fallback path works correctly when Gemini is unavailable, calling all
  three tools in sequence and producing a real structured report.

**Next up (Week 5):** the report page, HTTP routes for the agent, and
connecting the frontend to the real backend.

## Week 5 — Wiring it together

**Goal:** the first real end-to-end path — type a company in the browser,
get a report back from the Go backend.

- **HTTP API** (`internal/httpapi/handlers.go`, `cmd/server/main.go`):
  `POST /api/agent` plus direct `POST /api/tools/{search,financials,news}`
  routes. CORS answers browser preflight requests itself (so they never
  reach the handlers) and the allowed origin is configurable via
  `ALLOWED_ORIGIN`. Inputs are validated (400 on blank, wrong type or over
  100 characters) and the server has explicit timeouts.
- **Report page** (`src/pages/Report.tsx`, `src/components/ReportCard.tsx`)
  and `/report/:company` routing. `src/lib/api.ts` is now the real client
  (the Week 2 stub is gone) with readable errors for "server unreachable"
  and for validation failures. A visible timer runs while waiting, the
  research log is available under the finished report, and the report
  carries an informational-not-financial-advice note.
- **Financials fixes** (found while tracing real requests through the
  chain):
  - Indian tickers: price and day-change now come from Twelve Data/Yahoo and
    Screener.in only *adds* the fundamentals (market cap, P/E, revenue).
    Before, a Twelve Data key made market cap show "N/A" for Indian stocks,
    and Screener-first meant Yahoo's real day-change was never used.
  - Screener.in has no day-change, so it is now left unknown instead of
    being reported as exactly 0. Missing P/E / dividend yield are also
    unknown rather than 0.
  - The hard-coded sample figures were being returned as if live. They are
    now off by default (`ALLOW_SAMPLE_DATA=1` to enable) and labelled when
    used. Every result now carries a `source`.
- **Agent fixes** (found by running it through the API):
  - The report now has all six sections from the proposal (Week 4 only
    asked the model for five; "Competitive Landscape" was missing).
  - Tool errors are passed back to the model (it was being told `null`),
    and an empty result no longer counts as "data gathered".
  - The model can't write a report until a tool has returned real data;
    if none has, the report is built from what was retrieved and every
    unsupported section says so rather than guessing.
- **Tests:** 25 Go unit tests (handlers, CORS, input validation, financials
  ordering, agent report shape) and `scripts/integration_test.sh`, which
  starts the real server and checks it over HTTP.

**Integration run:** the integration script passes 12/12; the real
`api.ts` client was run against the live server (report with six sections,
readable errors for a blank company and for a stopped server). Run with no
network access and no Gemini key, so the agent took its fallback path.
**Not yet verified:** a full run with a real Gemini key and live data
sources, and the pages in an actual browser.

**Known limitation:** `/api/agent` is synchronous — the browser sees
nothing until the whole run finishes. Streaming the steps as they happen
is the natural next improvement.

**Next up (Week 6):** documentation, responsive polish, deployment.

## Week 6 — Polish, docs, deploy

**Goal:** submission-ready.

- **Docs** (Garv, Ananya): a README for each service (setup, configuration,
  API and error reference, troubleshooting), fuller `.env.example` files,
  and a top-level README with the local run, checks, deployment walkthrough
  and known limitations.
- **Backend hardening** (Garv):
  - `ALLOWED_ORIGIN` takes a comma-separated list and ignores trailing
    slashes (a trailing slash silently blocked every browser request).
  - At most `MAX_CONCURRENT_RUNS` research runs at once; extras get a quick
    503 + `Retry-After`.
  - A run stops researching after `RUN_BUDGET_SECONDS` and writes up what it
    has; graceful shutdown on SIGTERM; JSON 500 on any handler panic.
  - The Gemini key moved from the URL to a header so it can't leak through
    logged network errors.
- **Frontend polish** (Ananya): checked in a real browser at 320 / 375 / 768 /
  1280 px — no horizontal overflow; stacked search box and smaller type on
  phones; plain-language messages for a sleeping/starting backend, a busy
  server and timeouts; "Try again" no longer reloads the page; the backend is
  warmed up while the user types; error boundary; page titles and favicon.
- **Edge cases** (Vishwa): tickers with `-`, `&`, `^`, `=` were being
  mangled (`BRK-B` was looked up as `BRKB`); `NASDAQ:AAPL` / `NSE:TCS` forms;
  company names outside the small built-in table are now resolved through
  Yahoo's symbol search (and labelled as a name match); non-ASCII names are no
  longer turned into junk tickers; a missing previous close no longer reports
  the whole price as the day's change; symbol-only input is rejected; control
  characters are stripped before reaching prompts and logs; multi-byte-safe
  truncation; sub-dollar prices keep their decimals.
- **Release** (everyone): `render.yaml` Blueprint for both services,
  deployment steps in the top-level README, version 1.0.0.

**Verification:** 48 Go unit tests (25 at the end of Week 5) and 15 integration
checks pass; the frontend builds from a clean `npm ci`. The new tests were
checked to fail against the old behaviour.

**Not verified:**
- The deployed services — the Render deploy and the `v1.0.0` tag still have to
  be done from the team's GitHub/Render accounts (steps in `README.md`).
- Live data. The build environment could not reach Yahoo Finance, Google
  News or DuckDuckGo, so the new name-to-ticker lookup is tested against
  fakes, not the real endpoint, and a full run with a real Gemini key and live
  data is still outstanding (as it was after Week 5). Try a few names
  (`Nvidia`, `Zomato`, `Nestlé`) against the deployed backend.

