package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"analystagent/internal/types"
)

type mockFinancial struct {
	price, change, changePercent, peRatio, dividendYield float64
	marketCap, revenue                                   string
}

// mockData is a last-resort fallback, used only if both live sources fail
// for one of these two well-known tickers.
var mockData = map[string]mockFinancial{
	"INFY.NS": {price: 1650.45, change: -12.30, changePercent: -0.74, marketCap: "6,85,000 Cr", peRatio: 25.4, revenue: "1,54,000 Cr", dividendYield: 2.15},
	"AAPL":    {price: 189.43, change: 1.25, changePercent: 0.66, marketCap: "2.93T", peRatio: 31.2, revenue: "383.29B", dividendYield: 0.51},
}

func ptr(f float64) *float64 { return &f }

// nonZero returns nil for zero, so "the page had no value" is reported as
// unknown rather than as a real zero.
func nonZero(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func commaFormat(f float64) string {
	whole := int64(f)
	s := strconv.FormatInt(whole, 10)
	var out strings.Builder
	n := len(s)
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}

// --- Screener.in scrape (for Indian tickers) ---

var salesRe = regexp.MustCompile(`(?i)Sales[\s\S]*?</button>[\s\S]*?</td>([\s\S]*?)</tr>`)
var tdRe = regexp.MustCompile(`<td.*?>([\d,.]+)</td>`)
var tdTagRe = regexp.MustCompile(`<[^>]+>`)

func extractMetric(html, name string) (float64, bool) {
	pattern := fmt.Sprintf(`(?i)%s[\s\S]*?<span class="number">([\d,.]+)`, regexp.QuoteMeta(name))
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func fetchScreenerData(ticker string) *types.FinancialData {
	symbol := strings.ToUpper(strings.SplitN(ticker, ".", 2)[0])
	targetURL := "https://www.screener.in/company/" + url.PathEscape(symbol) + "/"

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	htmlStr := string(bodyBytes)

	marketCapCr, hasMarketCap := extractMetric(htmlStr, "Market Cap")
	currentPrice, hasPrice := extractMetric(htmlStr, "Current Price")
	peRatio, _ := extractMetric(htmlStr, "Stock P/E")
	divYield, _ := extractMetric(htmlStr, "Dividend Yield")

	if !hasPrice && !hasMarketCap {
		return nil
	}

	var revenueCr float64
	if salesMatch := salesRe.FindStringSubmatch(htmlStr); salesMatch != nil {
		tdMatches := tdRe.FindAllStringSubmatch(salesMatch[1], -1)
		if len(tdMatches) > 0 {
			last := tdTagRe.ReplaceAllString(tdMatches[len(tdMatches)-1][1], "")
			last = strings.ReplaceAll(last, ",", "")
			if v, err := strconv.ParseFloat(last, 64); err == nil {
				revenueCr = v
			}
		}
	}

	marketCap := "N/A"
	if hasMarketCap {
		marketCap = commaFormat(marketCapCr) + " Cr"
	}
	revenue := "N/A"
	if revenueCr != 0 {
		revenue = commaFormat(revenueCr) + " Cr"
	}

	return &types.FinancialData{
		Ticker:      symbol,
		CompanyName: symbol,
		Currency:    "INR",
		// Screener.in's page has no day-change figure. Leave it unknown
		// (nil) instead of claiming a change of exactly zero.
		Price:         nonZero(currentPrice),
		MarketCap:     marketCap,
		PERatio:       nonZero(peRatio),
		DividendYield: nonZero(divYield),
		Revenue:       revenue,
		Source:        "Screener.in",
	}
}

// --- Yahoo Finance (unauthenticated chart endpoint) ---

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				PreviousClose      float64 `json:"previousClose"`
				LongName           string  `json:"longName"`
				ShortName          string  `json:"shortName"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func fetchYahooQuote(ticker string) (*types.FinancialData, error) {
	endpoint := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(ticker)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("yahoo finance returned status %d", resp.StatusCode)
	}

	return quoteFromYahooChart(bodyBytes, ticker)
}

// quoteFromYahooChart turns a Yahoo chart response body into a quote.
func quoteFromYahooChart(bodyBytes []byte, ticker string) (*types.FinancialData, error) {
	var parsed yahooChartResponse
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse yahoo finance response: %w", err)
	}
	if len(parsed.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data from yahoo finance for %s", ticker)
	}

	meta := parsed.Chart.Result[0].Meta
	if meta.RegularMarketPrice == 0 {
		// A "successful" response with no actual price isn't useful to a
		// caller — treat it as a failure so the orchestration below falls
		// through to the next source instead of returning an empty quote.
		return nil, fmt.Errorf("yahoo finance returned no price for %s", ticker)
	}

	prevClose := meta.PreviousClose
	if prevClose == 0 {
		prevClose = meta.ChartPreviousClose
	}
	// With no previous close the day's change is unknown. (It used to be
	// computed against zero, which reported the whole price as the change.)
	var change, changePercent *float64
	if prevClose != 0 {
		c := meta.RegularMarketPrice - prevClose
		change, changePercent = ptr(c), ptr(c/prevClose*100)
	}

	name := meta.LongName
	if name == "" {
		name = meta.ShortName
	}
	if name == "" {
		name = ticker
	}

	return &types.FinancialData{
		Ticker:        ticker,
		CompanyName:   name,
		Price:         ptr(meta.RegularMarketPrice),
		Change:        change,
		ChangePercent: changePercent,
		Currency:      meta.Currency,
		Source:        "Yahoo Finance",
	}, nil
}

// --- Orchestration ---

// The three live sources are variables so tests can swap in fakes and check
// the ordering without any network access.
var (
	fetchTwelveData = fetchTwelveDataQuote
	fetchYahoo      = fetchYahooQuote
	fetchScreener   = fetchScreenerData
)

func isMissing(s string) bool { return s == "" || s == "N/A" }

// hasFundamentals reports whether a quote already carries the
// fundamentals (market cap, P/E) that price-only sources don't provide.
func hasFundamentals(d *types.FinancialData) bool {
	return !isMissing(d.MarketCap) && d.PERatio != nil
}

// mergeFundamentals fills gaps in dst from src without overwriting
// anything dst already has, and records src as an additional source if it
// contributed anything.
func mergeFundamentals(dst, src *types.FinancialData) {
	merged := false
	if isMissing(dst.MarketCap) && !isMissing(src.MarketCap) {
		dst.MarketCap, merged = src.MarketCap, true
	}
	if dst.PERatio == nil && src.PERatio != nil {
		dst.PERatio, merged = src.PERatio, true
	}
	if dst.DividendYield == nil && src.DividendYield != nil {
		dst.DividendYield, merged = src.DividendYield, true
	}
	if isMissing(dst.Revenue) && !isMissing(src.Revenue) {
		dst.Revenue, merged = src.Revenue, true
	}
	if merged && src.Source != "" {
		dst.Source += " + " + src.Source
	}
}

// fetchLiveQuote returns a real-time price from the first source that
// works: Twelve Data (only if a key is configured), then Yahoo Finance.
func fetchLiveQuote(cleanTicker string) *types.FinancialData {
	if twelveDataAPIKey() != "" {
		if q, err := fetchTwelveData(cleanTicker); err == nil {
			return q
		}
	}
	if q, err := fetchYahoo(cleanTicker); err == nil {
		return q
	}
	return nil
}

// sampleDataAllowed gates the hard-coded demo figures. They are NOT live
// data, so they are off unless explicitly enabled (ALLOW_SAMPLE_DATA=1),
// which is only meant for offline demos.
func sampleDataAllowed() bool { return os.Getenv("ALLOW_SAMPLE_DATA") == "1" }

// Financials looks up financial data for a ticker or well-known company
// name. Order of preference:
//
//  1. A live price from Twelve Data (if keyed) or Yahoo Finance.
//  2. For Indian listings, Screener.in then fills in the fundamentals
//     (market cap, P/E, dividend yield, revenue) the price sources lack.
//  3. If no live price source works, Screener.in alone (price and
//     fundamentals, but no day-change figure).
//  4. Sample data, only if explicitly enabled.
//
// If nothing works it returns a failure, so the caller reports the data as
// missing instead of inventing it.
func Financials(rawTicker string) types.ToolResult {
	rawTicker = strings.TrimSpace(rawTicker)
	if rawTicker == "" {
		return types.ToolResult{Tool: types.ToolFinancials, Success: false, Data: nil, Error: "Invalid ticker identifier"}
	}

	// Work out what the input can be used as: a known company, a ticker, or
	// (symbol == "") only a name that has to be looked up.
	in := interpretTicker(rawTicker)
	symbol := in.Symbol

	var quote *types.FinancialData
	if symbol != "" {
		quote = fetchLiveQuote(symbol)
	}

	// The direct attempt failed (or the input was not ticker-shaped, e.g.
	// "Nvidia Corporation" or "Nestlé"): treat it as a company name and
	// search for its symbol. Skipped for the known-company table, whose
	// entries were chosen precisely so no guessing is needed.
	if quote == nil && !in.Known {
		if resolved, err := lookupSymbol(rawTicker); err == nil && resolved != symbol {
			if q := fetchLiveQuote(resolved); q != nil {
				// Say so, since a name match can be wrong (e.g. "Delta").
				q.Source += fmt.Sprintf(" (matched %q to %s)", rawTicker, resolved)
				quote, symbol = q, resolved
			}
		}
	}

	cleanTicker := symbol
	isIndian := strings.HasSuffix(cleanTicker, ".NS") || strings.HasSuffix(cleanTicker, ".BO")

	if quote != nil {
		if isIndian && !hasFundamentals(quote) {
			if s := fetchScreener(cleanTicker); s != nil {
				mergeFundamentals(quote, s)
			}
		}
	} else if cleanTicker != "" {
		// No live price source worked. Screener.in can still supply a price
		// (and works for Indian symbols typed without a suffix).
		quote = fetchScreener(cleanTicker)
	}

	if quote != nil {
		return types.ToolResult{Tool: types.ToolFinancials, Success: true, Data: *quote}
	}

	if mock, ok := mockData[cleanTicker]; ok && sampleDataAllowed() {
		currency := "USD"
		if strings.HasSuffix(cleanTicker, ".NS") {
			currency = "INR"
		}
		return types.ToolResult{
			Tool:    types.ToolFinancials,
			Success: true,
			Data: types.FinancialData{
				Ticker:        cleanTicker,
				CompanyName:   cleanTicker,
				Currency:      currency,
				Price:         ptr(mock.price),
				Change:        ptr(mock.change),
				ChangePercent: ptr(mock.changePercent),
				MarketCap:     mock.marketCap,
				PERatio:       ptr(mock.peRatio),
				DividendYield: ptr(mock.dividendYield),
				Revenue:       mock.revenue,
				Source:        "Sample data (not live)",
			},
		}
	}

	return types.ToolResult{
		Tool:    types.ToolFinancials,
		Success: false,
		Data:    nil,
		Error: fmt.Sprintf("No financial data found for %q. It may not be a listed company; "+
			"try its ticker symbol (for example AAPL, BRK-B or INFY.NS).", rawTicker),
	}
}
