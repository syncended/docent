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

## Code style

Applies to Go and frontend code alike.

- Functional style: a domain package is the entity, its package-level
  functions are the operations — `code.SearchTypes(q, n)`, `code.ReadFile(root, path)`.
- No service objects: no `Service` interfaces, no structs with methods holding
  dependencies, no constructors wiring them together.
- Dependencies are static package state, initialized once at startup
  (e.g. `code.Open`). `context.Context` carries cancellation and deadlines.
- Data is plain structs. Methods only where the language requires them
  (`sort`, `container/heap`) or on small data helpers.
- No boolean flags that switch behavior: separate functions instead
  (`SearchText` / `SearchRegex`), sharing private helpers.

## Layout

- `core/` — shell-independent core:
  - `config` — user config (TOML)
  - `source` — listing files of a root
  - `symbols` — tree-sitter declaration extraction, per-language queries
  - `fuzzy` — IDE-style fuzzy name matching
  - `code` — index (zoekt + symbols) and search; the core's public API
  - `provider` — model provider presets, the bundled model catalog
    (`go generate` regenerates it from models.dev) and live model lists
  - `agent` — the built-in agent's tool-calling loop over provider APIs
- `cmd/docentctl` — development CLI over `core/code`.
- Tests: `go test ./...`.

## Repository

- The main branch is `trunk`.
- This is a public open-source project. Repository content never mentions
  employers, internal repositories, private hosts or internal tooling —
  including in comments, examples, test fixtures and generated files.
- Lockfiles resolve to public registries only (`frontend/.npmrc` pins npmjs);
  check `package-lock.json` hosts before committing.
