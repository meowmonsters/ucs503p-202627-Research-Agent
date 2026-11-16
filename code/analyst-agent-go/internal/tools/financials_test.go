package tools

import (
	"errors"
	"strings"
	"testing"

	"analystagent/internal/types"
)

// stubFetchers swaps the three live sources for fakes (nil = "this source
// fails") and restores the real ones when the test ends.
func stubFetchers(t *testing.T, td, yahoo, screener *types.FinancialData) (calls *[]string) {
	t.Helper()
	origTD, origYahoo, origScreener, origLookup := fetchTwelveData, fetchYahoo, fetchScreener, lookupSymbol
	t.Cleanup(func() {
		fetchTwelveData, fetchYahoo, fetchScreener, lookupSymbol = origTD, origYahoo, origScreener, origLookup
	})
	// Name lookups must never reach the network in tests.
	lookupSymbol = func(string) (string, error) { return "", errors.New("no match") }

	log := []string{}
	fetchTwelveData = func(string) (*types.FinancialData, error) {
		log = append(log, "twelvedata")
		if td == nil {
			return nil, errors.New("down")
		}
		c := *td
		return &c, nil
	}
	fetchYahoo = func(string) (*types.FinancialData, error) {
		log = append(log, "yahoo")
		if yahoo == nil {
			return nil, errors.New("down")
		}
		c := *yahoo
		return &c, nil
	}
	fetchScreener = func(string) *types.FinancialData {
		log = append(log, "screener")
		if screener == nil {
			return nil
		}
		c := *screener
		return &c
	}
	return &log
}

func yahooQuote() *types.FinancialData {
	return &types.FinancialData{
		Ticker: "INFY.NS", CompanyName: "Infosys", Currency: "INR",
		Price: ptr(1500), Change: ptr(12), ChangePercent: ptr(0.8),
		MarketCap: "", Source: "Yahoo Finance",
	}
}

func screenerData() *types.FinancialData {
	return &types.FinancialData{
		Ticker: "INFY", CompanyName: "INFY", Currency: "INR",
		Price:     ptr(1499), // no Change: Screener.in has no day-change figure
		MarketCap: "6,00,000 Cr", PERatio: ptr(24), DividendYield: ptr(2.1),
		Revenue: "1,50,000 Cr", Source: "Screener.in",
	}
}

func TestIndianTickerKeepsLivePriceAndAddsScreenerFundamentals(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	stubFetchers(t, nil, yahooQuote(), screenerData())

	res := Financials("INFY.NS")
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	d := res.Data.(types.FinancialData)

	if d.Change == nil || *d.Change != 12 {
		t.Errorf("day change should come from Yahoo (12), got %v", d.Change)
	}
	if d.MarketCap != "6,00,000 Cr" || d.PERatio == nil || *d.PERatio != 24 {
		t.Errorf("fundamentals should be filled in from Screener.in, got %+v", d)
	}
	if !strings.Contains(d.Source, "Yahoo Finance") || !strings.Contains(d.Source, "Screener.in") {
		t.Errorf("source should credit both, got %q", d.Source)
	}
}

func TestTwelveDataQuoteGetsScreenerFundamentalsForIndianTicker(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "test-key")
	td := &types.FinancialData{
		Ticker: "INFY.NS", Currency: "INR", Price: ptr(1500), Change: ptr(5),
		MarketCap: "N/A", Revenue: "N/A", Source: "Twelve Data",
	}
	stubFetchers(t, td, nil, screenerData())

	d := Financials("INFY.NS").Data.(types.FinancialData)
	if d.MarketCap != "6,00,000 Cr" || d.Revenue != "1,50,000 Cr" {
		t.Errorf("N/A fundamentals from Twelve Data should be replaced, got %+v", d)
	}
	if d.Source != "Twelve Data + Screener.in" {
		t.Errorf("unexpected source %q", d.Source)
	}
}

func TestTwelveDataIsTriedFirstWhenKeyed(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "test-key")
	td := &types.FinancialData{Ticker: "AAPL", Currency: "USD", Price: ptr(200), Source: "Twelve Data"}
	calls := stubFetchers(t, td, yahooQuote(), nil)

	Financials("AAPL")
	if len(*calls) != 1 || (*calls)[0] != "twelvedata" {
		t.Errorf("only Twelve Data should have been called, got %v", *calls)
	}
}

func TestTwelveDataSkippedWithoutKey(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	calls := stubFetchers(t, &types.FinancialData{Price: ptr(1)}, yahooQuote(), nil)

	Financials("AAPL")
	for _, c := range *calls {
		if c == "twelvedata" {
			t.Errorf("Twelve Data must not be called without a key, calls: %v", *calls)
		}
	}
}

func TestUSTickerDoesNotCallScreenerWhenYahooWorks(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	us := &types.FinancialData{Ticker: "AAPL", Currency: "USD", Price: ptr(200), Source: "Yahoo Finance"}
	calls := stubFetchers(t, nil, us, screenerData())

	Financials("AAPL")
	for _, c := range *calls {
		if c == "screener" {
			t.Errorf("Screener.in is for Indian listings; calls: %v", *calls)
		}
	}
}

func TestScreenerAloneLeavesDayChangeUnknown(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	stubFetchers(t, nil, nil, screenerData())

	res := Financials("INFY.NS")
	if !res.Success {
		t.Fatalf("Screener.in alone should still succeed, got %+v", res)
	}
	d := res.Data.(types.FinancialData)
	if d.Change != nil || d.ChangePercent != nil {
		t.Errorf("a missing day change must stay nil, not become zero: %+v", d)
	}
}

func TestAllSourcesFailingIsAFailureNotInventedData(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	t.Setenv("ALLOW_SAMPLE_DATA", "")
	stubFetchers(t, nil, nil, nil)

	res := Financials("AAPL") // AAPL has hard-coded sample figures
	if res.Success {
		t.Fatalf("sample figures must not be returned by default, got %+v", res)
	}
	if res.Data != nil || res.Error == "" {
		t.Errorf("expected a clear error and no data, got %+v", res)
	}
}

func TestSampleDataIsLabelledWhenEnabled(t *testing.T) {
	t.Setenv("TWELVEDATA_API_KEY", "")
	t.Setenv("ALLOW_SAMPLE_DATA", "1")
	stubFetchers(t, nil, nil, nil)

	res := Financials("AAPL")
	if !res.Success {
		t.Fatalf("expected sample data when enabled, got %+v", res)
	}
	d := res.Data.(types.FinancialData)
	if !strings.Contains(d.Source, "Sample data") {
		t.Errorf("sample data must say so in Source, got %q", d.Source)
	}
}

func TestMergeFundamentalsNeverOverwrites(t *testing.T) {
	dst := &types.FinancialData{MarketCap: "1 Cr", PERatio: ptr(10), Source: "A"}
	src := &types.FinancialData{MarketCap: "9 Cr", PERatio: ptr(99), Source: "B"}
	mergeFundamentals(dst, src)

	if dst.MarketCap != "1 Cr" || *dst.PERatio != 10 {
		t.Errorf("existing values were overwritten: %+v", dst)
	}
	if dst.Source != "A" {
		t.Errorf("source should be unchanged when nothing was merged, got %q", dst.Source)
	}
}

func TestBlankTickerRejected(t *testing.T) {
	if res := Financials("   "); res.Success {
		t.Errorf("blank ticker should fail, got %+v", res)
	}
}
