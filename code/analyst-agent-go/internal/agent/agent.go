// Package agent runs the actual research loop: it asks the LLM to plan,
// calls tools based on what the LLM decides it needs, feeds the results
// back, and eventually asks the LLM to synthesize everything into a final
// report.
//
// Two rules keep the report grounded in real data:
//   - the model may not write a report until at least one tool has
//     actually returned data, and
//   - if no tool returned data (or the model is unavailable), a
//     deterministic report is built from whatever was retrieved, with
//     every missing section explicitly marked as unavailable.
package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"analystagent/internal/llm"
	"analystagent/internal/tools"
	"analystagent/internal/types"
)

const (
	maxSteps           = 8
	maxToolResultChars = 3000
	notAvailable       = "Not available from the retrieved data."
)

// runBudget caps how long the research loop may keep gathering data. It is
// checked between steps, so the last LLM or tool call can overrun it; the
// HTTP server's write timeout leaves room for that. When it runs out the
// agent stops researching and writes the report from what it has.
// Override with RUN_BUDGET_SECONDS.
var runBudget = defaultRunBudget()

func defaultRunBudget() time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("RUN_BUDGET_SECONDS"))); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 90 * time.Second
}

// The LLM and tool entry points are variables so tests can run the loop
// with fakes, without network access.
var (
	callLLM  = llm.CallGemini
	callTool = tools.Call
)

var validTools = map[types.ToolName]bool{
	types.ToolSearch:     true,
	types.ToolFinancials: true,
	types.ToolNews:       true,
}

func createSystemPrompt(company string) string {
	toolDescriptions := tools.DescriptionsForPrompt()
	return fmt.Sprintf(`You are a research analyst. Your objective is to research "%s" and produce a structured investment brief.

## Tools
%s

## Protocol
1. State a short research plan (3-5 steps).
2. Call tools one at a time to gather what the plan needs.
3. After each tool result, decide whether you have enough to write the
   report, or whether you need another tool call.
4. Once you have enough, synthesize.

## Rules
- Only state figures and facts that appear in a tool result. Never fill gaps from memory.
- If a tool returns an error or no results, try different parameters or another tool.

## Response format — respond with ONLY one JSON object, no markdown fences:

Plan:
{"action":"plan","plan":["step 1","step 2",...]}

Tool call:
{"action":"tool_call","tool":"search|financials|news","args":{"param":"value"},"reasoning":"why this call"}

Reflection:
{"action":"reflect","assessment":"what you've learned so far"}

Ready to write the report:
{"action":"synthesize"}`, company, toolDescriptions)
}

const synthesisPrompt = `Write the final investment report as ONLY one JSON object, no markdown fences:
{
  "company": "Official Company Name",
  "overallSentiment": "Bullish" | "Neutral" | "Bearish",
  "sections": [
    {"title": "Company Overview", "icon": "OVERVIEW", "content": "- point 1\n- point 2\n- point 3"},
    {"title": "Financial Snapshot", "icon": "FINANCIALS", "content": "- point 1\n- point 2\n- point 3"},
    {"title": "Recent News", "icon": "NEWS", "content": "- point 1\n- point 2"},
    {"title": "Competitive Landscape", "icon": "COMPETITION", "content": "- point 1\n- point 2"},
    {"title": "Risk Factors", "icon": "RISK", "content": "- point 1\n- point 2"},
    {"title": "Investment Summary", "icon": "SUMMARY", "content": "- bull case\n- bear case\n- verdict"}
  ]
}
Rules:
- Use only information from the tool results above. Do not invent figures.
- If the tool results do not support a section, write exactly one bullet: "- Not available from the retrieved data." instead of guessing.
- Each section's content is 2-5 bullet points, one per line, prefixed with "- ".`

// Gemini doesn't always follow the "no markdown fences" instruction —
// strip a ```json ... ``` wrapper if present before parsing, and fall back
// to extracting the first {...} block if that doesn't match either.
var codeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
var jsonObjectRe = regexp.MustCompile(`(?s)\{.*\}`)

func parseJSON(text string) (map[string]any, bool) {
	cleaned := strings.TrimSpace(text)
	toParse := cleaned
	if m := codeFenceRe.FindStringSubmatch(cleaned); m != nil {
		toParse = strings.TrimSpace(m[1])
	} else if m := jsonObjectRe.FindString(cleaned); m != "" {
		toParse = m
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(toParse), &out); err != nil {
		return nil, false
	}
	return out, true
}

// normalizeSentiment maps whatever the model wrote onto the three values
// the frontend knows how to display.
func normalizeSentiment(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "bullish":
		return "Bullish"
	case "bearish":
		return "Bearish"
	default:
		return "Neutral"
	}
}

// truncate shortens s to at most max bytes without ever cutting a
// multi-byte character in half (which would produce invalid UTF-8 — and
// company names and headlines are not always ASCII).
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…(truncated)"
}

// --- Step log ---

// stepEmitter numbers each step and forwards it as an AgentMessage.
type stepEmitter struct {
	stepID int
	emit   func(types.AgentMessage)
}

func (s *stepEmitter) add(stepType types.AgentStepType, content string, call *types.ToolCall, result *types.ToolResult) {
	s.stepID++
	s.emit(types.AgentMessage{
		Type: types.MsgStep,
		Step: &types.AgentStep{
			ID:         s.stepID,
			Type:       stepType,
			Content:    content,
			ToolCall:   call,
			ToolResult: result,
			Timestamp:  time.Now().UnixMilli(),
		},
	})
}

// --- Collected data ---

// collected holds the most recent usable result from each tool.
type collected struct {
	search     []types.SearchResult
	financials *types.FinancialData
	news       []types.NewsItem
}

// record stores a tool result and reports whether it actually carried data.
// A call can "succeed" yet return nothing (e.g. an empty search result when
// every source was unreachable), and that must not count as evidence.
func (c *collected) record(result types.ToolResult) bool {
	if !result.Success {
		return false
	}
	switch d := result.Data.(type) {
	case []types.SearchResult:
		if len(d) > 0 {
			c.search = d
			return true
		}
	case []types.NewsItem:
		if len(d) > 0 {
			c.news = d
			return true
		}
	case types.FinancialData:
		v := d
		c.financials = &v
		return true
	}
	return false
}

func describeResult(tool types.ToolName, result types.ToolResult, gotData bool) string {
	name := strings.ToUpper(string(tool))
	switch {
	case !result.Success:
		return fmt.Sprintf("%s error: %s", name, result.Error)
	case !gotData:
		return fmt.Sprintf("%s returned no results.", name)
	default:
		return fmt.Sprintf("%s data acquired.", name)
	}
}

// fetch calls a tool as one logged step pair (call + result) and records
// anything useful it returned.
func (c *collected) fetch(se *stepEmitter, call types.ToolCall) {
	se.add(types.StepToolCall, call.Reasoning, &call, nil)
	result := callTool(call.Tool, call.Args)
	gotData := c.record(result)
	se.add(types.StepToolResult, describeResult(call.Tool, result, gotData), nil, &result)
}

// --- Deterministic report ---

// fmtNum formats a figure for the report. Sub-dollar prices (penny stocks,
// crypto such as SHIB-USD) keep enough decimals not to show as "0.00".
func fmtNum(p *float64) string {
	if p == nil {
		return "n/a"
	}
	digits := 2
	switch abs := math.Abs(*p); {
	case abs == 0:
	case abs < 0.01:
		digits = 6
	case abs < 1:
		digits = 4
	}
	return strconv.FormatFloat(*p, 'f', digits, 64)
}

func orNA(s string) string {
	if s == "" || s == "N/A" {
		return "n/a"
	}
	return s
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func bullets(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, "- "+l)
	}
	return strings.Join(out, "\n")
}

// buildFallbackReport assembles a report straight from retrieved data. It
// always returns all six sections; anything that needs the AI model's
// judgment (competition, risks, the verdict) is explicitly marked
// unavailable rather than left out or invented.
func buildFallbackReport(company string, c *collected) types.ResearchReport {
	name := company

	overview := []string{"No overview information could be retrieved."}
	if len(c.search) > 0 {
		overview = nil
		for i, r := range c.search {
			if i >= 3 {
				break
			}
			overview = append(overview, fmt.Sprintf("%s: %s", clip(r.Title, 80), clip(r.Snippet, 160)))
		}
	}

	financial := []string{"Financial data could not be retrieved."}
	if fd := c.financials; fd != nil {
		if fd.CompanyName != "" {
			name = fd.CompanyName
		}
		price := "n/a"
		if fd.Price != nil {
			price = fmt.Sprintf("%s %s", fmtNum(fd.Price), fd.Currency)
			if fd.ChangePercent != nil {
				price += fmt.Sprintf(" (%s%% today)", fmtNum(fd.ChangePercent))
			}
		}
		financial = []string{
			fmt.Sprintf("%s (%s)", fd.CompanyName, fd.Ticker),
			"Price: " + price,
			"Market cap: " + orNA(fd.MarketCap),
			"P/E ratio: " + fmtNum(fd.PERatio),
			"Dividend yield: " + fmtNum(fd.DividendYield),
			"Revenue: " + orNA(fd.Revenue),
			"Source: " + orNA(fd.Source),
		}
	}

	news := []string{"No recent headlines could be retrieved."}
	if len(c.news) > 0 {
		news = nil
		for i, n := range c.news {
			if i >= 5 {
				break
			}
			news = append(news, fmt.Sprintf("%s (%s)", clip(n.Title, 140), n.Source))
		}
	}

	needsModel := func(what string) []string {
		return []string{notAvailable + " " + what + " needs the AI model, which was not used for this report."}
	}

	return types.ResearchReport{
		Company:     name,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		// No sentiment judgment was made; "Neutral" here means "not assessed".
		OverallSentiment: "Neutral",
		Sections: []types.ReportSection{
			{Title: "Company Overview", Icon: "OVERVIEW", Content: bullets(overview)},
			{Title: "Financial Snapshot", Icon: "FINANCIALS", Content: bullets(financial)},
			{Title: "Recent News", Icon: "NEWS", Content: bullets(news)},
			{Title: "Competitive Landscape", Icon: "COMPETITION", Content: bullets(needsModel("Competitor analysis"))},
			{Title: "Risk Factors", Icon: "RISK", Content: bullets(needsModel("Risk analysis"))},
			{Title: "Investment Summary", Icon: "SUMMARY", Content: bullets(append(needsModel("A bull/bear verdict"), "No sentiment assessment was made."))},
		},
	}
}

// runDeterministicFallback fetches whichever tools haven't produced data
// yet, then emits a report built only from retrieved data.
func runDeterministicFallback(company string, se *stepEmitter, c *collected, reason string) {
	se.add(types.StepReflection, reason+" Switching to deterministic research mode.", nil, nil)

	if c.search == nil {
		c.fetch(se, types.ToolCall{Tool: types.ToolSearch, Args: map[string]string{"query": company + " company overview"}, Reasoning: "Gather baseline context."})
	}
	if c.financials == nil {
		c.fetch(se, types.ToolCall{Tool: types.ToolFinancials, Args: map[string]string{"ticker": company}, Reasoning: "Resolve financial data."})
	}
	if c.news == nil {
		c.fetch(se, types.ToolCall{Tool: types.ToolNews, Args: map[string]string{"company": company}, Reasoning: "Pull recent headlines."})
	}

	report := buildFallbackReport(company, c)
	se.add(types.StepSynthesis, "Deterministic report assembled from retrieved data only.", nil, nil)
	se.emit(types.AgentMessage{Type: types.MsgReport, Report: &report})
	se.emit(types.AgentMessage{Type: types.MsgDone})
}

// --- The loop ---

// Run executes the research loop for the given company, calling emit once
// per AgentMessage in order.
func Run(company string, emit func(types.AgentMessage)) {
	se := &stepEmitter{emit: emit}
	data := &collected{}
	systemPrompt := createSystemPrompt(company)
	var history []llm.ConversationMessage

	se.add(types.StepPlan, fmt.Sprintf("Starting research on %s...", company), nil, nil)

	history = append(history, llm.ConversationMessage{
		Role: "user",
		Text: fmt.Sprintf(`Research "%s". Identify the correct stock ticker if one isn't given. Start with a short plan.`, company),
	})

	llmFailed := false
	usefulResults := 0 // tool calls that returned real data

	deadline := time.Now().Add(runBudget)
	for iterations := 0; iterations < maxSteps; iterations++ {
		if time.Now().After(deadline) {
			se.add(types.StepReflection, "Time budget for research reached — wrapping up with what was gathered.", nil, nil)
			break
		}
		history = append(history, llm.ConversationMessage{Role: "user", Text: `Continue. Respond with the next action's JSON.`})

		response, err := callLLM(systemPrompt, history)
		if err != nil {
			llmFailed = true
			break
		}
		history = append(history, llm.ConversationMessage{Role: "model", Text: response})

		actionData, ok := parseJSON(response)
		if !ok {
			se.add(types.StepError, "Could not parse the model's response. Retrying.", nil, nil)
			history = append(history, llm.ConversationMessage{Role: "user", Text: "That wasn't valid JSON. Respond with only the JSON object."})
			continue
		}

		action, _ := actionData["action"].(string)

		if action == "synthesize" {
			if usefulResults == 0 && iterations < maxSteps-1 {
				se.add(types.StepReflection, "No data gathered yet — continuing research.", nil, nil)
				history = append(history, llm.ConversationMessage{Role: "user", Text: "You have not retrieved any data yet. Call at least one tool before synthesizing."})
				continue
			}
			se.add(types.StepSynthesis, "Enough information gathered. Writing the report.", nil, nil)
			break
		}

		switch action {
		case "tool_call":
			toolName := types.ToolName(fmt.Sprintf("%v", actionData["tool"]))
			reasoning, _ := actionData["reasoning"].(string)
			args := map[string]string{}
			if rawArgs, ok := actionData["args"].(map[string]any); ok {
				for k, v := range rawArgs {
					args[k] = fmt.Sprintf("%v", v)
				}
			}

			if !validTools[toolName] {
				se.add(types.StepError, "Model requested an unknown tool: "+string(toolName), nil, nil)
				history = append(history, llm.ConversationMessage{Role: "user", Text: "Unknown tool. Valid tools: search, financials, news."})
				continue
			}

			call := types.ToolCall{Tool: toolName, Args: args, Reasoning: reasoning}
			se.add(types.StepToolCall, reasoning, &call, nil)

			result := callTool(toolName, args)
			gotData := data.record(result)
			se.add(types.StepToolResult, describeResult(toolName, result, gotData), nil, &result)

			switch {
			case gotData:
				usefulResults++
				dataJSON, _ := json.Marshal(result.Data)
				history = append(history, llm.ConversationMessage{Role: "user", Text: "Tool result: " + truncate(string(dataJSON), maxToolResultChars)})
			case !result.Success:
				// Tell the model *why* it failed — it can't adjust otherwise.
				history = append(history, llm.ConversationMessage{Role: "user", Text: "Tool error: " + result.Error + ". Try different parameters or another tool."})
			default:
				history = append(history, llm.ConversationMessage{Role: "user", Text: "The tool returned no results. Try different parameters or another tool."})
			}
		case "reflect":
			assessment, _ := actionData["assessment"].(string)
			se.add(types.StepReflection, assessment, nil, nil)
		default:
			se.add(types.StepReflection, "Processing...", nil, nil)
		}
	}

	if llmFailed {
		runDeterministicFallback(company, se, data, "The AI model was unavailable.")
		return
	}
	if usefulResults == 0 {
		runDeterministicFallback(company, se, data, "No tool returned usable data, so a report will not be written from the model's memory alone.")
		return
	}

	history = append(history, llm.ConversationMessage{Role: "user", Text: synthesisPrompt})
	synthesisResponse, err := callLLM(systemPrompt, history)
	if err != nil {
		runDeterministicFallback(company, se, data, "The AI model was unavailable for the final report.")
		return
	}

	report, ok := parseReport(company, synthesisResponse)
	if !ok {
		runDeterministicFallback(company, se, data, "The model's final report could not be parsed.")
		return
	}

	se.emit(types.AgentMessage{Type: types.MsgReport, Report: &report})
	se.emit(types.AgentMessage{Type: types.MsgDone})
}

// parseReport turns the model's synthesis JSON into a ResearchReport. It
// reports false if there are no usable sections.
func parseReport(company, text string) (types.ResearchReport, bool) {
	reportData, ok := parseJSON(text)
	if !ok {
		return types.ResearchReport{}, false
	}

	report := types.ResearchReport{
		Company:          company,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		OverallSentiment: "Neutral",
	}
	if c, ok := reportData["company"].(string); ok && strings.TrimSpace(c) != "" {
		report.Company = c
	}
	if s, ok := reportData["overallSentiment"].(string); ok {
		report.OverallSentiment = normalizeSentiment(s)
	}

	sections, _ := reportData["sections"].([]any)
	for _, raw := range sections {
		secMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		sec := types.ReportSection{}
		sec.Title, _ = secMap["title"].(string)
		sec.Icon, _ = secMap["icon"].(string)
		sec.Content, _ = secMap["content"].(string)
		if strings.TrimSpace(sec.Title) == "" || strings.TrimSpace(sec.Content) == "" {
			continue
		}
		report.Sections = append(report.Sections, sec)
	}

	return report, len(report.Sections) > 0
}
