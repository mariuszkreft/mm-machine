# Montage Manager — design system contract

One surface, one language. `internal/web/static/base.css` owns the tokens and primitives;
every package styles itself with them. A package may add a CSS file for what is genuinely
local (`onboarding.css`, `search.css`, `dev.css`), but must not redefine a token or restyle
a primitive — if a primitive is wrong, fix it in `base.css` so every surface moves together.

## First principles

1. **The prompt is the interface.** A visitor arrives with a need in their own words. The
   model translates; deterministic code filters and ranks. Never make a human fill in a
   taxonomy the machine could have inferred.
2. **Results live in the thread.** Search results, onboarding questions and assistant answers
   are all cards and messages in one conversation, not separate page states.
3. **Every machine number is explained next to it.** A fit score without `.mm-why` is a bug.
4. **Progressive disclosure.** Ask only what cannot be inferred, one thing at a time, and
   never re-ask what the profile already knows.
5. **Quiet chrome, loud content.** Type and spacing carry hierarchy. No decorative gradients,
   no marketing copy on a working surface.
6. **Degrade honestly.** When the model is unavailable, the surface still works mechanically
   and says so; it never shows a spinner that cannot end.

## Tokens

Colour: `--mm-bg --mm-surface --mm-surface-2 --mm-ink --mm-ink-soft --mm-muted --mm-line
--mm-line-strong --mm-accent --mm-accent-soft --mm-accent-ink --mm-good --mm-warn --mm-bad`
(all redefined under `prefers-color-scheme: dark` — never hardcode a hex outside `base.css`).

Type: `--mm-font --mm-mono --mm-text-xs|sm|md|lg|xl|2xl`.
Space: `--mm-1 … --mm-8` (4px scale). Shape: `--mm-r-sm|md|lg|pill`, `--mm-shadow`,
`--mm-shadow-lift`. Layout: `--mm-measure` (reading width), `--mm-max` (page width).

## Primitives

| class | use |
|---|---|
| `.mm-shell` `.mm-top` `.mm-main` `.mm-foot` | page frame |
| `.mm-brand` `.mm-brand-mark` | wordmark |
| `.mm-ask` `.mm-prompt` `.mm-suggest` | the prompt and its example chips |
| `.mm-thread` `.mm-msg.you` `.mm-msg.mm` `.mm-who` | conversation |
| `.mm-cards` `.mm-card` `.mm-card-head` `.mm-fit` `.mm-why` | result cards |
| `.mm-btn` (`.ghost` `.quiet`) `.mm-chip` (`.on`) `.mm-badge` (`.good` `.warn` `.bad`) | controls |
| `.mm-panel` `.mm-field` `.mm-meter` `.mm-empty` `.mm-skel` | containers, inputs, states |
| `.mm-muted` `.mm-mono` | text helpers |

## Markup conventions

- Partials are htmx fragments with a stable outer id (`id="mm-thread"`, `id="mm-results"`),
  swapped with `outerHTML` or appended with `beforeend`.
- Every fragment that a package appends to the thread renders `.mm-msg` or `.mm-cards` —
  never a bespoke wrapper.
- Loading states use `.mm-skel` inside the final layout, so nothing jumps when content lands.
- All user- and model-produced text is escaped. `template.HTML` is never applied to either.
- Buttons that trigger an LLM round-trip carry `hx-indicator` and are disabled while in flight.

## Routes and who owns them

| route | package | owner |
|---|---|---|
| `/`, `/about`, `/offers*`, `/perspective` | `internal/web` | design slice |
| `/start*` (onboarding thread, profile) | `internal/onboarding` | onboarding slice |
| `/find*` (natural-language search) | `internal/search` | search slice |
| `/ask` (routes a prompt to onboarding or search) | `main.go` | orchestrator |
| `/assistant/*`, `/feedback` | `internal/assistant` | unchanged |
| `/dev*` | `internal/devloop` | unchanged |
