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
xargs -P 8 -I{} sh -c '[ -s {}.json.gz ] || curl -fsSL -o {}.json.gz.tmp '"$base"'/{}.json.gz && { [ -s {}.json.gz ] || mv {}.json.gz.tmp {}.json.gz; }' < list.txt
echo "SingleStepTests in $dir"
