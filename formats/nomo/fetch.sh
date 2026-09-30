#!/usr/bin/env bash
# Obtain the Nomo engine and the math font this package embeds, and pin them.
#
# The engine is Nomo's WebAssembly module — the artifact the Nomo editor runs in
# the browser — so a worksheet rendered here is the page the editor draws, bit
# for bit. It is fetched from a Nomo release rather than built, so this repo
# needs no Rust toolchain, and every file is checked against the SHA256SUMS the
# release published before it is accepted. The hashes of what was accepted are
# then written to assets/SHA256SUMS, which is what --verify checks.
#
#   ./fetch.sh v0.7.0                # from the GitHub release of that tag
#   ./fetch.sh --local ../../../nomo # from a Nomo checkout's own build
#   ./fetch.sh --verify              # check assets/ against assets/SHA256SUMS
#
# --local is for a module that has not been released yet. In the Nomo checkout,
# first run:
#   cargo build -p nomo-wasm --release --target wasm32-unknown-unknown
#   ./scripts/build-web.sh     (for web/dist/fonts)

set -euo pipefail
cd "$(dirname "$0")"

repo=https://github.com/rveen/nomo/releases/download
out=assets

sums() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

pin() {
    (cd "$out" && sums nomo.wasm NOTICE.txt stix-two-math-subset.woff2 OFL.txt > SHA256SUMS)
    echo "$1" > "$out/VERSION"
    cat "$out/SHA256SUMS"
    echo "pinned: $1"
}

case "${1:-}" in
--verify)
    (cd "$out" && sums -c --quiet SHA256SUMS)
    echo "ok: assets match assets/SHA256SUMS ($(cat "$out/VERSION"))"
    ;;
--local)
    nomo=${2:?"--local needs the path of a Nomo checkout"}
    wasm="$nomo/target/wasm32-unknown-unknown/release/nomo_wasm.wasm"
    fonts="$nomo/web/dist/fonts"
    [ -f "$wasm" ] || { echo "error: no $wasm — build nomo-wasm first" >&2; exit 1; }
    [ -f "$fonts/stix-two-math-subset.woff2" ] || { echo "error: no fonts in $fonts — run ./scripts/build-web.sh there" >&2; exit 1; }
    mkdir -p "$out"
    cp "$wasm" "$out/nomo.wasm"
    (cd "$nomo" && node scripts/notice.mjs --wasm "$OLDPWD/$out/NOTICE.txt" >/dev/null)
    cp "$fonts/stix-two-math-subset.woff2" "$fonts/OFL.txt" "$out/"
    pin "local build of nomo $(git -C "$nomo" describe --tags --always --dirty)"
    ;;
v*)
    tag=$1
    work=$(mktemp -d)
    trap 'rm -rf "$work"' EXIT
    for f in SHA256SUMS.txt "nomo_wasm-$tag.wasm" "nomo_wasm-$tag-NOTICE.txt" "nomo-web-$tag.zip"; do
        curl -fsSL -o "$work/$f" "$repo/$tag/$f"
    done
    # Only the files fetched are checked; the release also lists binaries.
    (cd "$work" && grep -E "nomo_wasm-|nomo-web-" SHA256SUMS.txt | sums -c --quiet -)
    (cd "$work" && unzip -q "nomo-web-$tag.zip" "nomo-web-$tag/fonts/*")
    mkdir -p "$out"
    cp "$work/nomo_wasm-$tag.wasm" "$out/nomo.wasm"
    cp "$work/nomo_wasm-$tag-NOTICE.txt" "$out/NOTICE.txt"
    cp "$work/nomo-web-$tag/fonts/stix-two-math-subset.woff2" "$work/nomo-web-$tag/fonts/OFL.txt" "$out/"
    pin "nomo $tag"
    ;;
*)
    echo "usage: $0 <tag> | --local <nomo-checkout> | --verify" >&2
    exit 2
    ;;
esac
