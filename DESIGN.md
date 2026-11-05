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

For looking around a project rather than finding one thing, Docent also has
an **overview window**: a regular, resizable window with the project tree and
files in editor tabs, opened when Docent is launched. Closing it leaves Docent
in the background with the palette.

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
5. **Lightweight.** Minimal resource usage in the background. No build
   configuration needed just to find something; projects are optional and
   stay out of sight until there is more than one.
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
- **Survey a project.** Launch Docent (or "Open Docent" in the tray) → the
  overview window opens → expand the tree, open a few files in tabs, follow
  definitions; the hotkey over the window searches and opens results in it.
- **From a search to the overview.** Found something in the palette and want
  to look around it → `Cmd+Enter` → it opens as a tab in the overview window.
- **Switch context.** Done with work code, on to a side project → hotkey →
  `Cmd+P` → pick the project → search, the agent and the overview window now
  see only its code; switching back restores the tabs left open there.

## Features

Shortcuts are written with `Cmd` as on macOS; on Linux and Windows `Cmd` is `Ctrl`.
Shortcuts are bound to physical keys, so they work in any keyboard layout
(`Cmd+W` is the same key with a Russian layout on). Characters typed into the
search bar, such as the tab shortcuts `#` and `@`, are taken as typed.

### Code sources

- Docent searches local checkouts: the user adds folders (code roots),
  Docent indexes them.
- Indexing of a root can be limited to subdirectories, so that only the
  relevant part of a large monorepo is indexed — say, a few services.
- **Projects.** Roots are grouped into projects: named contexts such as
  "Work" (several services of a monorepo plus a couple of other
  repositories) and "Home". A root belongs to one project; the same folder
  can be added to several projects, each with its own subdirectories.
- **One active project.** The palette, the agent, the viewer and the
  overview window see only the roots of the active project. Docent starts in
  the project that was active when it last ran.
- Only the active project is indexed and kept in memory. Other projects keep
  their index on disk: switching to one opens its last index at once and
  brings it up to date in the background, like indexing at startup.
- Projects stay out of sight until there is a second one: with a single
  project the palette shows no project name, and the first folder added goes
  into a project created for it. `Cmd+P` still lists projects (see
  [Switching projects](#switching-projects)).
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
- Summoned while the overview window has focus, the palette opens results as
  tabs in the overview window instead of its own viewer (see
  [Overview window](#overview-window)).
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

### Switching projects

- `Cmd+P` lists the projects in the result card, the active one selected,
  followed by "New project…" and "Manage projects…". It works with a single
  project too, as the way to discover projects. Typing filters the list by
  name (fuzzy, as in the Files tab); arrow keys move, `Enter` chooses, `Esc`
  returns to the search with the query as it was.
- With two or more projects the active project's name is also a pill next
  to the gear by the palette tabs; clicking it opens the same list. With one
  project there is no pill.
- "New project…" opens the system folder picker; the project is created
  with the folder as its root, named after it, and becomes active. It can be
  renamed and given more roots in settings.
- "Manage projects…" opens Settings at Projects.
- After switching, the palette returns to the tab and query it had and runs
  the query again in the new project. An agent answer is not re-run: the
  Agent tab starts empty, so switching never spends tokens.
- Switching with unsaved edits in the overview window asks to save, discard
  or cancel, as closing the window does.
- The overview window shows the active project's name in its tab bar and
  switches along with the palette.

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
- The agent sees only the active project: its tools search and read the
  project's roots and nothing else.
- The agent is read-only: it cannot modify files or run commands through Docent.
- Findings, progress and the final answer are shown the same way regardless of
  which agent produced them.
- **Follow-up questions.** Below an answer, a field asks a follow-up in the
  same thread: the agent sees the thread's earlier questions and answers.
  The search bar always starts a new thread. Earlier turns are shown
  collapsed above the latest answer. `Tab` in the search bar moves to the
  follow-up field while an answer is shown; `Esc` there returns to the
  search bar.
- Several agents can be configured; one is the default, another can be chosen
  per query.

### Settings

Opened with `Cmd+,` from the palette or the overview window, from the menu bar
(tray) icon or the gear next to the palette tabs; shown in the palette window.

- **Projects:** add, rename and remove projects; the active one is shown
  first. Removing the active project switches to another one.
- **Code roots** of a project: add a folder with the system folder picker or
  by typing a path, limit it to subdirectories, remove it. Saving re-indexes
  the active project in the background; search keeps working meanwhile.
- **Agent:** provider (Anthropic or OpenAI-compatible), model, optional base
  URL, API key. The key is written to the OS credential store and never shown
  again; it can be replaced or removed.
- **Palette:** which screen it opens on: the one with the focused window
  (default), the one with the pointer, or the primary screen.
- Shows where the config file lives, for those who prefer editing it by hand.
- First run: with no roots, the palette stays a compact bar and offers
  "Add folder…" (system folder picker, then indexing starts) and Settings.
  The folder goes into a project named after it. A project with no roots
  shows the same offer.
- `Cmd+S` saves, `Esc` closes without saving. Both return to where settings
  were opened from: the palette, or — when opened from the overview window
  or the tray — that window or application, hiding the palette.

### Results

- Each result is a highlighted code snippet with file path and line, not just a name.
- Results are navigated with arrow keys; the selected result is previewed immediately.
- `Enter` opens a result in the viewer; `Cmd+Enter` opens it as a tab in the
  overview window and hides the palette.
- In agent mode, findings appear as the search progresses; the final answer
  links to specific places in the code.

### Viewer

- Opens on top of the current window, without switching to a separate application.
- Laid out as floating bubbles, like the palette: the search context, the
  project tree and the code. Hidden bubbles free their space.
- **Search context:** the query and its results (or the agent's answer and
  sources); clicking a result opens it in place, and `Cmd+Alt+Down` /
  `Cmd+Alt+Up` go to the next / previous result. Can be hidden.
- **Project tree:** the indexed files of the open file's code root, expanded
  to the open file. Folders expand and collapse; choosing a file opens it.
  `Cmd+1` shows the tree and moves focus to it, or hides it when it has focus;
  in the tree the arrow keys move and expand or collapse, `Enter` opens and
  `Esc` returns to the code. Can be hidden.
- **Speed search in the tree**, as in IntelliJ IDEA: typing while the tree
  has focus searches the names of visible files and folders. The query shows
  over the tree, matches are highlighted and the selection jumps to the
  first one; `Up` / `Down` move between matches, `Backspace` edits the
  query, `Enter` opens and `Esc` clears the search.
- The path above the code is a breadcrumb; choosing a folder in it reveals
  that folder in the tree.
- Syntax highlighting, go to line, find in file.
- Go to definition and find usages, as in IntelliJ IDEA:
  - `Cmd+click` or `Cmd+B` on a symbol goes to its definition; on the
    definition itself it finds its usages instead.
  - `Alt+F7` finds usages of the symbol at the cursor.
  - Right-clicking a symbol offers both, with their shortcuts.
  - Several definitions, or usages, are shown as a list to choose from.
- Back and forward navigation history.
- `Esc` or `Cmd+W` closes the file and returns to the palette, like closing
  a tab. With unsaved edits the first press reminds to save, a second one
  discards them.
- Hidden from the viewer by clicking elsewhere, Docent reopens in the viewer
  at the same file and place, keeping unsaved edits.
- The hotkey pressed in the viewer goes to the palette, like `Esc`, with the
  query selected; pressed again there, it hides Docent. With unsaved edits it
  only reminds to save or discard them.
- `Cmd+Enter` (or the button in the viewer header) moves the open file to a
  tab in the overview window, unsaved edits included, and hides the palette.

### Overview window

An IDE-style window for looking around a project: the project tree and code in
tabs. Unlike the palette, it is an ordinary application window: it does not
hide when focus moves elsewhere and can be resized, maximized or made full
screen.

- Looks like the palette: tab pills and floating bubbles — the tree, the
  code, the results panel — over a backdrop, rather than a classic IDE frame.
  The tab bar is the title bar: the window is dragged by it and maximized by
  double-clicking it. On macOS the system window buttons sit in it; on
  Windows and Linux the window has no system frame and Docent draws its own
  minimize, maximize and close buttons.

- **Opening.** Launching Docent opens the overview window; launching it again
  while it runs brings the window forward. "Open Docent" in the tray menu does
  the same. `Cmd+Enter` in the palette or viewer opens a file in it.
- **Closing** the window leaves Docent running in the background; the hotkey
  still summons the palette. Until Docent restarts, the window reopens with
  the same tabs, tree state, size and position.
- **Per project.** Tabs and tree state belong to the project: switching
  projects (see [Switching projects](#switching-projects)) replaces them with
  the other project's, and switching back restores them, until Docent
  restarts. Size and position are shared.
- While the window is open, Docent shows in the Dock and `Cmd+Tab` on macOS
  and on the taskbar on Windows; with the window closed it lives only in the
  menu bar (tray).
- **Project tree** on the left: every code root of the active project as a
  top-level folder.
  Folders expand and collapse; choosing a file opens it in a tab. The tree
  follows the active tab, expanded to its file. Keys as in the viewer's tree:
  `Cmd+1` shows the tree and focuses it, or hides it when it has focus; arrow
  keys move and expand or collapse, `Enter` opens, `Esc` returns to the code;
  typing searches visible names (speed search).
- **Tabs.** A file opens in a new tab, or switches to its tab if it is
  already open. `Ctrl+Tab` / `Ctrl+Shift+Tab` switch tabs; `Cmd+W` closes the
  active tab, and with unsaved edits the first press reminds to save, a
  second one discards them. Clicking a tab's close button with unsaved edits
  works the same way.
- **Code** as in the viewer: syntax highlighting, go to line, find in file,
  go to definition and find usages with the same keys and clicks,
  editing and saving, a breadcrumb path that reveals folders in the tree.
- **Go to definition and find usages stay in the window:** a single
  definition opens in a tab; several definitions, or usages, are listed in a
  results panel below the code. Clicking a result opens it;
  `Cmd+Alt+Down` / `Cmd+Alt+Up` go to the next / previous result. `Esc` in the
  panel or its close button hides it.
- **Search** is the palette: the hotkey pressed over the window summons it,
  and results chosen there open as tabs in the window (agent answers'
  links included). `Esc` returns to the window.
- `Cmd+,` or the gear in the tab bar opens settings in the palette.
- `Cmd+P` opens the project list in the palette. With two or more projects
  the tab bar also shows the active project's name; clicking it does the same.
- With no code roots the window offers "Add folder…" and Settings, as the
  palette does on first run.
- Closing the window with unsaved edits in any tab asks to save, discard or
  cancel.

### Editing

- A file open in the viewer or the overview window can be edited and saved.
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
- Projects: roots grouped into projects, one active; switching from the
  palette and the overview window; only the active project indexed.
- Structural search for the full language minimum (Kotlin, Java, C++, Go,
  Protobuf, JavaScript, TypeScript).
- All palette tabs (All, Types, Symbols, Files, Text, Agent) with shortcut
  characters; results as snippets with preview.
- Viewer: syntax highlighting, go to line, find in file, go to definition and
  find usages, basic editing and saving, project tree.
- Overview window: opened on launch, project tree of the project's roots, editor tabs,
  definitions and usages in its results panel, palette results opening in it.
- Built-in agent: Anthropic and OpenAI-compatible providers, one configured
  agent, API key in the OS credential store.
- Settings screen for projects, code roots and the agent.

Out of scope (after MVP):

- External agents over ACP; exposing search tools over MCP; several agents.
- Per-project agents: a project overriding the global agent (e.g. a work
  provider for work code).
- Background index updates on file changes.
- Navigation history in the viewer.
- Navigating into library sources: definitions and usages outside the code
  roots (dependencies, the standard library).
- Double `Shift` hotkey.
- Linux build (code stays cross-platform; platform-specific parts are isolated).
- Changing the hotkey from the UI.
- Look and feel beyond a clean default.

## Non-goals

- Building, running, debugging, testing.
- Refactoring, IDE-grade autocompletion, error diagnostics.
- Git UI, terminal, task management.
- Replacing the agent window or a full-blown agent chat inside Docent:
  follow-ups refine one search, they do not make a chat.

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
- **Frameless overview window outside macOS.** Keeps the backdrop and
  bubbles up to the window's top edge on every platform; macOS gets the same
  with a hidden title bar and the system buttons. The cost on Windows 11 is
  the Snap Layouts flyout on the maximize button; snapping by dragging
  still works.
- **Single instance.** A second launch does not start another Docent; it
  brings the running one's overview window forward.
- **macOS activation policy follows the overview window.** Docent runs as an
  accessory app (no Dock icon) so the palette never activates it; while the
  overview window is open it switches to a regular app, so the window gets a
  Dock icon and `Cmd+Tab`, and back when the window closes.
- **Distribution: GitHub Releases.** A version tag builds the release in
  CI: an NSIS installer and a portable zip for Windows, a universal
  (arm64 + amd64) DMG for macOS. Builds are not code-signed yet, so Windows
  SmartScreen and macOS Gatekeeper warn on first launch.
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
- **2026-11-03 — Project tree as a bubble in the viewer, not a palette tab.**
  Palette tabs are search modes, and a tree is not one; the tree is needed
  while reading code, to see where a file sits and what is next to it.
  Bubbles in one window were chosen over separate windows for the code,
  search and tree: one window keeps summoning, hiding and returning focus as
  they are, while separate windows would need focus to pass between them
  without hiding Docent. Separate windows can be revisited if a tree on
  another screen turns out to be needed.
- **2026-11-03 — The hotkey in the viewer opens search.** Docent reopens in
  the viewer it was hidden from, so the way back to search has to be obvious;
  the summoning hotkey is already under the fingers. Considered: always
  reopening the palette with a "back to file" row, which loses the code on
  every click elsewhere.
- **2026-11-03 — `Cmd+W` in the viewer closes the file.** It reads as closing
  a tab, not hiding the app, so it returns to search like `Esc`; in the
  palette it still hides Docent.
- **2026-11-03 — Follow-ups in their own field.** Answers often need one more
  question. Where the question is typed decides what it is: the search bar
  starts over, the field below the answer continues, so there is no hidden
  mode or extra key to remember. Earlier turns go to the agent as plain
  questions and answers; their citations carry the places already found.
- **2026-11-03 — Unsigned releases for now.** Signing costs money and setup
  (Apple Developer ID and notarization; a Windows code signing service) that
  is not worth it before Docent has users beyond its authors. Signing is
  added to the release workflow when it is.
- **2026-11-04 — Overview window, separate from the palette.** Revisits the
  2026-11-03 choice of bubbles in one window: the palette covers "find and
  look", but surveying a project — walking the tree, keeping several files
  open — needs a window that stays put and can take the whole screen. The
  palette and its viewer stay as they are; the overview is a second,
  ordinary window rather than a mode of the palette, so the palette keeps
  hiding on focus loss and returning focus. Editor tabs, since surveying
  means keeping several files at hand; all roots in one tree, like modules of
  one project; definitions and usages stay inside the window instead of
  handing off to the palette.
- **2026-11-04 — Launching opens the overview, the palette lives in the
  background.** A launch by hand is a request to see Docent, so it opens the
  window; closing it keeps the hotkey working. The hotkey over the overview
  window searches into it, like Search Everywhere in an IDE, so the window
  needs no search of its own; `Cmd+Enter` is the way from the palette
  anywhere else. Considered: opening only from the tray (hard to discover)
  and a second global hotkey (one more binding to take from other apps).
- **2026-11-04 — Go to definition on a definition finds usages.** As in
  IntelliJ IDEA, one gesture (`Cmd+click`, `Cmd+B`) covers both directions:
  from a usage it is the way to the declaration, and on the declaration
  there is nowhere else to go, so the useful answer is where it is used.
  A context menu makes both discoverable without knowing the shortcuts.
- **2026-11-05 — Projects: named groups of roots, one active.** One person
  reads code in unrelated contexts — work services in a monorepo, Docent
  itself, side projects — and mixing them makes results noisy. The project
  scopes everything, not just search: the overview tree and tabs, and the
  agent, so that a question about a side project never sends work code to a
  provider. Only the active project is indexed, to stay light in the
  background; switching opens the cached index at once. Projects stay
  hidden until there are two, so a single folder still needs no setup;
  roots from an existing config become a project named "Default". The
  switcher is a pill and `Cmd+P` in the palette rather than a palette tab:
  tabs are search modes, and a project is not one. Considered: a project
  that filters only search (the tree and the agent would still mix
  contexts) and keeping every project indexed (instant switching at the
  cost of memory and CPU for code not being read). Per-project agents are
  left for after the MVP.
- **2026-11-05 — `Cmd+P` works with one project.** Hiding every trace of
  projects until there are two left them reachable only through settings.
  `Cmd+P` always opens the project list, with "New project…" and "Manage
  projects…" at its end, so projects can be found from the keyboard; the
  pill still appears only with two or more, so a single project shows
  nothing extra. A new project starts from a folder, like the first run.
