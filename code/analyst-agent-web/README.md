# ResearchAgent — frontend (React + Vite)

The web UI for ResearchAgent. Type a company name or stock ticker; the page
asks the Go backend (`../analyst-agent-go`) to research it and shows the
resulting six-section investment brief, plus a log of what the agent did.

Built with React 19, TypeScript, React Router and Tailwind CSS v4, bundled
by Vite. Layout is responsive from 320px phones up to desktop.

## Quick start

Requires Node 20 or newer. The backend needs to be running too (see its
README) — by default at `http://localhost:8080`.

```bash
npm install
npm run dev        # -> http://localhost:5173
```

Open the page, enter a company (`Apple`, `TCS`, `INFY.NS`, `AAPL`…) and wait
for the report. A run takes up to about a minute; a timer shows progress.

## Scripts

| Command | What it does |
| --- | --- |
| `npm run dev` | Dev server with hot reload on port 5173. |
| `npm run build` | Type-checks (`tsc -b`) then builds to `dist/`. Run this before pushing. |
| `npm run preview` | Serves the built `dist/` locally. |

## Configuration

One setting, `VITE_API_URL`: the URL of the backend. See `.env.example`.

```bash
cp .env.example .env     # only needed if the backend isn't on localhost:8080
```

> **It is read at build time.** Vite bakes `VITE_*` values into the
> JavaScript when it builds, so on a deployed site, changing the variable has
> no effect until the site is rebuilt and redeployed.

The backend must also allow this site: set its `ALLOWED_ORIGIN` to the
frontend's URL, or the browser will block the requests (the UI then shows
"Could not reach the backend").

## How it works

```
src/
  main.tsx               entry; wraps the app in an error boundary
  App.tsx                routes: / (home), /report/:company, anything else -> /
  pages/Home.tsx         search box and example tickers
  pages/Report.tsx       runs the agent, shows progress, report or error
  components/
    ReportCard.tsx       the finished brief (+ copy to clipboard)
    AgentThinking.tsx    the step-by-step research log
    ToolCallBadge.tsx    label for a tool call (web search, financials, news)
    ErrorBoundary.tsx    recovery screen if rendering ever throws
  lib/api.ts             the only place that talks to the backend
  types/index.ts         data shapes, mirroring the backend's internal/types
```

The backend answers `POST /api/agent` once, when the whole run finishes, so
the page shows a spinner and timer rather than live steps. `lib/api.ts`
turns failures into plain-language messages: backend unreachable, still
waking up (502/503/504), busy (503 with a message), rejected input (400), or
timed out after 5 minutes. "Try again" re-runs the request without a reload.

The home page sends a small request to the backend's `/healthz` as soon as
it loads, so a sleeping host starts waking while the user is still typing.

If you change a data shape on the backend, change `src/types/index.ts` to
match.

## Deploying to Render

The repository's `render.yaml` defines this as a **Static Site**: build
command `npm ci && npm run build`, publish directory `dist`. Two things must
be set in the dashboard:

1. `VITE_API_URL` — your deployed backend's URL (no trailing slash).
2. Nothing else for routing: the blueprint already adds the rewrite
   `/* -> /index.html`, which is what makes a refresh on `/report/AAPL` work
   instead of returning a 404.

See the top-level `README.md` for the full two-service walkthrough.

## Troubleshooting

- **"Could not reach the backend"** — the backend isn't running, the URL in
  `VITE_API_URL` is wrong (rebuild after fixing it), or the backend's
  `ALLOWED_ORIGIN` doesn't match this site's URL exactly.
- **First request on the deployed site is slow** — free Render services
  sleep when idle and need about a minute to wake. The page says so after
  20 seconds.
- **Blank page after deploying** — open the browser console. A 404 on
  `/assets/...` usually means the wrong publish directory (should be `dist`).
- **Refreshing a report page gives 404** — the SPA rewrite rule is missing.
