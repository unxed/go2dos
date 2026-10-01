#!/bin/sh
# Downloads the SingleStepTests 8088 v2 suite used by the CPU test
# (https://github.com/SingleStepTests/8088, about 600 MB).
# Usage: tools/fetch-sst.sh [DIR]; then GO2DOS_SST_DIR=DIR go test ./cpu/
set -eu
dir=${1:-.cache/sst}
base=https://raw.githubusercontent.com/SingleStepTests/8088/main/v2
mkdir -p "$dir"
curl -fsSL -o "$dir/metadata.json" "$base/metadata.json"
python3 - "$dir/metadata.json" > "$dir/list.txt" <<'PY'
import json, sys
ops = json.load(open(sys.argv[1]))["opcodes"]
ok = ("normal", "undocumented", "alias")
for k, v in ops.items():
    if "reg" in v:
        for r, rv in v["reg"].items():
            if rv["status"] in ok: print(f"{k}.{r}")
    elif v["status"] in ok:
        print(k)
PY
cd "$dir"
# metadata.json lists a few opcodes (0F, 8F/0, 9B, C6/0, C7/0, F4) whose test
# files are absent upstream: a 404 skips the file instead of failing.
xargs -P 8 -I{} sh -c '[ -s {}.json.gz ] || { curl -fsSL -o {}.json.gz.tmp '"$base"'/{}.json.gz && mv {}.json.gz.tmp {}.json.gz || echo "skipped (not published): {}" >&2; }' < list.txt
echo "SingleStepTests in $dir"
