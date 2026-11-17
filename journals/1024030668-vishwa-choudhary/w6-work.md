# Week 6 : Testing Across Companies and Fixing Edge Cases

This week, I worked on testing how the backend handles many different
company names and ticker symbols, and fixing the problems that turned up.

## What I did

- Wrote a test set of 39 company names and tickers, typed the way a real
  person might type them (different capitalisation, "Ltd" and "Limited",
  "&", full legal names). It covers large and mid-sized Indian companies
  and a few US ones, and each input has the symbol it should turn into.
- Found real problems by testing symbols that contain special characters.
  `M&M.NS` and `BAJAJ-AUTO.NS` were losing their `&` and `-` and turning
  into wrong symbols, and `L&T` was turning into `LT`. I changed the
  cleaning so these characters are kept.
- Found that input made only of punctuation (like `$$$`) sent an empty
  symbol to the data sources. It is now rejected with a clear message
  before anything is looked up.
- Added a company-name resolver (`internal/tools/resolve.go`). It matches
  names like "Larsen & Toubro Ltd." or "Dr. Reddy's Laboratories" against a
  built-in list, ignoring capitalisation, punctuation, "Ltd"/"Inc" endings
  and a leading "The". Before this, names with spaces were just squashed
  together into something that wasn't a real symbol.
- Fixed smaller cases the tests showed up: legal names that include
  "India" (like Maruti Suzuki India Limited) weren't matching, US class
  shares like `BRK.B` need to be sent as `BRK-B`, and symbols are now
  escaped properly when they are put into a web address.
- Made the server log why a data source failed, so problems are easier to
  find later without showing technical errors to users.
- All 39 test inputs now turn into the expected symbol, and the whole
  backend test suite passes.

This test checks that names turn into the right symbols. It does not check
that Yahoo Finance, Screener.in, or Twelve Data then return data for those
symbols, because those sites were not reachable from where I ran the tests.
That still needs to be checked with real network access. The company list
is also fixed, so a company that isn't on it is cleaned up and tried as a
ticker, and symbols can change over time when companies are renamed or
merge.
