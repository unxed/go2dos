#!/bin/sh
# Downloads Norton Commander 5.0 (English) for the end-to-end tests and
# verifies the SHA-256 of the archive. Usage: tools/fetch-nc.sh [DIR]
# Then: GO2DOS_NC_DIR=DIR/5.00 go test ./e2e/
#
# NC is NOT part of the repository and is not freely licensed; the archive
# is an abandonware copy used only for compatibility testing.
# Source:  http://old-dos.ru/files/file_1458.html, file id 168
#          ("Norton Commander.ver.5.00.English.zip", 2802061 bytes, stored zip);
#          fallback: the Wayback Machine copy of the same URL.
# Version: 5.0 (README.TXT: "NORTON COMMANDER Version 5.0", files dated
#          1995-05-04). NC.EXE (3772 bytes, loader) and NCMAIN.EXE
#          (233522 bytes) lie in the archive root, no installer is needed.
# Exit codes: 0 ok, 1 checksum/unpack error, 3 source unreachable (callers
# may treat 3 as "skip").
set -eu
dir=${1:-.cache/nc}
sha=eaa55357f4915e1aa5830365cbde495391ea7980e1d4162d39b049a234d97931
urls="http://old-dos.ru/dl.php?id=168
http://web.archive.org/web/20231213042936id_/http://old-dos.ru/dl.php?id=168"
mkdir -p "$dir"
zip="$dir/nc-5.00-en.zip"
ok=
if [ -f "$zip" ] && echo "$sha  $zip" | sha256sum -c - >/dev/null 2>&1; then ok=1; fi
if [ -z "$ok" ]; then
  for u in $urls; do
    if curl -fsSL --retry 2 --max-time 300 -o "$zip" "$u" 2>/dev/null &&
       echo "$sha  $zip" | sha256sum -c - >/dev/null 2>&1; then ok=1; break; fi
    echo "fetch-nc: $u: unavailable or wrong checksum" >&2
  done
fi
if [ -z "$ok" ]; then
  rm -f "$zip"
  echo "fetch-nc: NC 5.0 could not be downloaded (all sources failed)" >&2
  exit 3
fi
rm -rf "$dir/5.00" && mkdir -p "$dir/5.00"
unzip -q -o "$zip" -d "$dir/5.00"
rm -f "$dir/5.00/NC5CRK11.ARJ"   # a third-party crack in the archive; not needed
test -f "$dir/5.00/NC.EXE" && test -f "$dir/5.00/NCMAIN.EXE" || { echo "fetch-nc: NC.EXE/NCMAIN.EXE missing after unpack" >&2; exit 1; }
echo "NC 5.0 downloaded to $dir/5.00"
