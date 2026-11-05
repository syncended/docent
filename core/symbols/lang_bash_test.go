package symbols

import "testing"

func TestExtractBash(t *testing.T) {
	src := `#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
export BUILD_DIR="$ROOT_DIR/build"
readonly VERSION=1.2.3
declare -a TARGETS=(linux darwin)
HOST=localhost PORT=8080

if [ -z "${CI:-}" ]; then
  verbose=1
fi

for target in "${TARGETS[@]}"; do
  count=$((count + 1))
done

LANG=C sort input.txt

build() {
  local out="$BUILD_DIR/$1"
  status=0
  helper() { :; }
  echo "$out"
}

function clean {
  rm -rf "$BUILD_DIR"
}

function deploy() {
  :
}
`
	checkExtract(t, "bash", src, []string{
		"ROOT_DIR variable",
		"BUILD_DIR variable",
		"VERSION variable",
		"TARGETS variable",
		"HOST variable",
		"PORT variable",
		"build function",
		"clean function",
		"deploy function",
	}, []string{"verbose", "target", "count", "LANG", "out", "status", "helper"})
}
