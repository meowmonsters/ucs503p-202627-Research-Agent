// Package httpapi exposes the agent and the individual tools over
// HTTP/JSON, so the React frontend (or curl) can call them.
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"analystagent/internal/agent"
	"analystagent/internal/tools"
	"analystagent/internal/types"
)

const (
	maxBodyBytes  = 1 << 20 // 1 MiB — request bodies here are tiny
	maxInputRunes = 100     // longest company name / ticker / query accepted
)

// agentRunner is a variable (not a direct call) so tests can swap in a fake
// agent instead of making real LLM and network calls.
var agentRunner = agent.Run

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// allowedOrigins parses ALLOWED_ORIGIN: a comma-separated list of frontend
// origins. Whitespace and trailing slashes are ignored, because
// "https://app.onrender.com/" is the most common way to mistype this value
// and a browser's Origin header never has a trailing slash, so it would
// otherwise never match. An empty value (or "*") means "any origin".
func allowedOrigins() []string {
	var out []string
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGIN"), ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			out = append(out, o)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	for _, o := range out {
		if o == "*" {
			return []string{"*"}
		}
	}
	return out
}

// CORS adds cross-origin headers so a frontend served from a different
// origin (e.g. the Vite dev server on :5173) can call this API. The
// allowed origin(s) come from ALLOWED_ORIGIN and default to "*", which is
// fine for local development but should be set to the real frontend URL
// when deployed. With several origins configured, the request's own Origin
// is echoed back only if it is on the list. Preflight (OPTIONS) requests
// are answered here and never reach the real handlers.
func CORS(next http.Handler) http.Handler {
	origins := allowedOrigins()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case origins[0] == "*":
			w.Header().Set("Access-Control-Allow-Origin", "*")
		case len(origins) == 1:
			w.Header().Set("Access-Control-Allow-Origin", origins[0])
			w.Header().Set("Vary", "Origin")
		default:
			w.Header().Add("Vary", "Origin")
			reqOrigin := r.Header.Get("Origin")
			for _, o := range origins {
				if o == reqOrigin {
					w.Header().Set("Access-Control-Allow-Origin", o)
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Recover turns a panic in any handler into a clean JSON 500 instead of a
// dropped connection, and logs it. (The agent has its own, finer-grained
// recovery; this covers the direct tool endpoints and anything else.)
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, rec)
				writeError(w, http.StatusInternalServerError, "Internal server error.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- Limiting concurrent agent runs ---
//
// One agent run makes several LLM and network calls, and the LLM key is
// shared by everyone who can reach the server. Capping how many runs can be
// in flight at once keeps a burst of requests (or a script) from exhausting
// the API quota or the instance's memory. Extra requests get a fast 503
// with Retry-After instead of queueing until they time out.

const defaultMaxConcurrentRuns = 4

func maxConcurrentRuns() int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MAX_CONCURRENT_RUNS"))); err == nil && n > 0 {
		return n
	}
	return defaultMaxConcurrentRuns
}

// agentSlots is a semaphore; tests may replace it with a smaller one.
var agentSlots = make(chan struct{}, maxConcurrentRuns())

// HandleHealth reports that the server is up.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// cleanInput trims the value, turns control characters (newlines, tabs,
// NULs…) into spaces and collapses runs of whitespace. The value ends up in
// LLM prompts and logs, where a pasted newline could otherwise smuggle in
// extra "instructions" or forge log lines.
func cleanInput(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// readField reads a JSON body like {"<field>": "value"} and returns the
// trimmed value. On any problem it writes a 4xx response and returns false.
func readField(w http.ResponseWriter, r *http.Request, field string) (string, bool) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "Use POST for this endpoint.")
		return "", false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, `Request body must be JSON like {"`+field+`": "..."}.`)
		return "", false
	}

	value, _ := body[field].(string)
	value = cleanInput(value)
	if value == "" {
		writeError(w, http.StatusBadRequest, "Missing or empty '"+field+"' parameter.")
		return "", false
	}
	if utf8.RuneCountInString(value) > maxInputRunes {
		writeError(w, http.StatusBadRequest, "'"+field+"' is too long.")
		return "", false
	}
	if !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		writeError(w, http.StatusBadRequest, "'"+field+"' must contain a company name or ticker, not just symbols.")
		return "", false
	}
	return value, true
}

// runAgent runs the agent and collects every message it emits. A panic
// inside the agent is converted into an error message instead of crashing
// the request.
func runAgent(company string) (messages []types.AgentMessage) {
	messages = []types.AgentMessage{}
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("agent panic for %q: %v", company, rec)
			messages = append(messages, types.AgentMessage{
				Type:  types.MsgError,
				Error: "The research agent hit an internal error.",
			})
		}
	}()
	agentRunner(company, func(m types.AgentMessage) {
		messages = append(messages, m)
	})
	return messages
}

// HandleAgent runs the full research loop for a company and returns
// {"messages": [...]} — every step the agent took, then the report.
//
// Note this is synchronous: the response is only sent once the whole run
// finishes, so the frontend sees nothing until then.
func HandleAgent(w http.ResponseWriter, r *http.Request) {
	company, ok := readField(w, r, "company")
	if !ok {
		return
	}

	select {
	case agentSlots <- struct{}{}:
		defer func() { <-agentSlots }()
	default:
		w.Header().Set("Retry-After", "15")
		writeError(w, http.StatusServiceUnavailable,
			"The server is busy with other research requests. Please try again in a few seconds.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": runAgent(company)})
}

// toolHandler builds a handler for one tool: it reads a single string
// field from the body, calls the tool, and returns the tool's result.
func toolHandler(field string, call func(string) types.ToolResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		value, ok := readField(w, r, field)
		if !ok {
			return
		}
		result := call(value)
		status := http.StatusOK
		if !result.Success {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, result)
	}
}

// HandleToolSearch, HandleToolFinancials and HandleToolNews expose the
// individual tools directly, which is handy for testing a single data
// source without running the whole agent.
var (
	HandleToolSearch     = toolHandler("query", tools.Search)
	HandleToolFinancials = toolHandler("ticker", tools.Financials)
	HandleToolNews       = toolHandler("company", tools.News)
)
