# W5 : HTTP API Integration

## Objective

The objective of this week's work was to expose the backend research agent and its individual research tools through HTTP endpoints. This allows the frontend or other clients to send requests to the backend and receive JSON responses.

## HTTP API Handlers

I worked on the HTTP API handlers in:

```text
code/backend/internal/httpapi/handlers.go
```

The handlers connect the HTTP layer with the existing research agent and tools.

The API includes:
- A health endpoint to check whether the server is running.
- An endpoint to run the complete research agent for a company.
- Individual endpoints for search, financial data, and news.

## Request Validation

I added common input-reading and validation logic for POST requests.

The validation:
- Checks that the request uses the POST method.
- Limits the request body size to 1 MiB.
- Decodes the request body as JSON.
- Trims and cleans the supplied input.
- Rejects empty values.
- Rejects inputs longer than 100 characters.
- Rejects values that contain only symbols.

Invalid requests receive an appropriate JSON error response and HTTP status code.

## Connecting the Research Agent and Tools

The agent handler reads the company name, runs the research agent, collects the messages emitted during the run, and returns them in a JSON response.

I also added individual handlers for:
- Search, using the `query` field.
- Financials, using the `ticker` field.
- News, using the `company` field.

These endpoints make it possible to test each research data source separately from the complete agent workflow.

## CORS Configuration

I implemented CORS middleware so that a frontend hosted on a different origin can communicate with the backend.

The allowed frontend origins are configured through the `ALLOWED_ORIGIN` environment variable. The implementation supports multiple comma-separated origins, handles whitespace and trailing slashes, and responds to OPTIONS preflight requests.

If no origin is configured, the middleware allows any origin, which is useful for local development. For deployment, the frontend's actual origin can be configured.

## Error Handling and Reliability

I added several protections around API requests:

- A recovery middleware catches panics and returns a JSON 500 response.
- A panic inside the research agent is converted into an agent error message.
- Requests that fail validation receive clear client error responses.
- Failed tool calls return a gateway error response.
- A concurrency limit prevents too many research runs from executing at the same time.
- When all agent slots are occupied, additional requests receive HTTP 503 and a `Retry-After` header.

The maximum number of concurrent runs can be configured with `MAX_CONCURRENT_RUNS`; the default is four.

## Testing

I worked with the HTTP API handler tests in:

```text
code/backend/internal/httpapi/handlers_test.go
```

The tests cover health checks, CORS behaviour, input validation, incorrect HTTP methods, agent responses, panic recovery, tool endpoint validation, concurrent-run limits, and environment-based concurrency configuration.

## Git Workflow

The HTTP API implementation was prepared in the backend under:

```text
code/backend/internal/httpapi/
```

The Week 5 work was part of the HTTP API integration task:

```text
feat: wire HTTP API routes for agent and tools
```

## Key Learnings

- HTTP handlers provide a clear interface between a frontend and backend services.
- JSON request validation helps prevent malformed or unexpected input from reaching the application logic.
- CORS is needed when a browser frontend and backend are hosted on different origins.
- Middleware can be used for shared concerns such as CORS and panic recovery.
- Concurrency limits help protect services that make multiple external API calls.
- Handler tests help verify both successful requests and failure cases.

## Outcome

The backend HTTP API handlers expose the research agent and the individual search, financial, and news tools. The implementation includes request validation, configurable CORS, panic recovery, JSON error responses, and limits on concurrent agent runs.
