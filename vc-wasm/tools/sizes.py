"""Размеры тел функций: x86 (из VC.COM) против wasm (из vcsub1.wat).

    python3 vc-wasm/tools/sizes.py [.cache/vc/4.05/VC.COM]

x86: от смещения процедуры (из oracle/vectors.json) до её RET; для TxtNum —
до последовательности 5B 5A C3 (POP BX / POP DX / RET). wasm: размеры тел из
секции кода (включая объявления локальных переменных и завершающий end).
"""
import json
import sys
from pathlib import Path

from wasmtime import wat2wasm

ROOT = Path(__file__).resolve().parent.parent


def leb(buf, pos):
    result = shift = 0
    while True:
        b = buf[pos]
        pos += 1
        result |= (b & 0x7F) << shift
        if not b & 0x80:
            return result, pos
        shift += 7


def wasm_body_sizes(wasm):
    pos = 8  # магическое число и версия
    while pos < len(wasm):
        sec_id = wasm[pos]
        size, pos = leb(wasm, pos + 1)
        if sec_id == 10:  # секция кода
            count, p = leb(wasm, pos)
            sizes = []
            for _ in range(count):
                n, p = leb(wasm, p)
                sizes.append(n)
                p += n
            return sizes
        pos += size
    raise SystemExit("в модуле нет секции кода")


def main():
    com_path = Path(sys.argv[1] if len(sys.argv) > 1 else ".cache/vc/4.05/VC.COM")
    com = com_path.read_bytes()
    vec = json.loads((ROOT / "oracle" / "vectors.json").read_text(encoding="utf-8"))
    off = {k: v - 0x100 for k, v in vec["offsets"].items()}  # смещение в файле
    x86 = {
        "hexcod": com.index(b"\xC3", off["hexcod"]) - off["hexcod"] + 1,
        "hexbyt": com.index(b"\xC3", off["hexbyt"]) - off["hexbyt"] + 1,
        "txtnum": com.index(b"\x5B\x5A\xC3", off["txtnum"]) + 3 - off["txtnum"],
    }
    wat = (ROOT / "src" / "vcsub1.wat").read_text(encoding="utf-8")
    wasm = bytes(wat2wasm(wat))
    sizes = dict(zip(["hexcod", "hexbyt", "txtnum"], wasm_body_sizes(wasm)))
    print(f"{'процедура':<10}{'x86, байт':>11}{'wasm, байт':>12}{'wasm/x86':>10}")
    for k in ("hexcod", "hexbyt", "txtnum"):
        print(f"{k:<10}{x86[k]:>11}{sizes[k]:>12}{sizes[k] / x86[k]:>10.2f}")
    tx, tw = sum(x86.values()), sum(sizes.values())
    print(f"{'итого':<10}{tx:>11}{tw:>12}{tw / tx:>10.2f}")


if __name__ == "__main__":
    main()
