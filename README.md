# docent

A read-mostly IDE for searching and viewing code with an agent.

Docent lives in the background: a global hotkey brings up a search palette on
top of the active window. The palette finds symbols, types, proto definitions,
files and text — or searches by meaning through an agent — shows results as
code snippets, and opens a lightweight viewer with quick editing.

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
and search. Settings (`Cmd+,`, `Ctrl+,` on Windows) configure code roots and
the agent provider; API keys are kept in the system credential store.

Tests: `go test ./...`

## Releasing

Push a version tag; CI builds Windows and macOS packages and publishes a
GitHub Release (a tag with a suffix, like `v0.2.0-rc.1`, is a pre-release):

```sh
git tag v0.1.0
git push origin v0.1.0
```

`build/set-version.sh` writes the tag's version into the build assets.
