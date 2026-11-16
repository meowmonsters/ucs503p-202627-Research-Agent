package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// This file turns whatever the user (or the LLM) typed into a ticker the
// data sources understand. Inputs seen in practice:
//
//	AAPL, aapl, $AAPL      plain tickers, any case, cashtags
//	BRK-B, BAJAJ-AUTO.NS   tickers containing "-" (the old cleaner deleted it)
//	M&M.NS                 tickers containing "&"
//	^GSPC, EURUSD=X        index and currency symbols
//	NASDAQ:AAPL, NSE:TCS   "EXCHANGE:SYMBOL" as models like to write them
//	Apple Inc., Nvidia     company names, with or without a legal suffix
//	Nestlé, 株式会社        non-ASCII names (must never be "cleaned" into junk)

// knownTickers maps common company names (normalised by normalizeName) to
// their ticker, so "Infosys" or "Berkshire Hathaway" resolve without a
// network lookup. Only entries that are unambiguous are listed; anything
// else goes through the symbol search below.
var knownTickers = map[string]string{
	// India
	"infosys": "INFY.NS", "tcs": "TCS.NS", "tata consultancy services": "TCS.NS",
	"reliance": "RELIANCE.NS", "reliance industries": "RELIANCE.NS",
	"wipro": "WIPRO.NS", "hcl": "HCLTECH.NS", "hcl technologies": "HCLTECH.NS",
	"hdfc bank": "HDFCBANK.NS", "icici bank": "ICICIBANK.NS",
	"sbi": "SBIN.NS", "state bank of india": "SBIN.NS",
	"itc": "ITC.NS", "bharti airtel": "BHARTIARTL.NS", "airtel": "BHARTIARTL.NS",
	"larsen & toubro": "LT.NS", "larsen and toubro": "LT.NS", "l&t": "LT.NS",
	"asian paints": "ASIANPAINT.NS", "hindustan unilever": "HINDUNILVR.NS",
	"adani enterprises": "ADANIENT.NS", "mahindra & mahindra": "M&M.NS",
	// US / global
	"apple": "AAPL", "tesla": "TSLA", "microsoft": "MSFT", "google": "GOOGL",
	"alphabet": "GOOGL", "amazon": "AMZN", "nvidia": "NVDA", "meta": "META",
	"meta platforms": "META", "facebook": "META", "netflix": "NFLX",
	"visa": "V", "ford": "F", "target": "TGT", "coca cola": "KO", "coca-cola": "KO",
	"berkshire hathaway": "BRK-B", "johnson & johnson": "JNJ", "jpmorgan": "JPM",
	"jpmorgan chase": "JPM", "walmart": "WMT", "disney": "DIS", "walt disney": "DIS",
}

// nameSuffixes are legal-form words dropped before a name lookup, so
// "Apple Inc." and "Tata Consultancy Services Ltd" match their entries.
var nameSuffixes = map[string]bool{
	"inc": true, "incorporated": true, "corp": true, "corporation": true,
	"ltd": true, "limited": true, "plc": true, "co": true, "company": true,
	"llc": true, "nv": true, "ag": true,
}

// normalizeName lower-cases, drops punctuation that varies ("Inc." vs
// "Inc"), collapses whitespace and strips trailing legal-form words.
func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(".", " ", ",", " ", "’", "'").Replace(s)
	words := strings.Fields(s)
	for len(words) > 1 && nameSuffixes[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// tickerShapeRe matches strings that look like a ticker rather than a
// name: ASCII, no spaces, an optional leading ^, and up to two ".", "-" or
// "=" suffix parts (BRK-B, INFY.NS, BAJAJ-AUTO.NS, EURUSD=X).
var tickerShapeRe = regexp.MustCompile(`^\^?[A-Za-z0-9&]{1,12}([.\-=][A-Za-z0-9&]{1,10}){0,2}$`)

// exchangeSuffix maps "EXCHANGE:SYMBOL" prefixes onto Yahoo-style suffixes.
var exchangeSuffix = map[string]string{
	"NSE": ".NS", "BSE": ".BO",
	"NASDAQ": "", "NYSE": "", "AMEX": "", "NYSEARCA": "",
}

// resolvedTicker is the outcome of interpreting the raw input.
type resolvedTicker struct {
	Symbol string // upper-case candidate symbol to try directly ("" if none)
	Known  bool   // came from the known-company table
}

// interpretTicker decides what, if anything, the input can be used as a
// ticker without a network lookup.
func interpretTicker(raw string) resolvedTicker {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "$")

	// "Apple Inc. (AAPL)" -> the part in brackets is the ticker.
	if open := strings.LastIndex(s, "("); open >= 0 && strings.HasSuffix(s, ")") {
		if inner := strings.TrimSpace(s[open+1 : len(s)-1]); tickerShapeRe.MatchString(inner) {
			s = inner
		}
	}

	if mapped, ok := knownTickers[normalizeName(s)]; ok {
		return resolvedTicker{Symbol: mapped, Known: true}
	}

	// "NASDAQ:AAPL" / "NSE:TCS"
	if i := strings.Index(s, ":"); i > 0 {
		if suffix, ok := exchangeSuffix[strings.ToUpper(strings.TrimSpace(s[:i]))]; ok {
			sym := strings.ToUpper(strings.TrimSpace(s[i+1:]))
			if tickerShapeRe.MatchString(sym) {
				if suffix != "" && !strings.Contains(sym, ".") {
					sym += suffix
				}
				return resolvedTicker{Symbol: sym}
			}
		}
	}

	if tickerShapeRe.MatchString(s) {
		return resolvedTicker{Symbol: strings.ToUpper(s)}
	}
	return resolvedTicker{}
}

// --- Name -> symbol lookup (Yahoo Finance search) ---

// lookupSymbol is a variable so tests never touch the network.
var lookupSymbol = lookupYahooSymbol

type yahooSearchResponse struct {
	Quotes []struct {
		Symbol    string `json:"symbol"`
		QuoteType string `json:"quoteType"`
	} `json:"quotes"`
}

// lookupYahooSymbol searches Yahoo Finance for a company name and returns
// the symbol of the best equity match, or an error if there is none.
func lookupYahooSymbol(name string) (string, error) {
	endpoint := "https://query2.finance.yahoo.com/v1/finance/search?quotesCount=6&newsCount=0&q=" + url.QueryEscape(name)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("yahoo symbol search returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return pickSymbol(body, name)
}

// pickSymbol chooses the first listed equity from a search response. ETFs,
// funds, futures and currencies are skipped: someone typing a company name
// wants the company, not a fund that holds it.
func pickSymbol(body []byte, name string) (string, error) {
	var parsed yahooSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse yahoo symbol search: %w", err)
	}
	for _, q := range parsed.Quotes {
		if strings.EqualFold(q.QuoteType, "EQUITY") && q.Symbol != "" && tickerShapeRe.MatchString(q.Symbol) {
			return strings.ToUpper(q.Symbol), nil
		}
	}
	return "", fmt.Errorf("no listed company found matching %q", name)
}
