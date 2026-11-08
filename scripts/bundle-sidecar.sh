#!/usr/bin/env bash
# Copies the login sidecar (sidecar/ with its node_modules) next to where the
# built app looks for it, so 汽水音乐 / 酷狗音乐 login works in a local build.
#
# `mygo build` already bundles sidecar/ automatically (it is listed under
# "resources" in mygo.json), so this script is the fallback for ad-hoc / debug
# builds, or for re-injecting the bridge into an already-built app.
#
# Usage:
#   scripts/bundle-sidecar.sh                  # auto-find build/*.app
#   scripts/bundle-sidecar.sh path/to/app.app  # or name one (any platform)
#
# The sidecar is plain JavaScript; an unsigned copy is fine for local and
# ad-hoc builds. A notarized release should re-sign the app afterwards
# (or rely on the `resources` path in mygo.json, which mygo signs for you).
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
sidecar="$root/sidecar"
if [[ ! -f "$sidecar/index.cjs" ]]; then
	echo "sidecar/index.cjs not found at $sidecar" >&2
	exit 1
fi

# Make sure node_modules exists (it is gitignored, so a fresh checkout lacks it).
if [[ ! -d "$sidecar/node_modules" ]]; then
	echo "installing sidecar dependencies…"
	( cd "$sidecar" && npm install --no-audit --no-fund )
fi

# The app to fill: an argument, else the first one under build/.
app="${1:-}"
if [[ -z "$app" ]]; then
	app="$(find "$root/build" -maxdepth 3 \( -name '*.app' -o -name 'Echo云音乐' -o -name 'echo-cloudmusic' \) -type d 2>/dev/null | head -n1)"
fi
if [[ -z "$app" || ! -d "$app" ]]; then
	echo "no built app found; build first with \`mygo build\` or pass its path" >&2
	exit 1
fi

# Where internal/sidecar.LocateDir looks, per platform:
#   macOS  .app -> Contents/MacOS/sidecar  (or Contents/Resources/sidecar)
#   Linux  exe dir/sidecar
#   Windows exe dir/sidecar
dest=""
if [[ -d "$app/Contents/MacOS" ]]; then
	dest="$app/Contents/MacOS/sidecar"
elif [[ -f "$app" && ( "$app" == *.exe || -x "$app" ) ]]; then
	dest="$(dirname "$app")/sidecar"
elif [[ -d "$app" ]]; then
	dest="$app/sidecar"
fi
if [[ -z "$dest" ]]; then
	echo "could not determine bundle layout for $app" >&2
	exit 1
fi

echo "copying sidecar into $dest"
rm -rf "$dest"
mkdir -p "$dest"
if command -v rsync >/dev/null 2>&1; then
	rsync -a --exclude='.git' "$sidecar/" "$dest/"
else
	cp -R "$sidecar/." "$dest/"
fi
echo "done: $(du -sh "$dest" 2>/dev/null | cut -f1) of sidecar bundled"
