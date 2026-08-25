# Montage Manager

Go + htmx app for `mm.machinemachine.ai`, backed by the fleet's local model.

## What it is

- **Public surface** (`/`) — hero, role perspectives, live offer pipeline, modules, roadmap.
- **Assistant** (`/assistant/*`) — an in-app chat that knows what this app is, running on the
  cluster's `deepseek-v4-flash-0731` (no data leaves the fleet). Its second job is to collect
  honest feedback about the app itself.
- **Dev loop** (`/dev`) — the feedback the assistant collected, clustered into a ranked backlog
  that feeds the next development iteration. Machine-readable at `/dev/backlog.json`.

## Layout

```
main.go                  wiring only
internal/model           shared domain types
internal/store           persistence port + SQLite + in-memory fallback
internal/llm             OpenAI-compatible client for the local vLLM endpoint
internal/web             public marketplace surface (htmx)
internal/assistant       chat + feedback capture
internal/devloop         feedback clustering, backlog, /dev
```

## Run

```sh
go run .                                   # DB_PATH=data/mm.db, LLM on the LAN endpoint
DB_PATH=:memory: PORT=8080 go run .        # throwaway state
docker build -t mm-machine . && docker run --rm -p 8080:8080 mm-machine
```

Environment: `PORT`, `DB_PATH`, `LLM_BASE_URL` (default `http://192.168.31.90:8000/v1`),
`LLM_MODEL` (default `deepseek-v4-flash-0731`), `LLM_API_KEY` (any placeholder — vLLM is open).

Health: `/healthz` (process), `/readyz` (round-trips the model).

## Local model notes

Use `192.168.31.90`, not `192.168.100.11` — the latter is the GPU fabric and is not routable
from m2 or the gateway. Keep `max_tokens >= 512`: the model spends ~33 output tokens on a hidden
reasoning preamble, so small caps return `content: null` with `finish_reason: length`.
