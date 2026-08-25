# Backlog

- Generated: 2026-08-25T18:03:31Z
- Model: deepseek-v4-flash-0731

## 1. Clarify 'Attention' status and which offers need action

- Score: 6.00
- Count: 2
- Severity: 3.0
- Effort: S
- Kind: confusion
- Status: proposed
- Feedback ids: 3, 4

Users are unsure what the 'Attention' status means and cannot easily identify which offers require their attention, leading to missed actions and frustration.

Evidence:
> I can't tell which offers need my attention
> not obvious what 'Attention' status means

## 2. Offer list does not refresh after creation

- Score: 5.33
- Count: 2
- Severity: 4.0
- Effort: M
- Kind: bug
- Status: proposed
- Feedback ids: 1, 2

Users create offers but don't see them appear until a manual reload, which breaks the core workflow and causes confusion about whether the action succeeded.

Evidence:
> the offer list never refreshes after I create one
> creating an offer doesn't show up until I reload

## 3. Export pipeline to CSV

- Score: 1.33
- Count: 1
- Severity: 2.0
- Effort: M
- Kind: request
- Status: proposed
- Feedback ids: 5

Users want to export pipeline data for offline analysis or reporting, a common and valuable feature for data-driven workflows.

Evidence:
> let me export the pipeline to CSV

## 4. Mobile dashboard looks great

- Score: 1.00
- Count: 1
- Severity: 1.0
- Effort: S
- Kind: praise
- Status: proposed
- Feedback ids: 6

Positive feedback on mobile responsiveness indicates this is a strength to preserve and build upon.

Evidence:
> the dashboard looks great on mobile

---

How this was made: LLM clustering via deepseek-v4-flash-0731. Score = feedback count x average severity / effort discount (S=1, M=1.5, L=2.5), ranked highest first, ties broken alphabetically by theme.
