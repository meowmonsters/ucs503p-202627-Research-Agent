package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"analystagent/internal/types"
)

// Twelve Data (https://twelvedata.com) is an optional, free-tier financial
// data source. Unlike Yahoo's chart endpoint or the Screener.in scrape, it
// is a documented, stable JSON API, and it covers NSE/BSE-listed Indian
// tickers via an explicit SYMBOL:EXCHANGE format instead of a domain-style
// suffix. This is entirely optional: if TWELVEDATA_API_KEY is unset, or a
// call fails for any reason, Financials() falls straight through to the
// existing Yahoo/Screener/mock chain.

func twelveDataAPIKey() string {
	return os.Getenv("TWELVEDATA_API_KEY")
}

// toTwelveDataSymbol converts our Yahoo-style ticker format (e.g.
// "RELIANCE.NS") into Twelve Data's format ("RELIANCE:NSE").
func toTwelveDataSymbol(cleanTicker string) string {
	switch {
	case strings.HasSuffix(cleanTicker, ".NS"):
		return strings.TrimSuffix(cleanTicker, ".NS") + ":NSE"
	case strings.HasSuffix(cleanTicker, ".BO"):
		return strings.TrimSuffix(cleanTicker, ".BO") + ":BSE"
	default:
		return cleanTicker
	}
}

// flexFloat unmarshals a JSON number that Twelve Data may send as either a
// quoted string (its /quote endpoint does this, e.g. "close": "150.23")
// or a bare number. Anything unparseable degrades to zero rather than
// failing the whole decode, since zero is already treated as "not
// present" throughout this file.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexFloat(v)
	return nil
}

type twelveDataQuoteResponse struct {
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Currency      string    `json:"currency"`
	Close         flexFloat `json:"close"`
	Change        flexFloat `json:"change"`
	PercentChange flexFloat `json:"percent_change"`
}

// fetchTwelveDataQuote is the entry point. It requires TWELVEDATA_API_KEY
// (checked by the caller) and returns an error for any failure so
// Financials() can fall through to the next source.
func fetchTwelveDataQuote(cleanTicker string) (*types.FinancialData, error) {
	apiKey := twelveDataAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("TWELVEDATA_API_KEY is not configured")
	}

	symbol := toTwelveDataSymbol(cleanTicker)
	endpoint := "https://api.twelvedata.com/quote?symbol=" +
		url.QueryEscape(symbol) + "&apikey=" + url.QueryEscape(apiKey)

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("twelve data returned status %d", resp.StatusCode)
	}

	var parsed twelveDataQuoteResponse
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse twelve data response: %w", err)
	}
	if strings.EqualFold(parsed.Status, "error") {
		return nil, fmt.Errorf("twelve data error: %s", parsed.Message)
	}
	if parsed.Close == 0 {
		return nil, fmt.Errorf("twelve data returned no price for %s", symbol)
	}

	currency := parsed.Currency
	if currency == "" {
		currency = "USD"
	}
	name := parsed.Name
	if name == "" {
		name = cleanTicker
	}

	return &types.FinancialData{
		Ticker:        cleanTicker,
		CompanyName:   name,
		Price:         ptr(float64(parsed.Close)),
		Change:        ptr(float64(parsed.Change)),
		ChangePercent: ptr(float64(parsed.PercentChange)),
		Currency:      currency,
		MarketCap:     "N/A",
		Revenue:       "N/A",
	}, nil
}
