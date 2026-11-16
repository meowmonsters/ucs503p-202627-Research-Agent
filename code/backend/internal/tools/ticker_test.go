package tools

import (
	"errors"
	"strings"
	"testing"

	"analystagent/internal/types"
)

func TestInterpretTicker(t *testing.T) {
	cases := []struct {
		in        string
		wantSym   string
		wantKnown bool
	}{
		// plain tickers, any case, cashtag
		{"AAPL", "AAPL", false}, {"aapl", "AAPL", false}, {"$aapl", "AAPL", false},
		// punctuation the old cleaner destroyed (BRK-B became BRKB)
		{"BRK-B", "BRK-B", false}, {"brk-b", "BRK-B", false},
		{"BAJAJ-AUTO.NS", "BAJAJ-AUTO.NS", false}, {"M&M.NS", "M&M.NS", false},
		{"^GSPC", "^GSPC", false}, {"EURUSD=X", "EURUSD=X", false}, {"INFY.NS", "INFY.NS", false},
		// "EXCHANGE:SYMBOL" as an LLM might write it
		{"NASDAQ:AAPL", "AAPL", false}, {"NSE:TCS", "TCS.NS", false}, {"BSE:500325", "500325.BO", false},
		{"NSE:TCS.NS", "TCS.NS", false},
		// company names, with and without legal suffixes
		{"Apple", "AAPL", true}, {"Apple Inc.", "AAPL", true}, {"  APPLE INC  ", "AAPL", true},
		{"Nvidia Corporation", "NVDA", true}, {"Berkshire Hathaway Inc", "BRK-B", true},
		{"Coca-Cola Company", "KO", true}, {"Johnson & Johnson", "JNJ", true},
		{"L&T", "LT.NS", true}, {"Larsen & Toubro Limited", "LT.NS", true},
		{"Infosys", "INFY.NS", true}, {"Visa", "V", true},
		// a ticker in brackets after the name
		{"Zomato Limited (ZOMATO.NS)", "ZOMATO.NS", false},
		// ambiguous short names are tried as tickers, never "known"
		{"Delta", "DELTA", false},
		// names that need a lookup: no direct symbol at all
		{"Zomato Limited", "", false}, {"Nestlé", "", false}, {"株式会社トヨタ", "", false},
		{"Société Générale", "", false}, {"Procter & Gamble Co", "", false},
		// junk
		{"'; DROP TABLE x;--", "", false}, {"a b c d e f g h", "", false},
		{"THISISWAYTOOLONGTOBEATICKER", "", false},
	}
	for _, c := range cases {
		got := interpretTicker(c.in)
		if got.Symbol != c.wantSym || got.Known != c.wantKnown {
			t.Errorf("interpretTicker(%q) = %+v, want {%q %v}", c.in, got, c.wantSym, c.wantKnown)
		}
	}
}

// recordingSources fakes Yahoo (quotes only for symbols in `live`) and the
// name lookup, recording every symbol tried and every name looked up.
func recordingSources(t *testing.T, live map[string]*types.FinancialData, lookups map[string]string) (tried, looked *[]string) {
	t.Helper()
	stubFetchers(t, nil, nil, nil)
	t.Setenv("TWELVEDATA_API_KEY", "")
	var triedSyms, lookedNames []string
	fetchYahoo = func(sym string) (*types.FinancialData, error) {
		triedSyms = append(triedSyms, sym)
		if q, ok := live[sym]; ok {
			c := *q
			return &c, nil
		}
		return nil, errors.New("not found")
	}
	lookupSymbol = func(name string) (string, error) {
		lookedNames = append(lookedNames, name)
		if sym, ok := lookups[name]; ok {
			return sym, nil
		}
		return "", errors.New("no match")
	}
	return &triedSyms, &lookedNames
}

func usd(sym, name string) *types.FinancialData {
	return &types.FinancialData{Ticker: sym, CompanyName: name, Currency: "USD", Price: ptr(100), Source: "Yahoo Finance"}
}

func TestHyphenatedTickerReachesTheSourceIntact(t *testing.T) {
	tried, _ := recordingSources(t, map[string]*types.FinancialData{"BRK-B": usd("BRK-B", "Berkshire")}, nil)
	res := Financials("brk-b")
	if !res.Success || len(*tried) != 1 || (*tried)[0] != "BRK-B" {
		t.Errorf("want one lookup of BRK-B, got success=%v tried=%v", res.Success, *tried)
	}
}

func TestExchangePrefixedTickerIsUnwrapped(t *testing.T) {
	tried, _ := recordingSources(t, map[string]*types.FinancialData{"AAPL": usd("AAPL", "Apple")}, nil)
	if res := Financials("NASDAQ:AAPL"); !res.Success || (*tried)[0] != "AAPL" {
		t.Errorf("got success=%v tried=%v", res.Success, *tried)
	}
}

func TestCompanyNameIsResolvedAndTheMatchIsDisclosed(t *testing.T) {
	tried, looked := recordingSources(t,
		map[string]*types.FinancialData{"ZOMATO.NS": {Ticker: "ZOMATO.NS", CompanyName: "Zomato", Currency: "INR", Price: ptr(250), Source: "Yahoo Finance",
			MarketCap: "2,00,000 Cr", PERatio: ptr(80)}},
		map[string]string{"Zomato Limited": "ZOMATO.NS"})

	res := Financials("Zomato Limited")
	if !res.Success {
		t.Fatalf("a name with a spaces should resolve through the lookup: %+v", res)
	}
	d := res.Data.(types.FinancialData)
	if d.Ticker != "ZOMATO.NS" {
		t.Errorf("ticker = %q", d.Ticker)
	}
	if !strings.Contains(d.Source, `matched "Zomato Limited" to ZOMATO.NS`) {
		t.Errorf("a name match can be wrong, so the source must say it was one: %q", d.Source)
	}
	if len(*looked) != 1 {
		t.Errorf("want exactly one name lookup, got %v", *looked)
	}
	for _, s := range *tried {
		if s == "ZOMATOLIMITED" {
			t.Error("a multi-word name must not be squashed into a fake ticker and sent to the data source")
		}
	}
}

func TestNonASCIINameIsNeverMangledIntoATicker(t *testing.T) {
	tried, looked := recordingSources(t, nil, map[string]string{"Nestlé": "NESN.SW"})
	res := Financials("Nestlé")
	for _, s := range *tried {
		if s == "NESTL" {
			t.Error(`"Nestlé" was stripped to "NESTL" and looked up as a ticker`)
		}
	}
	if len(*looked) != 1 || (*looked)[0] != "Nestlé" {
		t.Errorf("the original name should be looked up, got %v", *looked)
	}
	// The resolved symbol has no live quote in this fake, so this fails —
	// but with a message the model and the user can act on.
	if res.Success || !strings.Contains(res.Error, "try its ticker") {
		t.Errorf("want a helpful failure, got %+v", res)
	}
}

func TestLookupIsSkippedWhenTheDirectTickerWorks(t *testing.T) {
	_, looked := recordingSources(t, map[string]*types.FinancialData{"AAPL": usd("AAPL", "Apple")}, nil)
	Financials("AAPL")
	if len(*looked) != 0 {
		t.Errorf("no name lookup needed for a working ticker, got %v", *looked)
	}
}

func TestKnownCompaniesNeverTriggerALookup(t *testing.T) {
	_, looked := recordingSources(t, nil, nil) // live sources all fail
	Financials("Berkshire Hathaway Inc.")
	if len(*looked) != 0 {
		t.Errorf("known names should not be guessed at via search, got %v", *looked)
	}
}

func TestUnknownCompanyFailsWithAnActionableMessage(t *testing.T) {
	recordingSources(t, nil, nil)
	res := Financials("Totally Made Up Holdings")
	if res.Success {
		t.Fatal("nothing should be invented for an unknown company")
	}
	if !strings.Contains(res.Error, "Totally Made Up Holdings") || !strings.Contains(res.Error, "ticker") {
		t.Errorf("error should name the input and suggest a ticker: %q", res.Error)
	}
}

func TestPickSymbolPrefersEquitiesOverFundsAndCurrencies(t *testing.T) {
	body := []byte(`{"quotes":[
		{"symbol":"NVDL","quoteType":"ETF"},
		{"symbol":"NVDA=X","quoteType":"CURRENCY"},
		{"symbol":"nvda","quoteType":"EQUITY"},
		{"symbol":"NVDA.MX","quoteType":"EQUITY"}]}`)
	got, err := pickSymbol(body, "nvidia")
	if err != nil || got != "NVDA" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := pickSymbol([]byte(`{"quotes":[{"symbol":"SPY","quoteType":"ETF"}]}`), "x"); err == nil {
		t.Error("a response with no equity should be an error")
	}
	if _, err := pickSymbol([]byte(`{"quotes":[]}`), "x"); err == nil {
		t.Error("an empty response should be an error")
	}
	if _, err := pickSymbol([]byte(`<html>blocked</html>`), "x"); err == nil {
		t.Error("an unparseable response should be an error, not a panic")
	}
}

func TestYahooWithoutPreviousCloseLeavesChangeUnknown(t *testing.T) {
	body := []byte(`{"chart":{"result":[{"meta":{"currency":"USD","regularMarketPrice":150.5,"longName":"Acme"}}]}}`)
	q, err := quoteFromYahooChart(body, "ACME")
	if err != nil {
		t.Fatal(err)
	}
	if q.Change != nil || q.ChangePercent != nil {
		t.Errorf("with no previous close the change is unknown, got %v / %v (it used to equal the full price)", q.Change, q.ChangePercent)
	}

	body = []byte(`{"chart":{"result":[{"meta":{"currency":"USD","regularMarketPrice":110,"chartPreviousClose":100}}]}}`)
	q, _ = quoteFromYahooChart(body, "ACME")
	if q.Change == nil || *q.Change != 10 || *q.ChangePercent != 10 {
		t.Errorf("normal case should still compute the change, got %v / %v", q.Change, q.ChangePercent)
	}
	if q.CompanyName != "ACME" {
		t.Errorf("falls back to the ticker when no name is given, got %q", q.CompanyName)
	}
}

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{
		"Apple Inc.": "apple", "  Tata   Consultancy Services Ltd ": "tata consultancy services",
		"Co":                   "co", // never reduces a name to nothing
		"Meta Platforms, Inc.": "meta platforms",
	} {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
