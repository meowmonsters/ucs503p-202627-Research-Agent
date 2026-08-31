// Package types holds the data structures that the backend's tools will
// return and the frontend will consume. This is a Week 1 skeleton: just the
// shapes for the three tools we know we'll need (search, financials, news).
// Agent-loop types (steps, messages, reports) get added once the agent
// itself is built.
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
}

type NewsItem struct {
	Title  string `json:"title"`
	Date   string `json:"date"`
	Source string `json:"source"`
	URL    string `json:"url"`
}
