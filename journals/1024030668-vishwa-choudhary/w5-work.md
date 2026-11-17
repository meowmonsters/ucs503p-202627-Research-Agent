# Week 5 : Financials Bug-Fix Pass

This week, I worked on fixing problems in the financial data lookup that
showed up once the whole system started running together.

## What I did

- Fixed the order in which financial sources are used for Indian stocks.
  The price and daily change now come from Twelve Data or Yahoo Finance,
  and Screener.in is only used to add the extra details (market cap, P/E
  ratio, dividend yield, revenue) that those sources don't provide. Before
  this, setting a Twelve Data key meant market cap showed as "N/A" for
  Indian stocks, even though Screener.in had it.
- Stopped Screener.in results from reporting a daily change of exactly
  zero. That page doesn't have a daily change, so it is now left as
  unknown instead of claiming the price didn't move. The same fix applies
  to P/E ratio and dividend yield when they are missing.
- Turned off the hard-coded sample figures by default. They were being
  returned as if they were real, live data when every source failed. They
  now only appear if `ALLOW_SAMPLE_DATA=1` is set, and they are clearly
  labelled as sample data.
- Added a `source` field to every financial result, so it's clear where
  the numbers came from (for example "Yahoo Finance + Screener.in").
- Wrote tests for the financials lookup using fake data sources, to check
  the order they are tried in, that existing values are never overwritten,
  and that a failure is reported as a failure instead of invented data.
- Ran the tests along with the rest of the backend tests, and they pass.

I have not yet tested this against the live Yahoo, Twelve Data, or
Screener.in sources, because they were not reachable from where I ran the
tests. The ordering and fallback logic is tested, but the real responses
from those sites still need to be checked.
