// Package tools holds the individual data-fetching tools (search,
// financials, news) that the agent will eventually call.
package tools

import (
	"net/http"
	"time"

	"analystagent/internal/types"
)

var httpClient = &http.Client{Timeout: 20 * time.Second}

// ToolParameter describes one argument a tool accepts.
type ToolParameter struct {
	Name        string
	Type        string
	Description string
}

// ToolDefinition describes a tool well enough that an LLM could be told
// about it later (once the agent loop exists) and decide when to call it.
type ToolDefinition struct {
	Name        types.ToolName
	Description string
	Parameters  []ToolParameter
}

// Definitions lists every tool currently available.
var Definitions = []ToolDefinition{
	{
		Name: types.ToolSearch,
		Description: "Search the web using DuckDuckGo. Use this for company " +
			"overviews, competitor information, or general research.",
		Parameters: []ToolParameter{
			{Name: "query", Type: "string", Description: "The search query to look up"},
		},
	},
	{
		Name: types.ToolFinancials,
		Description: "Get financial data for a publicly traded company using its " +
			"stock ticker symbol. Returns price, market cap, P/E ratio, and revenue.",
		Parameters: []ToolParameter{
			{Name: "ticker", Type: "string", Description: "The stock ticker symbol (e.g., AAPL for Apple, MSFT for Microsoft)"},
		},
	},
	{
		Name:        types.ToolNews,
		Description: "Get the latest news headlines about a company from Google News.",
		Parameters: []ToolParameter{
			{Name: "company", Type: "string", Description: "The company name to search news for"},
		},
	},
}

// Call dispatches to the named tool by name.
func Call(tool types.ToolName, args map[string]string) types.ToolResult {
	switch tool {
	case types.ToolSearch:
		return Search(args["query"])
	case types.ToolFinancials:
		return Financials(args["ticker"])
	case types.ToolNews:
		return News(args["company"])
	default:
		return types.ToolResult{
			Tool:    tool,
			Success: false,
			Data:    nil,
			Error:   "unknown tool: " + string(tool),
		}
	}
}
