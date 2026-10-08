// Package agent runs the actual research loop: it asks the LLM to plan,
// calls tools based on what the LLM decides it needs, feeds the results
// back, and eventually asks the LLM to synthesize everything into a final
// report. If the LLM is ever unavailable (no credentials, rate limited,
// etc.), it falls back to a deterministic report built directly from the
// tools' raw data, so a result still comes back either way.
package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"analystagent/internal/llm"
	"analystagent/internal/tools"
	"analystagent/internal/types"
)

const maxSteps = 8

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
    {"title": "Risks", "icon": "RISK", "content": "- point 1\n- point 2"},
    {"title": "Investment Summary", "icon": "SUMMARY", "content": "- bull case\n- bear case\n- verdict"}
  ]
}
Use only information gathered from the tool calls above — don't invent figures. Each section's content should be 2-5 bullet points, one per line, prefixed with "- ".`

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

// stepEmitter tracks the auto-incrementing step ID across the run.
type stepEmitter struct {
	stepID int
	emit   func(types.AgentMessage)
}

func (s *stepEmitter) step(stepType types.AgentStepType, content string, extra *types.AgentStep) {
	s.stepID++
	step := types.AgentStep{
		ID:        s.stepID,
		Type:      stepType,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}
	if extra != nil {
		step.ToolCall = extra.ToolCall
		step.ToolResult = extra.ToolResult
	}
	s.emit(types.AgentMessage{Type: types.MsgStep, Step: &step})
}

// runDeterministicFallback builds a report directly from the tools' raw
// data, with no LLM involved, for when the LLM is unavailable entirely.
func runDeterministicFallback(company string, se *stepEmitter) {
	se.step(types.StepReflection, "LLM unavailable. Switching to deterministic research mode.", nil)

	searchCall := types.ToolCall{Tool: types.ToolSearch, Args: map[string]string{"query": company + " company overview"}, Reasoning: "Gather baseline context."}
	se.step(types.StepToolCall, searchCall.Reasoning, &types.AgentStep{ToolCall: &searchCall})
	searchResult := tools.Call(types.ToolSearch, searchCall.Args)
	se.step(types.StepToolResult, fmt.Sprintf("SEARCH success=%v", searchResult.Success), &types.AgentStep{ToolResult: &searchResult})

	financialCall := types.ToolCall{Tool: types.ToolFinancials, Args: map[string]string{"ticker": company}, Reasoning: "Resolve financial data."}
	se.step(types.StepToolCall, financialCall.Reasoning, &types.AgentStep{ToolCall: &financialCall})
	financialResult := tools.Call(types.ToolFinancials, financialCall.Args)
	se.step(types.StepToolResult, fmt.Sprintf("FINANCIALS success=%v", financialResult.Success), &types.AgentStep{ToolResult: &financialResult})

	newsCall := types.ToolCall{Tool: types.ToolNews, Args: map[string]string{"company": company}, Reasoning: "Pull recent headlines."}
	se.step(types.StepToolCall, newsCall.Reasoning, &types.AgentStep{ToolCall: &newsCall})
	newsResult := tools.Call(types.ToolNews, newsCall.Args)
	se.step(types.StepToolResult, fmt.Sprintf("NEWS success=%v", newsResult.Success), &types.AgentStep{ToolResult: &newsResult})

	companyName := company
	sections := []types.ReportSection{
		{Title: "Company Overview", Icon: "OVERVIEW", Content: "- Built from raw tool data; the LLM was unavailable for this run."},
	}

	if financialResult.Success {
		if fd, ok := financialResult.Data.(types.FinancialData); ok {
			if fd.CompanyName != "" {
				companyName = fd.CompanyName
			}
			sections = append(sections, types.ReportSection{
				Title:   "Financial Snapshot",
				Icon:    "FINANCIALS",
				Content: fmt.Sprintf("- Ticker: %s\n- Currency: %s\n- Market cap: %s", fd.Ticker, fd.Currency, fd.MarketCap),
			})
		}
	}

	if newsResult.Success {
		if items, ok := newsResult.Data.([]types.NewsItem); ok && len(items) > 0 {
			var lines []string
			for i, n := range items {
				if i >= 3 {
					break
				}
				lines = append(lines, "- "+n.Title)
			}
			sections = append(sections, types.ReportSection{Title: "Recent News", Icon: "NEWS", Content: strings.Join(lines, "\n")})
		}
	}

	report := types.ResearchReport{
		Company:          companyName,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		OverallSentiment: "Neutral",
		Sections:         sections,
	}

	se.step(types.StepSynthesis, "Deterministic report assembled.", nil)
	se.emit(types.AgentMessage{Type: types.MsgReport, Report: &report})
	se.emit(types.AgentMessage{Type: types.MsgDone})
}

// Run executes the research loop for the given company, calling emit once
// per AgentMessage in order.
func Run(company string, emit func(types.AgentMessage)) {
	se := &stepEmitter{emit: emit}
	systemPrompt := createSystemPrompt(company)
	var history []llm.ConversationMessage

	se.step(types.StepPlan, fmt.Sprintf("Starting research on %s...", company), nil)

	initialPrompt := fmt.Sprintf(`Research "%s". Identify the correct stock ticker if one isn't given. Start with a short plan.`, company)
	history = append(history, llm.ConversationMessage{Role: "user", Text: initialPrompt})

	llmFailed := false
	iterations := 0
	for iterations < maxSteps {
		iterations++

		history = append(history, llm.ConversationMessage{
			Role: "user",
			Text: `Continue. Respond with the next action's JSON.`,
		})

		response, err := llm.CallGemini(systemPrompt, history)
		if err != nil {
			llmFailed = true
			break
		}
		history = append(history, llm.ConversationMessage{Role: "model", Text: response})

		actionData, ok := parseJSON(response)
		if !ok {
			se.step(types.StepError, "Could not parse the model's response. Retrying.", nil)
			history = append(history, llm.ConversationMessage{Role: "user", Text: "That wasn't valid JSON. Respond with only the JSON object."})
			continue
		}

		action, _ := actionData["action"].(string)

		switch action {
		case "synthesize":
			se.step(types.StepSynthesis, "Enough information gathered. Writing the report.", nil)
			iterations = maxSteps // exit the loop cleanly
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
				se.step(types.StepError, "Model requested an unknown tool: "+string(toolName), nil)
				history = append(history, llm.ConversationMessage{Role: "user", Text: "Unknown tool. Valid tools: search, financials, news."})
				continue
			}

			toolCall := types.ToolCall{Tool: toolName, Args: args, Reasoning: reasoning}
			se.step(types.StepToolCall, reasoning, &types.AgentStep{ToolCall: &toolCall})

			result := tools.Call(toolName, args)
			se.step(types.StepToolResult, fmt.Sprintf("%s success=%v", strings.ToUpper(string(toolName)), result.Success), &types.AgentStep{ToolResult: &result})

			dataJSON, _ := json.Marshal(result.Data)
			history = append(history, llm.ConversationMessage{
				Role: "user",
				Text: fmt.Sprintf("Tool result: %s", string(dataJSON)),
			})
		case "reflect":
			assessment, _ := actionData["assessment"].(string)
			se.step(types.StepReflection, assessment, nil)
		default:
			se.step(types.StepReflection, "Processing...", nil)
		}
	}

	if llmFailed {
		runDeterministicFallback(company, se)
		return
	}

	history = append(history, llm.ConversationMessage{Role: "user", Text: synthesisPrompt})
	synthesisResponse, err := llm.CallGemini(systemPrompt, history)
	if err != nil {
		runDeterministicFallback(company, se)
		return
	}

	reportData, ok := parseJSON(synthesisResponse)
	if !ok || reportData["sections"] == nil {
		runDeterministicFallback(company, se)
		return
	}

	report := types.ResearchReport{
		Company:          company,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		OverallSentiment: "Neutral",
	}
	if c, ok := reportData["company"].(string); ok && c != "" {
		report.Company = c
	}
	if s, ok := reportData["overallSentiment"].(string); ok && s != "" {
		report.OverallSentiment = s
	}
	if sections, ok := reportData["sections"].([]any); ok {
		for _, raw := range sections {
			secMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			sec := types.ReportSection{}
			if v, ok := secMap["title"].(string); ok {
				sec.Title = v
			}
			if v, ok := secMap["icon"].(string); ok {
				sec.Icon = v
			}
			if v, ok := secMap["content"].(string); ok {
				sec.Content = v
			}
			report.Sections = append(report.Sections, sec)
		}
	}

	se.emit(types.AgentMessage{Type: types.MsgReport, Report: &report})
	se.emit(types.AgentMessage{Type: types.MsgDone})
}
