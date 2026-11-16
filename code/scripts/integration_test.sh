#!/usr/bin/env bash
# End-to-end smoke test for the backend. Builds and starts the real server,
# then calls it over HTTP the same way the browser does.
#
# Needs: go, curl, python3.   Run from anywhere:  ./scripts/integration_test.sh
#
# With no GEMINI_API_KEY the agent uses its deterministic fallback, so this
# passes offline too. With a key set it exercises the real AI path (slower).
set -u

PORT="${PORT:-18080}"
BASE="http://localhost:${PORT}"
ORIGIN="http://localhost:5173"   # where the Vite dev server runs
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG="$(mktemp)"
BIN="$(mktemp)"

cd "$ROOT/analyst-agent-go" || exit 1
go build -o "$BIN" ./cmd/server || { echo "build failed"; exit 1; }

PORT="$PORT" "$BIN" >"$LOG" 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null; rm -f "$BIN"' EXIT

for _ in $(seq 1 40); do
  curl -sf "$BASE/healthz" >/dev/null && break
  sleep 0.25
done

pass=0; fail=0
check() { # check "<description>" <command that succeeds when the check passes...>
  local desc="$1"; shift
  if "$@" >/dev/null 2>&1; then echo "  PASS  $desc"; pass=$((pass+1))
  else echo "  FAIL  $desc"; fail=$((fail+1)); fi
}
status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
is() { [ "$1" = "$2" ]; }

echo "Backend integration test against $BASE"

check "GET /healthz returns 200" is "$(status "$BASE/healthz")" 200

check "CORS preflight returns 204" \
  is "$(status -X OPTIONS -H "Origin: $ORIGIN" -H 'Access-Control-Request-Method: POST' "$BASE/api/agent")" 204
check "CORS preflight allows the frontend's origin" \
  bash -c "curl -si -X OPTIONS -H 'Origin: $ORIGIN' '$BASE/api/agent' | grep -qi '^access-control-allow-origin:'"

check "POST /api/agent with no company is rejected (400)" \
  is "$(status -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/agent")" 400
check "POST /api/agent with invalid JSON is rejected (400)" \
  is "$(status -X POST -H 'Content-Type: application/json' -d 'nope' "$BASE/api/agent")" 400
check "GET /api/agent is rejected (405)" is "$(status "$BASE/api/agent")" 405

check "POST /api/agent with only symbols is rejected (400)" \
  is "$(status -X POST -H 'Content-Type: application/json' -d '{"company":"???"}' "$BASE/api/agent")" 400
check "POST /api/agent with a 101-character name is rejected (400)" \
  is "$(status -X POST -H 'Content-Type: application/json' -d "{\"company\":\"$(printf 'a%.0s' $(seq 1 101))\"}" "$BASE/api/agent")" 400
check "unknown company on the financials tool is a readable 502, not a crash" \
  bash -c "curl -s -X POST -H 'Content-Type: application/json' -d '{\"ticker\":\"Zzzz Not A Company Qqqq\"}' '$BASE/api/tools/financials' | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d[\"success\"] is False and d[\"error\"]'"

echo "  ...running the agent for a real request (may take a while)"
BODY="$(curl -s --max-time 240 -X POST -H 'Content-Type: application/json' -H "Origin: $ORIGIN" \
  -d '{"company":"Apple"}' "$BASE/api/agent")"

has() { echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); $1"; }
check "agent response is JSON with a messages array" has "assert isinstance(d['messages'], list) and d['messages']"
check "agent emitted step messages"                  has "assert any(m['type']=='step' for m in d['messages'])"
check "agent emitted a report"                       has "assert any(m['type']=='report' for m in d['messages'])"
check "agent finished with a done message"           has "assert d['messages'][-1]['type']=='done'"
check "report has sections, each with title and content" \
  has "r=[m for m in d['messages'] if m['type']=='report'][0]['report']; assert r['sections'] and all(s['title'] and s['content'] for s in r['sections'])"
check "report sentiment is one the UI can display" \
  has "r=[m for m in d['messages'] if m['type']=='report'][0]['report']; assert r['overallSentiment'] in ('Bullish','Neutral','Bearish')"

SECTIONS="$(echo "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len([m for m in d['messages'] if m['type']=='report'][0]['report']['sections']))" 2>/dev/null)"
echo "  info  report sections: ${SECTIONS:-?} (the proposal specifies six)"

echo
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
