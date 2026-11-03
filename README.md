# docent

A read-mostly IDE for searching and viewing code with an agent.

Docent lives in the background: a global hotkey brings up a search palette on
top of the active window. The palette finds symbols, types, proto definitions,
files and text — or searches by meaning through an agent — shows results as
code snippets, and opens a lightweight viewer with quick editing.

The specification is [`DESIGN.md`](DESIGN.md).

## Building

Requirements: Go 1.26+, Node.js, and the [Wails v3](https://v3.wails.io) CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Build and run (macOS for now):

```sh
wails3 build
./bin/docent
```

Then press `Cmd+Shift+Space`, add a folder, and search. Settings (`Cmd+,`)
configure code roots and the agent provider; API keys are kept in the system
credential store.

Tests: `go test ./...`
