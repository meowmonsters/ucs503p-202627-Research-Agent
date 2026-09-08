package tools

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"analystagent/internal/types"
)

// Search looks up a query on the web. It tries DuckDuckGo's HTML endpoint
// first (with a couple of retries), and falls back to Google News RSS if
// that fails entirely — DuckDuckGo scraping is inherently a bit fragile
// since it depends on markup that isn't a documented, stable API.
func Search(query string) types.ToolResult {
	if query == "" {
		return types.ToolResult{Tool: types.ToolSearch, Success: false, Data: nil, Error: "Invalid search query parameter"}
	}

	if results, err := duckDuckGoSearchWithRetry(query, 2); err == nil {
		if len(results) > 5 {
			results = results[:5]
		}
		return types.ToolResult{Tool: types.ToolSearch, Success: true, Data: results}
	}

	if results, err := googleNewsFallback(query); err == nil {
		return types.ToolResult{Tool: types.ToolSearch, Success: true, Data: results}
	}

	// Both sources failed — still return a "successful" empty result rather
	// than an error, so a caller can decide how to handle "no results"
	// itself instead of treating it as a hard failure.
	return types.ToolResult{Tool: types.ToolSearch, Success: true, Data: []types.SearchResult{}}
}

func duckDuckGoSearchWithRetry(query string, maxRetries int) ([]types.SearchResult, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		results, err := duckDuckGoSearch(query)
		if err == nil {
			return results, nil
		}
		lastErr = err
		if attempt < maxRetries {
			time.Sleep(time.Duration(1000*(attempt+1)) * time.Millisecond)
		}
	}
	return nil, lastErr
}

// (?s) makes "." match newlines too — DuckDuckGo's result markup has real
// line breaks inside the anchor tags, so this is needed for the match to
// span them correctly.
var ddgResultRe = regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)</a>.*?<a[^>]*class="result__snippet"[^>]*>(.*?)</a>`)
var tagStripRe = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagStripRe.ReplaceAllString(s, "")))
}

// decodeDDGRedirect turns DuckDuckGo's outbound redirect URLs
// (//duckduckgo.com/l/?uddg=<encoded target>&...) into the real target URL.
func decodeDDGRedirect(raw string) string {
	raw = html.UnescapeString(raw)
	if strings.Contains(raw, "uddg=") {
		if u, err := url.Parse(raw); err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				return target
			}
		}
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}

func duckDuckGoSearch(query string) ([]types.SearchResult, error) {
	endpoint := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query) + "&kp=-2"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	matches := ddgResultRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no results parsed from duckduckgo response")
	}

	results := make([]types.SearchResult, 0, len(matches))
	for _, m := range matches {
		results = append(results, types.SearchResult{
			Title:   stripTags(m[2]),
			URL:     decodeDDGRedirect(m[1]),
			Snippet: stripTags(m[3]),
		})
	}
	return results, nil
}

var rssItemRe = regexp.MustCompile(`(?s)<item>(.*?)</item>`)
var rssTitleRe = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
var rssLinkRe = regexp.MustCompile(`(?s)<link>(.*?)</link>`)
var rssPubDateRe = regexp.MustCompile(`(?s)<pubDate>(.*?)</pubDate>`)

func googleNewsFallback(query string) ([]types.SearchResult, error) {
	rssURL := "https://news.google.com/rss/search?q=" + url.QueryEscape(query) + "&hl=en-IN&gl=IN&ceid=IN:en"

	resp, err := httpClient.Get(rssURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RSS fallback connection failed")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	xmlStr := string(bodyBytes)

	items := rssItemRe.FindAllStringSubmatch(xmlStr, -1)
	if len(items) > 5 {
		items = items[:5]
	}

	results := make([]types.SearchResult, 0, len(items))
	for _, item := range items {
		block := item[1]
		title := ""
		if m := rssTitleRe.FindStringSubmatch(block); m != nil {
			title = stripTags(m[1])
		}
		link := ""
		if m := rssLinkRe.FindStringSubmatch(block); m != nil {
			link = strings.TrimSpace(m[1])
		}
		pubDate := ""
		if m := rssPubDateRe.FindStringSubmatch(block); m != nil {
			pubDate = strings.TrimSpace(m[1])
		}

		// Titles often look like "Headline - Source"; strip the source off
		// the end for a cleaner search-result title.
		if idx := strings.LastIndex(title, " - "); idx > 0 {
			title = title[:idx]
		}

		results = append(results, types.SearchResult{
			Title:   title,
			URL:     link,
			Snippet: fmt.Sprintf("Record date: %s. Related to %s.", pubDate, query),
		})
	}
	return results, nil
}
