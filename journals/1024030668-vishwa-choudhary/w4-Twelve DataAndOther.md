# Week 4 : Twelve Data, Exa, and a Stronger Financials Fallback

This week, I worked on adding two optional data sources to the backend and
making the financial data lookup more reliable.

## What I did

- Added Twelve Data (`internal/tools/twelvedata.go`) as an optional source
  for financial data. It is only used if an API key is set, and it works
  for both US and Indian (NSE/BSE) stocks. I also added the conversion
  from our ticker format (like `RELIANCE.NS`) to the format Twelve Data
  expects (`RELIANCE:NSE`).
- Handled the fact that Twelve Data sends some numbers as text instead of
  numbers, so they are read correctly instead of causing an error.
- Added Exa (`internal/tools/exa.go`) as an optional source for web search
  and news. If an Exa key is set, the search and news tools try it first;
  if it is not set or returns nothing, they use DuckDuckGo and Google News
  as before.
- Made the financials fallback chain stronger in `financials.go`. Twelve
  Data is now tried first (when configured), followed by the existing
  Screener.in and Yahoo Finance sources. A response that comes back with no
  price at all is now treated as a failure, so the next source gets tried
  instead of returning an empty result.
- Added a small helper in `tools.go` that describes each tool in plain
  text, so the agent can tell the AI model which tools are available.
- Tested that the tools still work when no API keys are set, and that the
  fallback chain continues correctly when a source fails or is unavailable.

At this stage, both new sources are optional — the backend works the same
as before without them. They are not connected to the agent yet, since the
agent loop is being built separately this week.
