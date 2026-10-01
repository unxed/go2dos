#!/bin/sh
# Downloads Norton Commander 5.51 (English, the newest DOS version) for the
# end-to-end tests and verifies the SHA-256 of the archive.
# Usage: tools/fetch-nc.sh [DIR]
# Then: GO2DOS_NC_DIR=DIR/5.51 go test ./e2e/
#
# NC is NOT part of the repository and is not freely licensed; the archive
# is an abandonware copy used only for compatibility testing.
# Source:  http://old-dos.ru/files/file_1458.html, file id 2256, listed there as
#          "Norton Commander 5.51, 1998, English" ("Norton Commander.ver.5.51.English.rar",
#          2779709 bytes, RAR 2.9 with three 1.44 MB floppy images NC551_1..3.IMG).
#          Fallback: the Wayback Machine copy of the same URL (same SHA-256).
# Version: README.TXT on disk 1 says "NORTON COMMANDER Version 5.5", Copyright 1998
#          Symantec; the site calls it 5.51. NC.EXE (3870 bytes) and NCMAIN.EXE
#          (264602 bytes) are stored uncompressed on disk 1 (NC551_1.IMG).
# Only disk 1 is unpacked (NC.EXE, NCMAIN.EXE, NCEDIT.EXE, NCZIP.EXE, NCFF.EXE,
# NC.HLP, README.TXT, ...). Disks 2 and 3 hold viewers, screen savers and
# other files compressed with Symantec's installer; they are not needed.
# Needs: curl, sha256sum, unrar (or unar), 7z (opens the FAT image).
# Exit codes: 0 ok, 1 checksum/unpack error or tool missing, 3 source unreachable
# (callers may treat 3 as "skip").
set -eu
dir=${1:-.cache/nc}
sha=eac55480eadd2d71983d465988ad26bd2916904f2483e624c5f7059b16778ef0
urls="http://old-dos.ru/dl.php?id=2256
http://web.archive.org/web/2id_/http://old-dos.ru/dl.php?id=2256"
mkdir -p "$dir"
rar="$dir/nc-5.51-en.rar"
ok=
if [ -f "$rar" ] && echo "$sha  $rar" | sha256sum -c - >/dev/null 2>&1; then ok=1; fi
if [ -z "$ok" ]; then
  for u in $urls; do
    if curl -fsSL --retry 2 --max-time 300 -o "$rar" "$u" 2>/dev/null &&
       echo "$sha  $rar" | sha256sum -c - >/dev/null 2>&1; then ok=1; break; fi
    echo "fetch-nc: $u: unavailable or wrong checksum" >&2
  done
fi
if [ -z "$ok" ]; then
  rm -f "$rar"
  echo "fetch-nc: NC 5.51 could not be downloaded (all sources failed)" >&2
  exit 3
fi
command -v 7z >/dev/null 2>&1 || { echo "fetch-nc: 7z not found" >&2; exit 1; }
tmp="$dir/unpack"
rm -rf "$tmp" "$dir/5.51" && mkdir -p "$tmp" "$dir/5.51"
if command -v unrar >/dev/null 2>&1; then
  unrar x -y -inul "$rar" "$tmp/" || { echo "fetch-nc: unrar failed" >&2; exit 1; }
elif command -v unar >/dev/null 2>&1; then
  unar -q -f -o "$tmp" "$rar" >/dev/null || { echo "fetch-nc: unar failed" >&2; exit 1; }
else
  echo "fetch-nc: neither unrar nor unar found (7z cannot unpack this RAR)" >&2; exit 1
fi
img=$(find "$tmp" -name 'NC551_1.IMG' | head -n 1)
[ -n "$img" ] || { echo "fetch-nc: NC551_1.IMG not found in the archive" >&2; exit 1; }
7z x -y -o"$dir/5.51" "$img" >/dev/null || { echo "fetch-nc: cannot read the floppy image" >&2; exit 1; }
rm -rf "$tmp"
test -f "$dir/5.51/NC.EXE" && test -f "$dir/5.51/NCMAIN.EXE" || { echo "fetch-nc: NC.EXE/NCMAIN.EXE missing after unpack" >&2; exit 1; }
echo "NC 5.51 downloaded to $dir/5.51"
