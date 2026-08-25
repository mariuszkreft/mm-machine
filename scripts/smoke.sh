#!/usr/bin/env bash
# End-to-end smoke test: builds the app, runs it against the live local model,
# and checks every surface that has to work for the dev loop to close.
set -euo pipefail

PORT="${PORT:-8231}"
BASE="http://127.0.0.1:${PORT}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
FAILED=0

cleanup() {
  [[ -n "${APP_PID:-}" ]] && kill "$APP_PID" 2>/dev/null || true
  rm -rf "$TMP"
}
trap cleanup EXIT

check() { # check <name> <expected-substring-or-code> <actual>
  if [[ "$3" == *"$2"* ]]; then
    printf '  ok    %s\n' "$1"
  else
    printf '  FAIL  %s (wanted %q, got %q)\n' "$1" "$2" "${3:0:200}"
    FAILED=1
  fi
}

echo "building…"
( cd "$ROOT" && go build -o "$TMP/mm" . )

echo "starting on :$PORT (db $TMP/mm.db)…"
DB_PATH="$TMP/mm.db" PORT="$PORT" "$TMP/mm" > "$TMP/app.log" 2>&1 &
APP_PID=$!
for _ in $(seq 1 40); do
  curl -fsS "$BASE/healthz" >/dev/null 2>&1 && break
  sleep 0.25
done

echo "surfaces:"
check "home renders"       "Montage Manager" "$(curl -fsS "$BASE/")"
check "pipeline partial"   "id=\"offers\""   "$(curl -fsS "$BASE/offers?view=open")"
check "perspective swap"   "perspective-panel" "$(curl -fsS "$BASE/perspective?role=executor")"
check "dev loop page"      "backlog"         "$(curl -fsS "$BASE/dev")"
BLJSON="$(curl -fsS "$BASE/dev/backlog.json")"
# an empty backlog must serialize as [] — Go encodes a nil slice as null, which
# breaks JSON consumers, so accept only a real array here.
check "backlog json"       "["               "$BLJSON"

echo "persistence:"
curl -fsS -X POST "$BASE/offers/new" \
  -d "title=Smoke test offer&location=Testville&category=QA&status=open" >/dev/null
check "offer created"      "Smoke test offer" "$(curl -fsS "$BASE/offers?view=open")"

echo "onboarding + search (live model):"
JAR="$TMP/cookies"
ASK1="$(curl -fsS -c "$JAR" -b "$JAR" -X POST "$BASE/ask" \
  --data-urlencode "message=I need 6 electrical fitters in Munich for 3 weeks, A1 ready" --max-time 180)"
check "onboarding learned"  "profile"         "$ASK1"
ASK2="$(curl -fsS -c "$JAR" -b "$JAR" -X POST "$BASE/ask" \
  --data-urlencode "message=show me open steel jobs in the Netherlands" --max-time 180)"
check "search ranked"       "mm-results"      "$ASK2"
check "match explained"     "mm-why"          "$ASK2"
check "shell prompt"        "mm-prompt"       "$(curl -fsS "$BASE/")"
check "about page kept"     "Montage Manager" "$(curl -fsS "$BASE/about")"

echo "assistant (live model):"
PANEL="$(curl -fsS "$BASE/assistant/panel?role=owner&route=home")"
check "panel renders"      "assistant-panel" "$PANEL"
CONV="$(printf '%s' "$PANEL" | sed -n 's/.*data-conversation="\([^"]*\)".*/\1/p' | head -1)"
ANSWER="$(curl -fsS -X POST "$BASE/assistant/message" \
  --data-urlencode "conversation=$CONV" \
  --data-urlencode "role=owner" \
  --data-urlencode "route=home" \
  --data-urlencode "message=In one sentence: what is this app for?" --max-time 180)"
check "model answered"     "bubble assistant" "$ANSWER"
if [[ "$ANSWER" == *"did not answer"* || "$ANSWER" == *"empty answer"* ]]; then
  printf '  FAIL  model returned an error bubble\n'; FAILED=1
fi

echo "feedback loop:"
curl -fsS -X POST "$BASE/feedback" \
  --data-urlencode "conversation=$CONV" \
  --data-urlencode "kind=confusion" \
  --data-urlencode "verbatim=The pipeline filters are not obvious on mobile." \
  --data-urlencode "role=owner" --data-urlencode "route=home" >/dev/null
check "feedback stored"    "pipeline filters" "$(curl -fsS "$BASE/dev")"
curl -fsS -X POST "$BASE/dev/refresh" --max-time 240 >/dev/null
check "backlog generated"  "backlog-item"    "$(curl -fsS "$BASE/dev")"

echo
if [[ "$FAILED" == 0 ]]; then
  echo "SMOKE OK"
else
  echo "SMOKE FAILED — app log:"; tail -20 "$TMP/app.log"
fi
exit "$FAILED"
