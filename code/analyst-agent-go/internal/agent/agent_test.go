package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"analystagent/internal/llm"
	"analystagent/internal/types"
)

func TestFallbackReportAlwaysHasAllSixSections(t *testing.T) {
	want := []string{"Company Overview", "Financial Snapshot", "Recent News", "Competitive Landscape", "Risk Factors", "Investment Summary"}

	// With no data at all, and with some data — the shape must not change.
	empty := buildFallbackReport("Acme", &collected{})
	price := 100.0
	full := buildFallbackReport("Acme", &collected{
		search:     []types.SearchResult{{Title: "Acme Corp", Snippet: "Makes things"}},
		financials: &types.FinancialData{CompanyName: "Acme Ltd", Ticker: "ACME", Currency: "USD", Price: &price, Source: "Yahoo Finance"},
		news:       []types.NewsItem{{Title: "Acme wins", Source: "Wire"}},
	})

	for name, r := range map[string]types.ResearchReport{"empty": empty, "full": full} {
		if len(r.Sections) != len(want) {
			t.Fatalf("%s: want %d sections, got %d", name, len(want), len(r.Sections))
		}
		for i, s := range r.Sections {
			if s.Title != want[i] {
				t.Errorf("%s: section %d is %q, want %q", name, i, s.Title, want[i])
			}
			if strings.TrimSpace(s.Content) == "" {
				t.Errorf("%s: section %q is empty", name, s.Title)
			}
		}
	}
}

func TestFallbackReportMarksMissingDataInsteadOfInventingIt(t *testing.T) {
	r := buildFallbackReport("Acme", &collected{})
	text := ""
	for _, s := range r.Sections {
		text += s.Content + "\n"
	}
	if !strings.Contains(text, "could not be retrieved") {
		t.Error("missing financial data should be stated explicitly")
	}
	if !strings.Contains(text, notAvailable) {
		t.Error("sections that need the model should say they are not available")
	}
	if strings.ContainsAny(text, "$₹") {
		t.Error("no figures should appear when none were retrieved")
	}
}

func TestFallbackReportShowsSourceAndUnknownChange(t *testing.T) {
	price := 1500.0
	r := buildFallbackReport("Infosys", &collected{financials: &types.FinancialData{
		CompanyName: "Infosys", Ticker: "INFY", Currency: "INR", Price: &price, Source: "Screener.in",
		// ChangePercent deliberately nil: unknown, not zero.
	}})
	fin := r.Sections[1].Content
	if !strings.Contains(fin, "Source: Screener.in") {
		t.Errorf("source should be shown:\n%s", fin)
	}
	if strings.Contains(fin, "today") {
		t.Errorf("an unknown day change must not be displayed:\n%s", fin)
	}
}

func TestEmptyResultsAreNotEvidence(t *testing.T) {
	c := &collected{}
	cases := []types.ToolResult{
		{Tool: types.ToolSearch, Success: true, Data: []types.SearchResult{}},
		{Tool: types.ToolNews, Success: true, Data: []types.NewsItem{}},
		{Tool: types.ToolFinancials, Success: false, Error: "down"},
		{Tool: types.ToolSearch, Success: true, Data: nil},
	}
	for i, res := range cases {
		if c.record(res) {
			t.Errorf("case %d should not count as retrieved data", i)
		}
	}
	if !c.record(types.ToolResult{Tool: types.ToolSearch, Success: true, Data: []types.SearchResult{{Title: "x"}}}) {
		t.Error("a non-empty search result should count")
	}
	if !c.record(types.ToolResult{Tool: types.ToolFinancials, Success: true, Data: types.FinancialData{Ticker: "A"}}) {
		t.Error("a financial result should count")
	}
}

func TestNormalizeSentiment(t *testing.T) {
	for in, want := range map[string]string{
		"Bullish": "Bullish", "BEARISH": "Bearish", " bullish ": "Bullish",
		"Neutral": "Neutral", "Very Positive!": "Neutral", "": "Neutral",
	} {
		if got := normalizeSentiment(in); got != want {
			t.Errorf("normalizeSentiment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseReport(t *testing.T) {
	fenced := "```json\n" + `{"company":"Acme","overallSentiment":"BULLISH","sections":[{"title":"A","icon":"X","content":"- one"},{"title":"","icon":"X","content":"- dropped"}]}` + "\n```"
	r, ok := parseReport("fallback", fenced)
	if !ok {
		t.Fatal("a fenced report should parse")
	}
	if r.Company != "Acme" || r.OverallSentiment != "Bullish" {
		t.Errorf("got %+v", r)
	}
	if len(r.Sections) != 1 {
		t.Errorf("a section with no title should be dropped, got %d sections", len(r.Sections))
	}

	for _, bad := range []string{"not json at all", `{"sections":[]}`, `{"company":"Acme"}`} {
		if _, ok := parseReport("x", bad); ok {
			t.Errorf("%q has no usable sections and should not parse", bad)
		}
	}
}

func TestParseJSONHandlesFencesAndSurroundingText(t *testing.T) {
	for _, in := range []string{
		`{"action":"synthesize"}`,
		"```json\n{\"action\":\"synthesize\"}\n```",
		`Sure! Here you go: {"action":"synthesize"} hope that helps`,
	} {
		m, ok := parseJSON(in)
		if !ok || m["action"] != "synthesize" {
			t.Errorf("parseJSON(%q) = %v, %v", in, m, ok)
		}
	}
	if _, ok := parseJSON("no braces here"); ok {
		t.Error("text without JSON should not parse")
	}
}

// withFakes swaps the LLM and tool entry points for the duration of a test.
func withFakes(t *testing.T, llmFn func(string, []llm.ConversationMessage) (string, error), toolFn func(types.ToolName, map[string]string) types.ToolResult) {
	t.Helper()
	origLLM, origTool, origBudget := callLLM, callTool, runBudget
	t.Cleanup(func() { callLLM, callTool, runBudget = origLLM, origTool, origBudget })
	callLLM, callTool = llmFn, toolFn
}

func runAndCollect(company string) []types.AgentMessage {
	var msgs []types.AgentMessage
	Run(company, func(m types.AgentMessage) { msgs = append(msgs, m) })
	return msgs
}

func TestRunStopsResearchingWhenTheTimeBudgetIsSpentAndWritesFromWhatItHas(t *testing.T) {
	llmCalls, toolCalls := 0, 0
	withFakes(t,
		func(sys string, h []llm.ConversationMessage) (string, error) {
			llmCalls++
			last := h[len(h)-1].Text
			if strings.HasPrefix(last, "Write the final investment report") {
				return `{"company":"Acme","overallSentiment":"Neutral","sections":[{"title":"Company Overview","icon":"OVERVIEW","content":"- from data"}]}`, nil
			}
			// Always asks for another tool call, never finishes on its own.
			return `{"action":"tool_call","tool":"search","args":{"query":"acme"},"reasoning":"more"}`, nil
		},
		func(tool types.ToolName, args map[string]string) types.ToolResult {
			toolCalls++
			time.Sleep(30 * time.Millisecond)
			return types.ToolResult{Tool: tool, Success: true, Data: []types.SearchResult{{Title: "Acme"}}}
		})
	runBudget = 50 * time.Millisecond

	msgs := runAndCollect("Acme")

	if toolCalls >= maxSteps {
		t.Errorf("budget should have cut research short, but %d tool calls ran", toolCalls)
	}
	var sawBudgetNote, sawReport bool
	for _, m := range msgs {
		if m.Type == types.MsgStep && strings.Contains(m.Step.Content, "Time budget") {
			sawBudgetNote = true
		}
		if m.Type == types.MsgReport {
			sawReport = true
		}
	}
	if !sawBudgetNote || !sawReport {
		t.Errorf("want a budget note and a report (note=%v report=%v)", sawBudgetNote, sawReport)
	}
	if msgs[len(msgs)-1].Type != types.MsgDone {
		t.Error("run must end with done")
	}
}

func TestRunWithNoDataAndBudgetSpentStillReturnsAHonestReport(t *testing.T) {
	withFakes(t,
		func(string, []llm.ConversationMessage) (string, error) {
			return `{"action":"tool_call","tool":"news","args":{"company":"x"},"reasoning":"r"}`, nil
		},
		func(tool types.ToolName, args map[string]string) types.ToolResult {
			return types.ToolResult{Tool: tool, Success: false, Error: "down"}
		})
	runBudget = 0 // already spent

	msgs := runAndCollect("Ghost Corp")
	last := msgs[len(msgs)-1]
	var report *types.ResearchReport
	for _, m := range msgs {
		if m.Type == types.MsgReport {
			report = m.Report
		}
	}
	if report == nil || len(report.Sections) != 6 || last.Type != types.MsgDone {
		t.Fatalf("want a six-section fallback report ending in done, got %+v", msgs)
	}
}

func TestTruncateNeverSplitsACharacter(t *testing.T) {
	s := strings.Repeat("é", 50) // two bytes each
	for max := 1; max <= 20; max++ {
		out := truncate(s, max)
		if !utf8.ValidString(out) {
			t.Fatalf("truncate(..., %d) produced invalid UTF-8: %q", max, out)
		}
	}
	if got := truncate("short", 100); got != "short" {
		t.Errorf("short strings are returned unchanged, got %q", got)
	}
	// Valid JSON is what gets truncated in practice; make sure it can be sent.
	if _, err := json.Marshal(truncate(strings.Repeat("株", 40), 10)); err != nil {
		t.Error(err)
	}
}

func TestFmtNumKeepsSmallPricesReadable(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	for _, c := range []struct {
		in   *float64
		want string
	}{
		{f(189.4321), "189.43"}, {f(0), "0.00"}, {f(0.5), "0.5000"},
		{f(0.0000089), "0.000009"}, {f(-12.3), "-12.30"}, {nil, "n/a"},
	} {
		if got := fmtNum(c.in); got != c.want {
			t.Errorf("fmtNum = %q, want %q", got, c.want)
		}
	}
}

func TestFallbackStepSaysWhyTheModelFailed(t *testing.T) {
	withFakes(t,
		func(string, []llm.ConversationMessage) (string, error) {
			return "", errors.New("Gemini API error (404): models/gemini-2.0-flash-lite is no longer available")
		},
		func(tool types.ToolName, args map[string]string) types.ToolResult {
			return types.ToolResult{Tool: tool, Success: false, Error: "down"}
		})
	var joined string
	for _, m := range runAndCollect("Acme") {
		if m.Type == types.MsgStep {
			joined += m.Step.Content + "\n"
		}
	}
	if !strings.Contains(joined, "404") || !strings.Contains(joined, "no longer available") {
		t.Errorf("the step log should carry the model error, got:\n%s", joined)
	}
}
