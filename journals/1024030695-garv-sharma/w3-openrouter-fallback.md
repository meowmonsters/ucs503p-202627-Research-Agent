# W3 : OpenRouter Fallback Integration

## Objective

The objective of this week's work was to improve the backend LLM client by adding a backup provider through OpenRouter. The existing implementation directly called Google's Gemini API. The new implementation keeps Gemini as the primary provider and uses OpenRouter as a fallback when the direct Gemini request fails and an OpenRouter API key is configured.

## Understanding the Existing LLM Client

Before making the change, I reviewed the existing `llm.go` implementation in the backend.

The existing client:
- Used Google's Gemini API directly.
- Read the API key from the `GEMINI_API_KEY` environment variable.
- Sent the system prompt and conversation history to Gemini.
- Used a temperature of `0.4`, a maximum output of `2048` tokens, and `topP` of `0.9`.
- Parsed the Gemini response and returned the generated text.

## OpenRouter Fallback Design

I extended the existing LLM client instead of replacing the Gemini implementation.

The updated flow is:

```text
CallGemini()
     |
     v
Direct Gemini API
     |
     +---- success ----> Return response
     |
     +---- failure
              |
              v
       OPENROUTER_API_KEY configured?
              |
          +---+---+
         Yes     No
          |       |
          v       v
     OpenRouter   Return
       request    original error
          |
          v
     Return response
```

Gemini remains the primary path, while OpenRouter provides a backup path.

## Implementation

I introduced the OpenRouter endpoint and model configuration:

- OpenRouter API endpoint: `https://openrouter.ai/api/v1/chat/completions`
- OpenRouter model: `google/gemini-2.0-flash-001`

I added request and response structures required by the OpenRouter chat-completions API.

The conversation history is converted to OpenRouter's message format:
- `user` remains `user`
- Gemini's `model` role is mapped to OpenRouter's `assistant`
- The system prompt is sent as a `system` message

The OpenRouter request uses:
- Temperature: `0.4`
- Maximum tokens: `2048`
- Bearer-token authentication using `OPENROUTER_API_KEY`

## Fallback Logic

The original Gemini request was separated into a helper function called `callDirectGemini`.

A separate `callOpenRouter` helper handles the backup provider.

`CallGemini` now:

1. Attempts the direct Gemini request first.
2. If Gemini succeeds, returns the Gemini response.
3. If Gemini fails and `OPENROUTER_API_KEY` is available, attempts the OpenRouter request.
4. If no OpenRouter key is configured, returns the original Gemini error.

This keeps the existing Gemini behavior as the primary path while providing an additional recovery path.

## Error Handling

The implementation checks HTTP status codes for both providers.

For unsuccessful responses, the response body is included in the returned error to make API failures easier to diagnose.

The implementation also handles:
- HTTP request errors
- Response body read errors
- JSON parsing errors
- Empty Gemini responses
- Empty OpenRouter responses
- Missing API keys

## Git Workflow

After implementing the OpenRouter fallback, I checked the changes using Git and staged the modified `llm.go`.

I committed the work with:

```text
feat: add OpenRouter fallback to LLM client
```

The commit created was:

```text
aae7487
```

Before pushing, I ran:

```text
git pull --rebase origin master
```

The branch was already up to date, so there were no conflicts with teammates' changes.

I then pushed the commit to the shared `master` branch successfully.

## Key Learnings

- A fallback provider can improve the reliability of an LLM integration.
- Keeping the primary provider unchanged makes the new functionality easier to integrate.
- Different LLM providers can expose different request formats, so conversation roles need to be mapped appropriately.
- Environment variables can be used to keep API credentials outside the source code.
- Separating provider-specific logic into helper functions keeps the main LLM interface simpler.
- Pulling with rebase before pushing helps keep changes synchronized when working on a shared branch.

## Outcome

The backend LLM client now supports a two-level provider flow:

**Primary:** Direct Google Gemini API

**Fallback:** OpenRouter-routed Gemini model

The implementation was committed and successfully pushed to the team's GitHub repository.
