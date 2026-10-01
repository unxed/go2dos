#!/bin/sh
# Records headless go2dos traces of Norton Commander 5.0 and Dos Navigator 1.51
# (tools/fetch-nc.sh, tools/fetch-dn.sh) and writes a plain summary of what
# was observed: exit status, last line of stderr, final screen. Nothing here
# interprets the results; docs/NC-TESTING.md quotes them from CI runs.
# Usage: tools/trace-nc-dn.sh GO2DOS_BINARY NC_DIR DN_DIR OUT_DIR
# A missing NC_DIR or DN_DIR is skipped (and said so in the summary).
set -u
bin=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
nc=$2; dn=$3; out=$4
mkdir -p "$out"
sum="$out/summary.txt"
: > "$sum"
# name:dir:program:keys
run() {
  name=$1; src=$2; prog=$3; keys=$4; extra=$5
  if [ ! -f "$src/$prog" ]; then
    echo "== $name: SKIPPED ($src/$prog not present)" >> "$sum"; return
  fi
  work=$(mktemp -d)
  cp -r "$src"/. "$work"/
  printf 'Sample README file\r\n' > "$work/README.TXT"
  mkdir -p "$work/SUBDIR"
  ( cd "$work" && timeout 120 "$bin" -headless -timeout 40s $extra \
      -trace "$out/$name.trace.jsonl" -screen-out "$out/$name.screen.txt" \
      -dump-dir "$out/$name-dump" -dump-on-exit \
      ${keys:+-keys "$keys"} "./$prog" ) > "$out/$name.stdout.txt" 2> "$out/$name.stderr.txt"
  rc=$?
  echo "== $name: exit status $rc" >> "$sum"
  echo "-- last 5 stderr lines:" >> "$sum"
  tail -n 5 "$out/$name.stderr.txt" >> "$sum"
  echo "-- trace: $(wc -l < "$out/$name.trace.jsonl" 2>/dev/null || echo 0) lines" >> "$sum"
  echo "-- final screen:" >> "$sum"
  cat "$out/$name.screen.txt" 2>/dev/null >> "$sum"
  echo >> "$sum"
  rm -rf "$work"
}
nckeys='<wait:5s><screen><Tab><wait:1s><Tab><F9><wait:1s><screen><Esc><wait:1s><F10><wait:1s><Enter>'
dnkeys='<wait:5s><screen><Tab><wait:1s><Tab><F9><wait:1s><screen><Esc><wait:1s><F10><wait:1s><Enter>'
run nc-exe-boot    "$nc" NC.EXE     "" ""
run nc-exe-keys    "$nc" NC.EXE     "$nckeys" ""
run nc-exe-lenient "$nc" NC.EXE     "$nckeys" "-lenient"
run ncmain-boot    "$nc" NCMAIN.EXE "" ""
run ncmain-keys    "$nc" NCMAIN.EXE "$nckeys" ""
run ncmain-lenient "$nc" NCMAIN.EXE "$nckeys" "-lenient"
run dn-boot        "$dn" DN.COM     "" ""
run dn-keys        "$dn" DN.COM     "$dnkeys" ""
run dn-lenient     "$dn" DN.COM     "$dnkeys" "-lenient"
gzip -f "$out"/*.trace.jsonl 2>/dev/null
cat "$sum"
exit 0
