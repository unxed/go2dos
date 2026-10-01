#!/bin/sh
# Downloads the Dos Navigator 1.51 binary distribution (RIT Research Labs,
# 1999-04-19; free for commercial and non-commercial use, see README.TXT
# inside the archive) for the end-to-end tests and verifies SHA-256.
# Usage: tools/fetch-dn.sh [DIR]
# Then: GO2DOS_DN_DIR=DIR/1.51 go test ./e2e/   (start DN.COM in that directory)
#
# Source:  https://ritlabs.com/download/dn/dn151.zip (redirects to
#          https://download.ritlabs.com/dn/dn151.zip, 786272 bytes, stored zip);
#          fallback: Wayback Machine copy of www.ritlabs.com/download/dn/dn151.zip.
# Contents: DN.COM (1778-byte loader), DN.PRG (MZ executable, 109613 bytes),
#          DN.OVR (overlay "FBOV", 745951 bytes), DN.LNG/DN.DLG/DN.HLP, ...
#          The matching sources are dn151src.zip in the same directory (not fetched).
# Exit codes: 0 ok, 1 checksum/unpack error, 3 source unreachable.
set -eu
dir=${1:-.cache/dn}
sha=f6dee3904945a2f390f8576a5d18d83446f4554e43d6f0a3c75f8aa5b570d7d9
urls="https://ritlabs.com/download/dn/dn151.zip
http://web.archive.org/web/20040530102908id_/http://www.ritlabs.com/download/dn/dn151.zip"
mkdir -p "$dir"
zip="$dir/dn151.zip"
ok=
if [ -f "$zip" ] && echo "$sha  $zip" | sha256sum -c - >/dev/null 2>&1; then ok=1; fi
if [ -z "$ok" ]; then
  for u in $urls; do
    if curl -fsSL --retry 2 --max-time 300 -o "$zip" "$u" 2>/dev/null &&
       echo "$sha  $zip" | sha256sum -c - >/dev/null 2>&1; then ok=1; break; fi
    echo "fetch-dn: $u: unavailable or wrong checksum" >&2
  done
fi
if [ -z "$ok" ]; then
  rm -f "$zip"
  echo "fetch-dn: DN 1.51 could not be downloaded (all sources failed)" >&2
  exit 3
fi
rm -rf "$dir/1.51" && mkdir -p "$dir/1.51"
unzip -q -o "$zip" -d "$dir/1.51"
test -f "$dir/1.51/DN.COM" && test -f "$dir/1.51/DN.PRG" && test -f "$dir/1.51/DN.OVR" || { echo "fetch-dn: DN files missing after unpack" >&2; exit 1; }
echo "DN 1.51 downloaded to $dir/1.51"
