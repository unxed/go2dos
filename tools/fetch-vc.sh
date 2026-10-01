#!/bin/sh
# Downloads the Volkov Commander binaries built from the published sources
# (BSD-2-Clause, https://github.com/ddanila/vc) for the end-to-end tests,
# verifying the SHA-256 of VC.COM. Usage: tools/fetch-vc.sh [DIR]
# Then: GO2DOS_VC_DIR=DIR go test ./e2e/
set -eu
dir=${1:-.cache/vc}
base=https://github.com/ddanila/vc/releases/download/latest-build
mkdir -p "$dir"
for v in 4.05 4.99.09; do
  curl -fsSL -o "$dir/vc-$v.zip" "$base/vc-$v.zip"
  rm -rf "$dir/$v" && mkdir -p "$dir/$v"
  unzip -q -o "$dir/vc-$v.zip" -d "$dir/$v"
  rm -rf "$dir/$v/jwasm"
done
# Pinned so that a rebuilt release cannot silently change what we test.
check() { echo "$2  $1" | sha256sum -c - >/dev/null || { echo "checksum mismatch: $1" >&2; exit 1; }; }
check "$dir/4.05/VC.COM" b408f14da5bcba174f5e86107437b22b2863ee6ec72f79bdadf1b812607405fb
echo "VC downloaded to $dir"
