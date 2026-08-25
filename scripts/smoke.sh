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

echo "languages:"
check "German by default"   "Was brauchen Sie"  "$(curl -fsS "$BASE/")"
check "German page marked"  'lang="de"'         "$(curl -fsS "$BASE/")"
check "English on request"  "What do you need"  "$(curl -fsS -H 'Accept-Language: en' "$BASE/")"
check "example profiles"    "demo/enter"        "$(curl -fsS "$BASE/demo")"

echo "example personas:"
PJAR="$TMP/persona"
curl -fsS -c "$PJAR" -b "$PJAR" -X POST "$BASE/demo/enter" -d "persona=gu-muenchen" -o /dev/null
PERSONA_HOME="$(curl -fsS -b "$PJAR" "$BASE/")"
check "persona banner"      "München"           "$PERSONA_HOME"
CREWS="$(curl -fsS -c "$PJAR" -b "$PJAR" -X POST "$BASE/ask" \
  --data-urlencode "message=6 Monteure für Trockenbau in Hamburg ab September, A1 vorhanden" --max-time 240)"
check "crew search (DE)"    "mm-why"            "$CREWS"
check "German reasons"      "Gewerk passt"      "$CREWS"

echo "onboarding + search (live model):"
JAR="$TMP/cookies"
ASK1="$(curl -fsS -c "$JAR" -b "$JAR" -X POST "$BASE/ask" \
  --data-urlencode "message=wir sind 8 Leute, Elektro und Trockenbau, Raum München, A1 liegt vor, ab Oktober frei" --max-time 240)"
check "onboarding learned"  "mm-chip"         "$ASK1"
ASK2="$(curl -fsS -c "$JAR" -b "$JAR" -X POST "$BASE/ask" \
  --data-urlencode "message=offene Stahlbauaufträge in den Niederlanden" --max-time 240)"
check "search ranked"       "mm-results"      "$ASK2"
check "match explained"     "mm-why"          "$ASK2"
check "shell prompt"        "mm-prompt"       "$(curl -fsS "$BASE/")"
check "about page kept"     "Montage Manager" "$(curl -fsS "$BASE/about")"
check "pipeline in German"  "Auftrag anlegen" "$(curl -fsS "$BASE/offers?view=all")"

echo "assistant (live model):"
PANEL="$(curl -fsS "$BASE/assistant/panel?role=owner&route=home")"
check "panel renders"      "assistant-panel" "$PANEL"
CONV="$(printf '%s' "$PANEL" | sed -n 's/.*data-conversation="\([^"]*\)".*/\1/p' | head -1)"
ANSWER="$(curl -fsS -X POST "$BASE/assistant/message" \
  --data-urlencode "conversation=$CONV" \
  --data-urlencode "role=owner" \
  --data-urlencode "route=home" \
  --data-urlencode "message=Wie funktioniert das hier?" --max-time 240)"
check "model answered"     "mm-msg mm"       "$ANSWER"
if [[ "$ANSWER" == *"did not answer"* || "$ANSWER" == *"empty answer"* ]]; then
  printf '  FAIL  model returned an error bubble\n'; FAILED=1
fi

echo "feedback loop:"
curl -fsS -X POST "$BASE/feedback" \
  --data-urlencode "conversation=$CONV" \
  --data-urlencode "kind=confusion" \
  --data-urlencode "verbatim=Die Filter in der Auftragsliste sind auf dem Handy schwer zu finden." \
  --data-urlencode "role=owner" --data-urlencode "route=home" >/dev/null
check "feedback stored"    "Handy"           "$(curl -fsS "$BASE/dev")"
curl -fsS -X POST "$BASE/dev/refresh" --max-time 240 >/dev/null
check "backlog generated"  "backlog-item"    "$(curl -fsS "$BASE/dev")"

echo
if [[ "$FAILED" == 0 ]]; then
  echo "SMOKE OK"
else
  echo "SMOKE FAILED — app log:"; tail -20 "$TMP/app.log"
fi
exit "$FAILED"
