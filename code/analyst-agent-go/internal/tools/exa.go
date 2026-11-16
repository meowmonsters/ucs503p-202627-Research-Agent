package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Exa (https://exa.ai) is an optional, higher-quality search backend. If
// EXA_API_KEY is set, search() and news() both try it first and fall back
// to DuckDuckGo / Google News RSS if it returns nothing — this is entirely
// optional and the rest of the pipeline works fine without it.

const exaAPIURL = "https://api.exa.ai/search"

func exaAPIKey() string {
	return os.Getenv("EXA_API_KEY")
}

type exaContents struct {
	Highlights struct {
		MaxCharacters int `json:"maxCharacters"`
	} `json:"highlights"`
	Summary bool `json:"summary"`
}

type exaRequest struct {
	Query      string      `json:"query"`
	Type       string      `json:"type"`
	NumResults int         `json:"numResults"`
	Category   string      `json:"category,omitempty"`
	Contents   exaContents `json:"contents"`
}

type exaResultRaw struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	PublishedDate string   `json:"publishedDate"`
	Author        string   `json:"author"`
	Highlights    []string `json:"highlights"`
	Summary       string   `json:"summary"`
}

type exaResponseRaw struct {
	Results []exaResultRaw `json:"results"`
}

// ExaResult is the normalized shape callers use, regardless of which Exa
// field the snippet actually came from.
type ExaResult struct {
	Title         string
	URL           string
	Snippet       string
	PublishedDate string
	Author        string
}

func fallbackSnippet(r exaResultRaw, query string) string {
	if r.Summary != "" {
		return r.Summary
	}
	if len(r.Highlights) > 0 {
		return strings.Join(r.Highlights, " ")
	}
	return fmt.Sprintf("Exa result for %s", query)
}

// exaSearch calls Exa's search API. category is optional ("news" or "").
func exaSearch(query string, numResults int, category string) ([]ExaResult, error) {
	apiKey := exaAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("EXA_API_KEY is not configured")
	}
	if numResults <= 0 {
		numResults = 5
	}

	reqBody := exaRequest{
		Query:      query,
		Type:       "auto",
		NumResults: numResults,
		Category:   category,
	}
	reqBody.Contents.Highlights.MaxCharacters = 280
	reqBody.Contents.Summary = true

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, exaAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Exa search failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	var parsed exaResponseRaw
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse Exa response: %w", err)
	}

	results := make([]ExaResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, ExaResult{
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       fallbackSnippet(r, query),
			PublishedDate: r.PublishedDate,
			Author:        r.Author,
		})
	}
	return results, nil
}
