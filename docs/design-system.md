# Montage Manager — design system contract

One surface, one language. `internal/web/static/base.css` owns the tokens and primitives;
every package styles itself with them. A package may add a CSS file for what is genuinely
local (`onboarding.css`, `search.css`, `dev.css`, and `internal/web`'s own `app.css`), but
must not redefine a token or restyle a primitive — if a primitive is wrong, fix it in
`base.css` so every surface moves together. `app.css` today holds only the `/about` hero's
two-column composition and its map image; everything else that page needs is a `base.css`
primitive.

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
| `.mm-shell` `.mm-topzone` `.mm-top` `.mm-main` `.mm-foot` | page frame. `.mm-topzone` wraps the header and (on the home surface) the prompt into one sticky unit — the prompt stays reachable while the thread scrolls, with no height math against the header |
| `.mm-brand` `.mm-brand-mark` | wordmark |
| `.mm-ask` `.mm-prompt` `.mm-suggest` | the prompt and its example chips. `.mm-ask` also doubles as a quiet headline+lede block outside the prompt context (used by `/about` and `/offers`) |
| `.mm-lede` | muted lede paragraph, usable standalone anywhere, not just inside `.mm-ask` |
| `.mm-thread` `.mm-msg.you` `.mm-msg.mm` `.mm-who` | conversation |
| `.mm-cards` `.mm-card` `.mm-card-head` `.mm-fit` `.mm-why` | result cards. `.mm-cards` is a bare grid — its children need not be `.mm-card` (used for pairs of `.mm-panel` too) |
| `.mm-btn` (`.ghost` `.quiet`) `.mm-chip` (`.on`, `[disabled]`) `.mm-badge` (`.good` `.warn` `.bad`) | controls |
| `.mm-steps` | a row of `.mm-chip` read as one control (segmented status/filter/role switch), not loose buttons |
| `.mm-toolbar` | a filter row: chips on one side, a field on the other |
| `.mm-panel` `.mm-field` `.mm-meter` `.mm-empty` `.mm-skel` | containers, inputs, states |
| `.mm-form` | a form that collapses into a `<details class="mm-panel">` until asked for; `.mm-form label` stacks a label over its field |
| `.mm-stats` `.mm-stat` | a row of label/value stats (counts, quick metrics) |
| `.mm-eyebrow` `.mm-section` | a quiet section header (eyebrow + `<h2>`) and the section wrapper it lives in |
| `.mm-muted` `.mm-mono` `.mm-sr` | text helpers — `.mm-sr` is visually hidden but still announced, for state a skeleton only conveys visually |

## Markup conventions

- Partials are htmx fragments with a stable outer id (`id="mm-thread"`, `id="mm-results"`),
  swapped with `outerHTML` or appended with `beforeend`.
- Every fragment that a package appends to the thread renders `.mm-msg` or `.mm-cards` —
  never a bespoke wrapper.
- Loading states use `.mm-skel` inside the final layout, so nothing jumps when content lands.
  The home surface clones a `<template>` skeleton bubble into `#mm-thread` on
  `hx-on::before-request` and removes it on `hx-on::after-swap` (not `after-request` — that
  fires after the browser has already painted the swap, which is one frame too late and
  reads as a jump).
- All user- and model-produced text is escaped. `template.HTML` is never applied to either.
- Buttons that trigger an LLM round-trip carry `hx-indicator` and are disabled while in flight.
- The thread and the pipeline grid both carry `aria-live="polite"` (the thread also `role="log"`)
  so screen readers hear what streams in without a page reload.
- The home surface keyboard contract: `/` focuses the prompt from anywhere outside a text
  field, `Enter` submits it (native `<input>` behaviour, no JS needed), `Esc` clears it while
  it has focus.

## Routes and who owns them

| route | package | owner |
|---|---|---|
| `/`, `/about`, `/offers*`, `/perspective` | `internal/web` | design slice |

`GET /offers` renders differently by request: an htmx request (`HX-Request: true`) gets the
`#offers` fragment it swaps in; a direct visit gets that same fragment inside the standalone
pipeline page, so the "Pipeline" nav link and a bare `/offers` visit both work.
| `/start*` (onboarding thread, profile) | `internal/onboarding` | onboarding slice |
| `/find*` (natural-language search) | `internal/search` | search slice |
| `/ask` (routes a prompt to onboarding or search) | `main.go` | orchestrator |
| `/assistant/*`, `/feedback` | `internal/assistant` | unchanged |
| `/dev*` | `internal/devloop` | unchanged |
