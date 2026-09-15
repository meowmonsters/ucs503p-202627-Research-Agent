# Week 2 : Gemini LLM Client

## Objective

The objective for Week 2 was to implement the initial LLM integration for the backend and establish a basic client for communicating with Google's Gemini API.

## LLM Client Development

I worked on the backend LLM integration by adding a dedicated `llm` package under:

```text
code/backend/internal/llm/
```

The package contains:

```text
code/backend/internal/llm/llm.go
```

The implementation provides an initial Gemini API client for sending requests to the language model.

A `ConversationMessage` structure was defined to represent individual conversation turns, with support for `user` and `model` roles.

The implementation also defines the required request and response structures for communicating with the Gemini API.

## Gemini API Integration

The main function implemented was:

```go
CallGemini(systemPrompt string, history []ConversationMessage)
```

The function accepts a system prompt and conversation history and returns the model's generated text response.

The Gemini API key is obtained through the `GEMINI_API_KEY` environment variable rather than being hard-coded into the application.

The client sends an HTTP POST request to the Gemini API with the system instruction, conversation history, and generation configuration.

The initial generation configuration includes:

- Temperature: `0.4`
- Maximum output tokens: `2048`
- Top P: `0.9`

The response is parsed from the Gemini API response structure and the generated text is returned to the caller.

Basic error handling was also included for cases such as a missing API key, HTTP request failures, unsuccessful API responses, response parsing failures, and empty model responses.

## Git Workflow

After adding the LLM client, I checked the repository status to verify that only the intended new backend package was being tracked.

The file was staged using:

```bash
git add code/backend/internal/llm/llm.go
```

The changes were committed with the message:

```text
feat: add Gemini LLM client
```

The commit was successfully created and then pushed to the shared GitHub repository using:

```bash
git push origin master
```

After pushing, I verified the repository status and confirmed that the local branch was up to date with `origin/master` and that the working tree was clean.

## Key Learnings

- Learned how an LLM client can be integrated into a Go backend.
- Understood how to structure request and response data for an external API.
- Learned how conversation history can be represented and passed to an LLM.
- Understood the importance of using environment variables for API credentials.
- Learned how HTTP requests can be used to communicate with an external API from Go.
- Improved understanding of JSON marshaling and unmarshaling in Go.
- Practiced Git staging, committing, and pushing changes to a shared repository.
- Learned the importance of verifying the repository status after completing a change.

## Outcome

The initial Gemini LLM client was successfully added to the backend under:

```text
code/backend/internal/llm/llm.go
```

The client provides the basic functionality required to send a system prompt and conversation history to Gemini and receive the generated response.

The implementation was committed and successfully pushed to the shared GitHub repository, completing the Week 2 LLM integration task.
