package tools

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"analystagent/internal/types"
)

// News fetches the latest headlines about a company from Google News RSS.
func News(company string) types.ToolResult {
	if company == "" {
		return types.ToolResult{Tool: types.ToolNews, Success: false, Data: nil, Error: "Missing or invalid 'company' parameter"}
	}

	rssURL := "https://news.google.com/rss/search?q=" + url.QueryEscape(company) + "&hl=en-IN&gl=IN&ceid=IN:en"
	resp, err := httpClient.Get(rssURL)
	if err != nil {
		return types.ToolResult{Tool: types.ToolNews, Success: false, Data: nil, Error: fmt.Sprintf("News fetch failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return types.ToolResult{Tool: types.ToolNews, Success: false, Data: nil, Error: fmt.Sprintf("News fetch failed: status %d", resp.StatusCode)}
	}

	bodyBytes := make([]byte, 0)
	buf := make([]byte, 8192)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			bodyBytes = append(bodyBytes, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	xmlStr := string(bodyBytes)

	items := rssItemRe.FindAllStringSubmatch(xmlStr, -1)
	if len(items) > 8 {
		items = items[:8]
	}

	newsItems := make([]types.NewsItem, 0, len(items))
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
		if pubDate == "" {
			pubDate = time.Now().UTC().Format(time.RFC1123Z)
		}

		// Titles usually look like "Headline - Source".
		source := "Google News"
		if idx := strings.LastIndex(title, " - "); idx > 0 {
			source = title[idx+3:]
			title = title[:idx]
		}

		newsItems = append(newsItems, types.NewsItem{
			Title:  title,
			Date:   pubDate,
			Source: source,
			URL:    link,
		})
	}

	return types.ToolResult{Tool: types.ToolNews, Success: true, Data: newsItems}
}
