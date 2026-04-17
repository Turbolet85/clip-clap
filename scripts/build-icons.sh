#!/usr/bin/env bash
# Build both assets/app.ico (deep-ink #0E1013) and assets/app-amber.ico
# (safelight-amber #C64A1E) from the single source assets/app.svg in ONE
# atomic action. Both ICOs MUST regenerate together to maintain
# byte-identical silhouettes — required by the design system §Iconography
# single-source binding rule, otherwise the Phase 3 signature flash reads
# as a shape change instead of a color shift.
#
# Output: multi-resolution ICO containers embedding 16/24/32/48/256 px
# variants, per design system §Iconography §Size grid.
#
# Toolchain: ImageMagick 7+ (magick), GNU sed (msys/git-bash on Windows
# already provides this).

set -euo pipefail

SRC="assets/app.svg"
test -f "$SRC" || { echo "ERROR: $SRC not found (run from project root)" >&2; exit 1; }
command -v magick >/dev/null 2>&1 || { echo "ERROR: magick (ImageMagick 7+) not on PATH" >&2; exit 1; }

# Use temp files inside the project tree (under .gitignore via *.tmp.svg)
# so all I/O stays on Dev Drive D: (no /tmp = C: leak).
TMP_DEEP="assets/.app-deepink.tmp.svg"
TMP_AMBER="assets/.app-amber.tmp.svg"
trap 'rm -f "$TMP_DEEP" "$TMP_AMBER"' EXIT

# Atomic substitution: read SVG once via sed, emit color-substituted SVGs.
sed 's/fill="#000000"/fill="#0E1013"/g' "$SRC" > "$TMP_DEEP"
sed 's/fill="#000000"/fill="#C64A1E"/g' "$SRC" > "$TMP_AMBER"

# Emit both ICOs back-to-back so they are guaranteed to derive from the same
# app.svg snapshot (no race window where app.svg could be edited between
# the two magick invocations).
magick -background none -density 300 "$TMP_DEEP" \
       -define icon:auto-resize=16,24,32,48,256 \
       "assets/app.ico"

magick -background none -density 300 "$TMP_AMBER" \
       -define icon:auto-resize=16,24,32,48,256 \
       "assets/app-amber.ico"

# Sanity: both files exist and are non-trivial (multi-resolution ICO ≥ 1 KB).
test -f "assets/app.ico"       && [ "$(wc -c < assets/app.ico)" -gt 1024 ]       || { echo "ERROR: assets/app.ico failed (missing or < 1 KB)" >&2; exit 1; }
test -f "assets/app-amber.ico" && [ "$(wc -c < assets/app-amber.ico)" -gt 1024 ] || { echo "ERROR: assets/app-amber.ico failed (missing or < 1 KB)" >&2; exit 1; }

echo "Built: assets/app.ico (#0E1013, $(wc -c < assets/app.ico) bytes)"
echo "Built: assets/app-amber.ico (#C64A1E, $(wc -c < assets/app-amber.ico) bytes)"
