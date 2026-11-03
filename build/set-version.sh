#!/usr/bin/env bash
# Sets the application version in every build asset that carries it.
# Usage: build/set-version.sh 1.2.3
set -euo pipefail

version="${1:?usage: $0 <version>}"
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "not a version: $version" >&2
  exit 1
fi
# Windows file versions are numeric only.
numeric="${version%%-*}"
cd "$(dirname "$0")"

perl -pi -e "s/^(\s+version: )\"[^\"]*\"/\${1}\"$version\"/" config.yml
for plist in darwin/Info.plist darwin/Info.dev.plist; do
  perl -0pi -e "s|(<key>CFBundle(?:Short)?VersionString</key>\s*<string>)[^<]*|\${1}$version|; s|(<key>CFBundleVersion</key>\s*<string>)[^<]*|\${1}$numeric|" "$plist"
done
perl -pi -e "s/(\"(?:file_version|product_version|FileVersion|ProductVersion)\": )\"[^\"]*\"/\${1}\"$numeric\"/" windows/info.json
perl -pi -e "s/(!define INFO_PRODUCTVERSION )\"[^\"]*\"/\${1}\"$numeric\"/" windows/nsis/wails_tools.nsh
