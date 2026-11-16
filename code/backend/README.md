# ResearchAgent Backend

The ResearchAgent backend is a Go HTTP API that coordinates an AI research agent with search, financial-data, and news tools. It accepts a company name or ticker, gathers relevant information, and returns the agent's research steps and structured report to the frontend.

## Features

- **Research agent:** Runs an iterative Plan → Act → Observe → Reflect → Synthesise workflow.
- **LLM integration:** Uses Gemini as the primary model and can use OpenRouter as a fallback when configured.
- **Research tools:** Provides search, financial-data, and company-news retrieval.
- **Fallback data sources:** Tool integrations can fall back to alternative/public sources when a preferred provider is unavailable.
- **HTTP API:** Exposes the complete agent workflow and individual tool endpoints as JSON.
- **Request protection:** Validates input, limits request-body size, recovers from handler panics, and limits concurrent agent runs.
- **CORS:** Allows the separately hosted frontend to call the backend.

## Technology

- Go
- Go standard-library HTTP server
- Gemini API (primary LLM)
- OpenRouter (optional LLM fallback)
- External/public search and financial-data sources

## Project Structure

```text
backend/
├── cmd/
│   └── server/          # Backend server entry point and route registration
├── internal/
│   ├── agent/            # Agent planning, tool orchestration and report synthesis
│   ├── httpapi/          # HTTP handlers, validation, CORS and recovery middleware
│   ├── llm/              # Gemini client and OpenRouter fallback
│   ├── tools/            # Search, financials, ticker resolution and news tools
│   └── types/            # Shared request, response and agent data structures
├── .env.example         # Example environment configuration
└── README.md
```

## Requirements

- Go installed and available on your `PATH`.
- A Gemini API key for the primary LLM integration.
- Optional API keys for the configured fallback/data providers.

## Configuration

From the backend directory, copy `.env.example` to `.env` and fill in the credentials you intend to use. Do not commit `.env` or publish API keys.

| Variable | Required | Description |
|---|---|---|
| `GEMINI_API_KEY` | Yes | API key for the primary Gemini LLM integration. |
| `OPENROUTER_API_KEY` | No | Enables the OpenRouter fallback if the direct Gemini call fails. |
| `TWELVEDATA_API_KEY` | No | Preferred financial-data source when configured; the tools can fall back to other sources. |
| `EXA_API_KEY` | No | Preferred search/news provider when configured; fallback sources are used when unavailable. |
| `ALLOWED_ORIGIN` | No | Comma-separated list of permitted frontend origins for CORS. If unset, the API allows any origin; set the actual frontend URL in deployment. |
| `MAX_CONCURRENT_RUNS` | No | Maximum number of simultaneous agent runs. Defaults to `4` when unset or invalid. |

The `.env` file is not loaded automatically by Go itself. Export the variables in your shell or configure them in your hosting provider's environment settings before starting the server.

## Run Locally

Run these commands from the backend directory (`code/backend`):

```bash
# Download dependencies
 go mod download

# Start the API server
 go run ./cmd/server
```

The server's listening address and port are defined by the server entry point / deployment configuration. Use the configured address when calling the API.

## API Overview

The API uses JSON request and response bodies. The HTTP handlers expose these operations (the exact URL paths are registered in `cmd/server/main.go`):

| Operation | Input JSON | Purpose |
|---|---|---|
| Health check | None | Checks whether the server is responding. |
| Research agent | `{"company":"Apple"}` | Runs the full research workflow and returns the emitted agent messages. |
| Search tool | `{"query":"Apple"}` | Runs the search tool independently. |
| Financials tool | `{"ticker":"AAPL"}` | Retrieves financial information independently. |
| News tool | `{"company":"Apple"}` | Retrieves company news independently. |

POST endpoints validate that the required field is present and non-empty, clean whitespace/control characters, and reject inputs longer than 100 characters. Request bodies are limited to 1 MiB. Invalid requests return a JSON error response.

The research-agent handler returns a JSON object containing a `messages` array. The array represents the agent's emitted steps and final report. Individual tool endpoints return their tool result as JSON.

## Reliability and Error Handling

- CORS middleware handles browser preflight (`OPTIONS`) requests and supports multiple configured origins.
- Recovery middleware converts unexpected handler panics into a JSON `500` response.
- The agent handler catches agent panics and emits an agent error message.
- Tool failures are returned as gateway errors (`502`).
- If the configured concurrent-run limit has been reached, a new agent request receives `503 Service Unavailable` with a `Retry-After` header.
- The default concurrent agent-run limit is four; configure `MAX_CONCURRENT_RUNS` to change it.

## Run Tests

From the backend directory:

```bash
 go test ./...
```

## Deployment

The backend is designed to run as a standalone Go web service. For Render or another hosting provider:

1. Configure the backend service to build and run from `code/backend`.
2. Set the required `GEMINI_API_KEY` in the service's environment variables.
3. Add optional provider keys as needed.
4. Set `ALLOWED_ORIGIN` to the deployed frontend's origin (not a wildcard) for production.
5. Set `MAX_CONCURRENT_RUNS` if the deployment requires a different concurrency limit.
6. Ensure the service uses the port provided by the hosting environment, as handled by the server entry point.

Never include real API keys in source control, screenshots, logs, or documentation.
