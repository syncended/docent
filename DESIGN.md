# Docent — Design

The single product specification. Describes **what** Docent does and **why**.
Technical details are recorded here only where they are critical to the product.

## Problem

More and more development happens through AI agents: a separate window where
tasks are dispatched. A full IDE can go unopened for months. Yet the need to
**read and search code** never goes away: to understand how something works,
to find where an enum lives, to check the fields of a proto message.

Today there are two awkward paths:

1. **I know what I'm looking for** (a class, enum, proto, function) — search in
   the repository's web UI. Slow, imprecise, clumsy results, and the code itself
   is reached through a browser.
2. **I don't know what I'm looking for** (searching by functionality) — ask
   the agent. It works, but it's a context switch, the answer comes back as
   prose rather than code, and getting to the code is a separate step.

Launching a heavyweight IDE for either is out of proportion to the task.

## Solution

Docent is a **background app** with a global hotkey. When summoned, a search
palette in the spirit of Spotlight / Search Everywhere appears on top of
whatever window is active. Results are shown as code snippets; any of them
opens in a lightweight viewer where the file can be edited if needed.

Docent is a notepad with great search, not a development environment.

## Principles

1. **Read-mostly.** Reading and searching are the core. Editing is minimal and
   incidental.
2. **Keyboard-first.** Summoning, choosing a mode, navigating results, opening
   and closing — all from the keyboard.
3. **Instant.** The palette appears with no perceptible delay. Structural search
   responds on every keystroke. Agent search may take longer, but shows progress
   and intermediate findings.
4. **Unobtrusive.** Lives in the background, never steals focus unless summoned,
   and returns focus to where it was when dismissed.
5. **Lightweight.** Minimal resource usage in the background. No projects,
   workspaces or build configuration needed just to find something.
6. **Complements agents, doesn't replace them.** The agent window stays the
   primary tool; Docent covers the "find and look" use case.

## Scenarios

- **Find a known symbol.** Hotkey → type an enum name → see matches as code
  snippets → Enter → viewer opens the file at the right line.
- **Find by meaning.** Hotkey → "where is the cart discount calculated" → the
  agent searches, showing locations as code snippets as it goes, and finishes
  with a short answer linking to specific places, each opening in the viewer.
- **Quick edit.** Open a file from the results or by path, edit, save, close.
- **Walk the code.** From the viewer — go to definition, find usages, move
  back and forward through navigation history.

## Features

Shortcuts are written with `Cmd` as on macOS; on Linux and Windows `Cmd` is `Ctrl`.

### Code sources

- Docent searches local checkouts: the user adds folders, Docent indexes them.
- Indexing can be limited to subdirectories, so that only the relevant part
  of a large monorepo is indexed.
- The index is updated in the background as files change; search remains
  available while indexing is in progress.

### Languages

- Structural search supports as many languages as possible. Required minimum:
  Kotlin, Java, C++, Go, Protobuf, JavaScript, TypeScript.
- Beyond that — other widespread languages (Python, Rust, C#, Swift, etc.)
  without reworking search for each one.
- For unsupported languages, file search and full-text search still work.

### Summoning

- A global hotkey summons the palette from any application. Default:
  `Cmd+Shift+Space` on macOS, `Ctrl+Shift+Space` on Linux and Windows;
  the hotkey is configurable.
- Double-tap `Shift` (as in IntelliJ IDEA) is available as an opt-in
  alternative on platforms that allow it.
- Pressing the hotkey again, `Esc`, `Cmd+W` or clicking elsewhere hides the
  palette and returns focus to the previous window.
- The palette reopens with the last query and tab; the query is selected so
  a new one can be typed right away.
- The palette can be moved by dragging the search bar. Until Docent restarts
  it reopens at the moved position, relative to the screen it opens on;
  results and the viewer grow from there and stay within the screen.

### Palette and search modes

Search modes are tabs, as in IntelliJ IDEA's Search Everywhere:

| Tab     | Searches                                                            | Shortcut |
|---------|---------------------------------------------------------------------|----------|
| All     | Types, symbols, files and text together                             |          |
| Types   | Classes, enums, interfaces, proto messages and services             | `#`      |
| Symbols | Any named declaration: functions, methods, fields, constants, enum values | `@` |
| Files   | File names and paths                                                | `/`      |
| Text    | Full-text, ignoring case; optional regular expression and Match case modes |   |
| Agent   | Natural language query, search by functionality (see [Agents](#agents)) | `?`  |

- `Tab` / `Shift+Tab` cycle through tabs.
- Typing a shortcut character into an empty query switches to its tab; the
  character is not added to the query. If that tab is already active, the
  character is typed as usual.
- The query is kept when switching tabs, so the same query can be tried in
  another mode.
- **The agent never starts implicitly.** In the All tab, the last result row
  is "Ask agent: <query>"; the agent starts only when that row is chosen or
  when the query is submitted in the Agent tab. Typing alone never spends
  tokens.

### Agents

Two kinds of agents, interchangeable from the user's point of view:

- **Built-in agent.** Works out of the box with just an API key: the user picks
  a provider, pastes a key, chooses a model. Supported providers: Anthropic and
  any OpenAI-compatible API (OpenAI, DeepSeek, OpenRouter, local servers such as
  Ollama, etc.).
- **External agent.** An existing agent harness (Claude Code, Codex, opencode,
  etc.) connected by its launch command. It brings its own models, configuration,
  credentials and tools.

For both kinds:

- The agent searches with Docent's own tools (structural, file and text search,
  reading files), so it gets the same fast index as the palette.
- The agent is read-only: it cannot modify files or run commands through Docent.
- Findings, progress and the final answer are shown the same way regardless of
  which agent produced them.
- Several agents can be configured; one is the default, another can be chosen
  per query.

### Settings

Opened with `Cmd+,` from the palette, from the menu bar (tray) icon or the gear next
to the palette tabs; shown in the palette window.

- **Code roots:** add a folder with the system folder picker or by typing a
  path, limit it to subdirectories, remove it. Saving re-indexes in the
  background; search keeps working meanwhile.
- **Agent:** provider (Anthropic or OpenAI-compatible), model, optional base
  URL, API key. The key is written to the OS credential store and never shown
  again; it can be replaced or removed.
- **Palette:** which screen it opens on: the one with the focused window
  (default), the one with the pointer, or the primary screen.
- Shows where the config file lives, for those who prefer editing it by hand.
- First run: with no roots, the palette stays a compact bar and offers
  "Add folder…" (system folder picker, then indexing starts) and Settings.
- `Cmd+S` saves, `Esc` returns to the palette without saving.

### Results

- Each result is a highlighted code snippet with file path and line, not just a name.
- Results are navigated with arrow keys; the selected result is previewed immediately.
- In agent mode, findings appear as the search progresses; the final answer
  links to specific places in the code.

### Viewer

- Opens on top of the current window, without switching to a separate application.
- Keeps the search context: a side panel shows the query and its results (or
  the agent's answer and sources); clicking a result opens it in place, and
  `Cmd+Alt+Down` / `Cmd+Alt+Up` go to the next / previous result. The panel
  can be hidden.
- Syntax highlighting, go to line, find in file.
- Go to definition and find usages; back and forward navigation history.
- `Esc` returns to the palette; `Cmd+W` hides the window, keeping unsaved
  edits for the next time it is shown.

### Editing

- A file open in the viewer can be edited and saved.
- Basic text editing only: no autocompletion, refactoring or diagnostics.

## Milestones

### MVP

Goal: replace the repository web UI for "I know what I'm looking for" and the
agent window for "find by meaning", on macOS and Windows.

In scope:

- Background app with the default global hotkey; palette over the active
  window; `Esc` / hotkey hides it and returns focus.
- Code sources: folders from the settings screen (stored in a config file),
  optionally limited to subdirectories. Indexing at startup, on demand and
  after settings change.
- Structural search for the full language minimum (Kotlin, Java, C++, Go,
  Protobuf, JavaScript, TypeScript).
- All palette tabs (All, Types, Symbols, Files, Text, Agent) with shortcut
  characters; results as snippets with preview.
- Viewer: syntax highlighting, go to line, find in file, go to definition and
  find usages, basic editing and saving.
- Built-in agent: Anthropic and OpenAI-compatible providers, one configured
  agent, API key in the OS credential store.
- Settings screen for code roots and the agent.

Out of scope (after MVP):

- External agents over ACP; exposing search tools over MCP; several agents.
- Background index updates on file changes.
- Navigation history in the viewer.
- Double `Shift` hotkey.
- Linux build (code stays cross-platform; platform-specific parts are isolated).
- Changing the hotkey from the UI.
- Look and feel beyond a clean default.

## Non-goals

- Building, running, debugging, testing.
- Refactoring, IDE-grade autocompletion, error diagnostics.
- Git UI, terminal, task management.
- Replacing the agent window or a full-blown agent chat inside Docent.

## Technical decisions

Only what is critical to the product.

- **Platforms:** macOS, Linux, Windows.
- **The index is built locally** by Docent from local checkouts; remote code
  search services are not used.
- **Stack:** Go. Desktop shell — Wails v3 (system webview, global shortcuts,
  always-on-top windows).
- **Search engine:** zoekt (trigram index for text and regex search);
  symbols are extracted with tree-sitter.
- **Core is independent of the shell.** Indexing, search and the agent bridge
  live in a core package that the shell only calls into. The shell can be
  replaced (e.g. by Tauri with the Go core as a sidecar process) without
  touching the core, and the same core can later be exposed to agents as an
  MCP server.
- **Transparent palette window on macOS.** The palette is drawn as separate
  bubbles over a transparent window, which requires making WKWebView
  transparent through a private API (`private_mac_apis` build tag). This rules
  out the Mac App Store; Docent is distributed outside it.
- **zoekt fork with Windows support.** Upstream zoekt builds only on Unix.
  Docent uses a fork whose Windows build reads index shards into memory
  instead of memory-mapping them: Windows cannot replace a mapped file, and
  re-indexing replaces shards while they are being searched. On Windows the
  index therefore occupies RAM; the fork is dropped once upstream supports
  Windows.
- **Agents:**
  - The built-in agent is Docent's own tool-calling loop over provider APIs
    (Anthropic Messages API, OpenAI-compatible Chat Completions), calling core
    search tools directly.
  - External agents are connected over ACP (Agent Client Protocol); Docent acts
    as the ACP client and exposes its search tools to the agent over MCP.
    Write and command execution requests from external agents are denied.
  - API keys are stored in the OS credential store (Keychain, Credential
    Manager, Secret Service), never in plain-text config.

## Open questions

- **Look and feel.** Translucent materials in the spirit of macOS Liquid Glass
  (Mica/Acrylic on Windows) for the palette bubbles, with code always on an
  opaque surface? To be decided during the MVP.

## Decision log

Decisions on open questions, with date and a short rationale.

- **2026-11-03 — Code source: local checkouts.** Search must be fast, work
  offline and not depend on a repository's web UI.
- **2026-11-03 — Platforms: cross-platform.** The product is not tied to macOS.
- **2026-11-03 — Languages: the broader, the better.** Minimum — Kotlin, Java,
  C++, Go, Protobuf, JS, TS; the search approach must not require a separate
  implementation per language.
- **2026-11-03 — Stack: Go + Wails v3.** Go is simpler to write well than Rust:
  idiomatic Go performs well by default, while Rust only pays off when written
  idiomatically at a low level. zoekt gives a mature, purpose-built code search
  engine; tree-sitter covers symbols across languages. Considered: Rust + Tauri 2
  (more mature shell, native tree-sitter, but no comparable embeddable code search
  engine and a higher complexity cost), Electron (too heavy in the background),
  native Rust GUI (viewer would have to be built from scratch), Compose
  Multiplatform (JVM in the background, weak window control). Risk: Wails v3
  became stable on 2026-10-26; mitigated by keeping the core independent of the shell.
- **2026-11-03 — Agents: hybrid.** A built-in agent so that pasting an API key is
  enough to get started, plus external harnesses over ACP for users who already
  have an agent set up. ACP lets one integration cover many harnesses instead
  of one adapter per tool.
- **2026-11-03 — Hotkey: `Cmd/Ctrl+Shift+Space`, double Shift opt-in.**
  Standard global hotkey APIs only support modifier + key combinations; double
  Shift requires a global keyboard hook (an extra permission on macOS,
  unavailable on Wayland). `Cmd+Shift+F` was rejected: it is "find in files"
  in most editors and a global binding would take it away from them.
- **2026-11-03 — Mode selection: tabs with shortcut characters.** IDEA-style
  tabs keep the current mode visible; `#`, `@`, `/`, `?` are quick ways to
  switch tabs rather than a query syntax. The agent never starts implicitly,
  so that typing does not spend tokens.
- **2026-11-03 — Palette as floating bubbles; private macOS API accepted.**
  A Spotlight-like search bubble with tab bubbles below it, and a result card
  that appears only when there are results. A transparent webview has no public
  API on macOS; the private one is acceptable since Docent does not target the
  Mac App Store. If a macOS update breaks it, the fallback is an opaque window.
- **2026-11-03 — Settings UI in the MVP.** Pasting an API key needs a UI: the
  key goes to the OS credential store, not to the config file, so there is no
  file to edit for it. Roots are managed on the same screen.
- **2026-11-03 — Windows in the MVP.** Docent is needed on Windows too, and
  the core already builds and passes its tests there with the zoekt fork.
  Linux stays after the MVP.
- **2026-11-03 — Moved palette position lasts until restart.** Dragging the
  palette is for adjusting it to the current setup; a restart returns it to
  the default place, so there is nothing to reset in settings.
