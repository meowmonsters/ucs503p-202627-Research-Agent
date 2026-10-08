# W4 : Research Agent Loop

## Objective

The objective of this week's work was to implement the core research agent loop for the backend. The agent coordinates the LLM and available research tools to gather information about a company and eventually produce a structured investment report.

The implementation also includes a deterministic fallback mode so that the system can still produce a report when the LLM is unavailable.

## Research Agent Structure

I implemented the research agent inside:

```text
code/backend/internal/agent/agent.go
```

The `agent` package is responsible for running the research workflow and coordinating the LLM with the available tools.

The agent uses the following components:

- LLM client
- Search tool
- Financials tool
- News tool
- Shared project types

## Research Workflow

The agent follows an iterative research process.

The overall flow is:

```text
Company
   |
   v
Create Research Plan
   |
   v
Ask LLM for Next Action
   |
   v
Parse JSON Response
   |
   +---- Tool Call ----> Execute Tool
   |                         |
   |                         v
   |                    Feed Result to LLM
   |                         |
   |                         v
   |                    Continue Research
   |
   +---- Reflection ----> Record Assessment
   |
   +---- Synthesize ----> Generate Report
   |
   v
Final Research Report
```

The research loop is limited to a maximum of 8 iterations.

## LLM System Prompt and Protocol

I added a system prompt that instructs the model to behave as a research analyst.

The research protocol requires the model to:

1. State a short research plan.
2. Call tools one at a time.
3. Evaluate tool results and determine whether additional research is required.
4. Synthesize the final report once enough information has been collected.

The model is instructed to respond using specific JSON action formats for planning, tool calls, reflection, and synthesis.

## Tool Integration

The agent supports three research tools:

```text
search
financials
news
```

I added validation so that the model can only request tools that are registered as valid.

For a valid tool call, the agent:

1. Extracts the tool name.
2. Extracts the reasoning.
3. Extracts the arguments.
4. Executes the corresponding tool.
5. Emits the tool result.
6. Adds the result to the LLM conversation history.

This allows the LLM to iteratively decide what information it needs.

## JSON Response Parsing

Since the LLM is expected to return JSON, I implemented JSON parsing logic.

The parser handles cases where the model returns:

- A plain JSON object.
- A JSON object wrapped inside a Markdown code fence.
- A response containing a JSON object that needs to be extracted.

If the response cannot be parsed, the agent records an error and asks the model to return only the JSON object before continuing.

## Agent Step Tracking

I implemented a `stepEmitter` structure to track and emit agent steps.

Each step receives an automatically incremented ID and a timestamp.

The agent can emit different types of steps, including:

- Plan
- Tool call
- Tool result
- Reflection
- Error
- Synthesis

This provides a structured representation of the research process while the agent is running.

## Deterministic Fallback

An important part of the implementation is the deterministic fallback mechanism.

If the LLM becomes unavailable, the agent switches to a deterministic research mode instead of failing completely.

The fallback directly calls:

1. Search
2. Financials
3. News

The collected raw tool data is then used to construct a basic research report without requiring the LLM.

The fallback also emits a reflection step indicating that deterministic research mode has been activated.

## Report Generation

After sufficient research has been completed, the agent sends a synthesis prompt to the LLM.

The expected report contains:

- Company Overview
- Financial Snapshot
- Recent News
- Risks
- Investment Summary

The report also contains an overall sentiment such as:

```text
Bullish
Neutral
Bearish
```

The generated JSON is parsed and converted into the project's `ResearchReport` structure.

## Error Handling

The implementation handles several failure cases:

- LLM request failure
- Invalid JSON returned by the model
- Unknown tool requested by the model
- Failure during report synthesis

When the LLM cannot continue the workflow, the deterministic fallback is used to ensure that a result can still be produced.

## Git Workflow

After implementing the research agent, I added:

```text
code/backend/internal/agent/agent.go
```

I committed the implementation with:

```text
feat: add research agent loop
```

The commit created was:

```text
b9fe6f9
```

Before pushing, I synchronized the branch using:

```text
git pull --rebase origin master
```

The latest team changes were successfully integrated through rebase.

The working tree was clean and my branch was ahead of `origin/master` by one commit, ready to be pushed.

## Key Learnings

- An LLM agent can be structured as an iterative decision-making loop rather than a single LLM request.
- Tool calling allows the model to gather external information before generating a final response.
- Strict JSON action formats make LLM-driven workflows easier to parse and control.
- Validating tool names prevents the model from invoking unsupported tools.
- Maintaining conversation history allows the LLM to reason over previously gathered information.
- A deterministic fallback improves system reliability when the LLM is unavailable.
- Step tracking provides a structured way to represent and monitor the agent's execution.

## Outcome

The backend now contains a functional research agent loop that:

- Creates a research plan.
- Interacts with the LLM.
- Dynamically selects research tools.
- Processes tool results.
- Maintains conversation history.
- Generates a structured investment report.
- Falls back to deterministic research when the LLM is unavailable.

The implementation was committed and successfully synchronized with the team's GitHub repository.
