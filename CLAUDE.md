# Docent

A read-mostly IDE for searching and viewing code with an agent: a background app,
a global hotkey, a search palette on top of the active window.

## Process: spec-driven

The specification is `DESIGN.md` — the single source of truth for what the product does.

- Read `DESIGN.md` before any work.
- A behavior change starts with an edit to `DESIGN.md`; code follows in the same PR or later.
- If implementation hits a contradiction or a gap in the spec — stop and raise
  the question instead of silently filling it in.
- Once an open question is decided, move it to the "Decision log" with a date
  and rationale, and remove it from "Open questions".
- `DESIGN.md` describes the product. Add to "Technical decisions" only what is
  critical to the product (affects behavior, speed, platforms, data sources).
- All repository content — code, docs, commits, PRs — is in English.

## Repository

- The main branch is `trunk`.
