# Week 2 : Web Search Tool

This week, I worked on adding the first real data source to the backend —
a web search tool.

## What I did

- Built a dispatcher (`internal/tools/tools.go`) that can call any tool by
  name, so new tools can be plugged in later without changing how they're
  called.
- Built the actual search tool (`internal/tools/search.go`), which looks up
  a query on the web using DuckDuckGo.
- Added a fallback to Google News RSS in case DuckDuckGo doesn't return
  anything, so the tool doesn't just fail outright if one source is down.
- Added `ToolName` and `ToolResult` to the shared types, so every tool
  reports back the same way (success or failure, plus its data).
- Tested the search tool directly by calling it with a few company names
  and confirming it returned real results.

At this stage, the search tool works fully on its own, but it isn't
connected to anything yet — there's no AI agent calling it, and no website
route for it either. It's just proven to work as a standalone piece before
the rest of the system is built around it.
