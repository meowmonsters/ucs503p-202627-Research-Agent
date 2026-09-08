// Package tools holds the individual data-fetching tools (search,
// financials, news) that the agent will eventually call. Only "search" is
// implemented so far — financials and news get added once their own
// sources are wired up.
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
}

// Call dispatches to the named tool by name.
func Call(tool types.ToolName, args map[string]string) types.ToolResult {
	switch tool {
	case types.ToolSearch:
		return Search(args["query"])
	default:
		return types.ToolResult{
			Tool:    tool,
			Success: false,
			Data:    nil,
			Error:   "unknown tool: " + string(tool),
		}
	}
}
