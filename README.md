# docent

**Read the code your agents write.**

Writing code is moving to agents. What stays with you is reading it: finding
where something lives, seeing how the pieces fit, checking what changed and
why. A full IDE is too heavy to keep open just for that, and a repository's
web UI is too slow.

Docent is the reading half of an IDE, and nothing else. Point it at any
folder — a whole repository, or just the few services you care about in a
monorepo — and it builds a local index: a trigram index for text and regular
expressions, and tree-sitter declarations for types, functions, fields and
proto messages. No project files, no build, no language servers; it works
the same on code it has never seen before.

Two ways in:

- **Search from anywhere.** A global hotkey brings up a palette over whatever
  you are doing. Find a type, a symbol, a file or a line of text — or ask in
  plain words, and an agent searches the same index for you. Results are
  code snippets, not just file names.
- **Survey a project.** The overview window has the project tree, files in
  tabs, go to definition and find usages: everything for walking through
  code, without the rest of an IDE.

Folders are grouped into projects — work services in one, side projects in
another — and `Cmd+P` switches between them. Only the active project is
indexed and kept in memory, so Docent stays light in the background.

Structural search covers Go, Java, Kotlin, C++, Protobuf, JavaScript,
TypeScript, Python, Rust, C#, Swift, Ruby, PHP, Scala, Lua, Elixir, Groovy,
OCaml, Elm, Bash, SQL, CSS, Terraform, and Svelte and Vue components; file
and full-text search work for everything else.

![Search palette](docs/screenshots/palette.png)

Opening a result shows the file next to the search and the project tree:

![Viewer with search results and project tree](docs/screenshots/viewer.png)

The specification is [`DESIGN.md`](DESIGN.md).

## Installing

Download the latest build from
[Releases](https://github.com/syncended/docent/releases): the installer or
the zip for Windows, the DMG for macOS (Apple silicon and Intel).

Builds are not code-signed yet, so the first launch needs one extra step:

- **Windows:** SmartScreen shows "Windows protected your PC" — choose
  "More info", then "Run anyway".
- **macOS:** Gatekeeper refuses to open the app — right-click it and choose
  "Open", or run `xattr -dr com.apple.quarantine /Applications/docent.app`.

## Building

Requirements: Go 1.26+, Node.js, and the [Wails v3](https://v3.wails.io) CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Build and run on macOS:

```sh
wails3 build
./bin/docent
```

On Windows, a C compiler is needed for tree-sitter
([llvm-mingw](https://github.com/mstorsjo/llvm-mingw) or MSYS2 MinGW-w64 on `PATH`):

```sh
wails3 build
bin\docent.exe
```

To cross-compile for Windows from macOS or Linux, build the Docker image once
with `wails3 task setup:docker`, then run `wails3 build GOOS=windows`.

Then press `Cmd+Shift+Space` (`Ctrl+Shift+Space` on Windows), add a folder,
and search. Settings (`Cmd+,`, `Ctrl+,` on Windows) configure projects, their
folders and the agent provider; API keys are kept in the system credential
store.

Tests: `go test ./...`

## Releasing

Push a version tag; CI builds Windows and macOS packages and publishes a
GitHub Release (a tag with a suffix, like `v0.2.0-rc.1`, is a pre-release):

```sh
git tag v0.1.0
git push origin v0.1.0
```

`build/set-version.sh` writes the tag's version into the build assets.
