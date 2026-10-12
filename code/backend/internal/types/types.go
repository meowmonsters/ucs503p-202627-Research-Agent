// Package types holds the data structures that the backend's tools will
// return and the frontend will consume.
package types

type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type FinancialData struct {
	Ticker        string   `json:"ticker"`
	CompanyName   string   `json:"companyName"`
	Price         *float64 `json:"price"`
	Change        *float64 `json:"change"`
	ChangePercent *float64 `json:"changePercent"`
	Currency      string   `json:"currency"`
	MarketCap     string   `json:"marketCap"`
	PERatio       *float64 `json:"peRatio"`
	DividendYield *float64 `json:"dividendYield"`
	Revenue       string   `json:"revenue"`
}

type NewsItem struct {
	Title  string `json:"title"`
	Date   string `json:"date"`
	Source string `json:"source"`
	URL    string `json:"url"`
}

// ToolName identifies a callable tool.
type ToolName string

const (
	ToolSearch     ToolName = "search"
	ToolFinancials ToolName = "financials"
	ToolNews       ToolName = "news"
)

// ToolResult is the uniform shape every tool call returns, regardless of
// which tool it was or what kind of data it fetched.
type ToolResult struct {
	Tool    ToolName `json:"tool"`
	Success bool     `json:"success"`
	Data    any      `json:"data"`
	Error   string   `json:"error,omitempty"`
}

// --- Agent types ---
//
// These describe the agent's research loop itself: a tool call the agent
// decided to make, one step in its running log, and the final report it
// produces.

type ToolCall struct {
	Tool      ToolName          `json:"tool"`
	Args      map[string]string `json:"args"`
	Reasoning string            `json:"reasoning"`
}

type AgentStepType string

const (
	StepPlan       AgentStepType = "plan"
	StepToolCall   AgentStepType = "tool_call"
	StepToolResult AgentStepType = "tool_result"
	StepReflection AgentStepType = "reflection"
	StepSynthesis  AgentStepType = "synthesis"
	StepError      AgentStepType = "error"
)

// AgentStep is one entry in the agent's running log — the frontend renders
// a list of these as the research happens.
type AgentStep struct {
	ID         int           `json:"id"`
	Type       AgentStepType `json:"type"`
	Content    string        `json:"content"`
	ToolCall   *ToolCall     `json:"toolCall,omitempty"`
	ToolResult *ToolResult   `json:"toolResult,omitempty"`
	Timestamp  int64         `json:"timestamp"`
}

// --- Report types ---

type ReportSection struct {
	Title   string `json:"title"`
	Icon    string `json:"icon"`
	Content string `json:"content"`
}

type ResearchReport struct {
	Company          string          `json:"company"`
	GeneratedAt      string          `json:"generatedAt"`
	Sections         []ReportSection `json:"sections"`
	OverallSentiment string          `json:"overallSentiment"` // "Bullish" | "Neutral" | "Bearish"
}

// --- Streaming / message types ---
//
// The agent emits a sequence of these as it runs: a step at a time, then a
// final report, then a "done" marker (or an error if something went wrong).

type AgentMessageType string

const (
	MsgStep   AgentMessageType = "step"
	MsgReport AgentMessageType = "report"
	MsgError  AgentMessageType = "error"
	MsgDone   AgentMessageType = "done"
)

type AgentMessage struct {
	Type   AgentMessageType `json:"type"`
	Step   *AgentStep       `json:"step,omitempty"`
	Report *ResearchReport  `json:"report,omitempty"`
	Error  string           `json:"error,omitempty"`
}
