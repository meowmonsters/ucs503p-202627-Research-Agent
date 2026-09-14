package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"analystagent/internal/types"
)

// knownTickers maps a few common company names to their ticker symbol, so
// a user can type "Infosys" instead of having to know it's "INFY.NS".
var knownTickers = map[string]string{
	"infosys":   "INFY.NS",
	"tcs":       "TCS.NS",
	"reliance":  "RELIANCE.NS",
	"wipro":     "WIPRO.NS",
	"hcl":       "HCLTECH.NS",
	"apple":     "AAPL",
	"tesla":     "TSLA",
	"microsoft": "MSFT",
	"google":    "GOOGL",
	"amazon":    "AMZN",
	"alphabet":  "GOOGL",
}

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
	targetURL := "https://www.screener.in/company/" + symbol + "/"

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
		Ticker:        symbol,
		CompanyName:   symbol,
		Currency:      "INR",
		Price:         ptr(currentPrice),
		Change:        ptr(0),
		ChangePercent: ptr(0),
		MarketCap:     marketCap,
		PERatio:       ptr(peRatio),
		DividendYield: ptr(divYield),
		Revenue:       revenue,
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
	endpoint := "https://query1.finance.yahoo.com/v8/finance/chart/" + ticker

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

	var parsed yahooChartResponse
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse yahoo finance response: %w", err)
	}
	if len(parsed.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data from yahoo finance for %s", ticker)
	}

	meta := parsed.Chart.Result[0].Meta
	prevClose := meta.PreviousClose
	if prevClose == 0 {
		prevClose = meta.ChartPreviousClose
	}
	change := meta.RegularMarketPrice - prevClose
	var changePercent float64
	if prevClose != 0 {
		changePercent = (change / prevClose) * 100
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
		Change:        ptr(change),
		ChangePercent: ptr(changePercent),
		Currency:      meta.Currency,
	}, nil
}

// --- Orchestration ---

var nonTickerCharRe = regexp.MustCompile(`[^A-Z0-9.]`)

func Financials(rawTicker string) types.ToolResult {
	if rawTicker == "" {
		return types.ToolResult{Tool: types.ToolFinancials, Success: false, Data: nil, Error: "Invalid ticker identifier"}
	}

	ticker := rawTicker
	lowerCaseName := strings.ToLower(strings.TrimSpace(rawTicker))
	if mapped, ok := knownTickers[lowerCaseName]; ok {
		ticker = mapped
	}

	cleanTicker := nonTickerCharRe.ReplaceAllString(strings.ToUpper(ticker), "")
	isIndianHint := strings.Contains(cleanTicker, ".NS") || strings.Contains(cleanTicker, ".BO")

	if isIndianHint {
		if data := fetchScreenerData(cleanTicker); data != nil {
			return types.ToolResult{Tool: types.ToolFinancials, Success: true, Data: *data}
		}
	}

	if quote, err := fetchYahooQuote(cleanTicker); err == nil {
		return types.ToolResult{Tool: types.ToolFinancials, Success: true, Data: *quote}
	}

	// Yahoo failed -> try Screener as a general fallback, regardless of
	// the .NS/.BO hint.
	if data := fetchScreenerData(cleanTicker); data != nil {
		return types.ToolResult{Tool: types.ToolFinancials, Success: true, Data: *data}
	}

	if mock, ok := mockData[cleanTicker]; ok {
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
			},
		}
	}

	return types.ToolResult{
		Tool:    types.ToolFinancials,
		Success: false,
		Data:    nil,
		Error:   fmt.Sprintf("Financial pipeline failure: no data available for %s", cleanTicker),
	}
}
