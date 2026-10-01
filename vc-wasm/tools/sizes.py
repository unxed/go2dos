"""Размеры тел функций: x86 (из VC.COM) против wasm (модуль из vc-wasm/src).

    python3 vc-wasm/tools/sizes.py [.cache/vc/4.05/VC.COM]

x86: от смещения процедуры (из oracle/vectors.json) до её RET; для TxtNum —
до последовательности 5B 5A C3 (POP BX / POP DX / RET). wasm: размеры тел из
секции кода (с объявлениями локальных переменных и завершающим end), имена —
из секции имён; общие функции rt.wat печатаются отдельной суммой.
"""
import json
import sys
from pathlib import Path

import build

ROOT = Path(__file__).resolve().parent.parent
PROCS = {"hexcod": "HexCod", "hexbyt": "HexByt", "txtnum": "TxtNum"}


def leb(buf, pos):
    result = shift = 0
    while True:
        b = buf[pos]
        pos += 1
        result |= (b & 0x7F) << shift
        if not b & 0x80:
            return result, pos
        shift += 7


def body_sizes(wasm):
    """{имя функции: размер тела} по секциям импорта, кода и имён."""
    pos, imports, bodies, names = 8, 0, [], {}
    while pos < len(wasm):
        sec, (size, pos) = wasm[pos], leb(wasm, pos + 1)
        end = pos + size
        if sec == 2:  # импорт: считаем только импортированные функции
            count, p = leb(wasm, pos)
            for _ in range(count):
                for _ in range(2):
                    n, p = leb(wasm, p)
                    p += n
                kind = wasm[p]
                p += 1
                if kind != 0:
                    raise SystemExit("ожидались только импорты функций")
                imports += 1
                _, p = leb(wasm, p)
        elif sec == 10:
            count, p = leb(wasm, pos)
            for _ in range(count):
                n, p = leb(wasm, p)
                bodies.append(n)
                p += n
        elif sec == 0:
            n, p = leb(wasm, pos)
            if wasm[p:p + n] == b"name":
                p += n
                while p < end:
                    sub, (sl, p) = wasm[p], leb(wasm, p + 1)
                    se = p + sl
                    if sub == 1:
                        count, q = leb(wasm, p)
                        for _ in range(count):
                            idx, q = leb(wasm, q)
                            n, q = leb(wasm, q)
                            names[idx] = wasm[q:q + n].decode()
                            q += n
                    p = se
        pos = end
    return {names.get(imports + i, f"f{imports + i}"): n for i, n in enumerate(bodies)}


def main():
    com = Path(sys.argv[1] if len(sys.argv) > 1 else ".cache/vc/4.05/VC.COM").read_bytes()
    vec = json.loads((ROOT / "oracle" / "vectors.json").read_text(encoding="utf-8"))
    off = {k: v - 0x100 for k, v in vec["offsets"].items()}  # смещение в файле
    x86 = {
        "hexcod": com.index(b"\xC3", off["hexcod"]) - off["hexcod"] + 1,
        "hexbyt": com.index(b"\xC3", off["hexbyt"]) - off["hexbyt"] + 1,
        "txtnum": com.index(b"\x5B\x5A\xC3", off["txtnum"]) + 3 - off["txtnum"],
    }
    sizes = body_sizes(build.build())
    print(f"{'процедура':<10}{'x86, байт':>11}{'wasm, байт':>12}{'wasm/x86':>10}")
    for k, name in PROCS.items():
        print(f"{name:<10}{x86[k]:>11}{sizes[name]:>12}{sizes[name] / x86[k]:>10.2f}")
    tx = sum(x86.values())
    tw = sum(sizes[n] for n in PROCS.values())
    rt = sum(n for name, n in sizes.items() if name not in PROCS.values())
    print(f"{'итого':<10}{tx:>11}{tw:>12}{tw / tx:>10.2f}")
    print(f"общие функции rt.wat (разовые): {rt} байт")


if __name__ == "__main__":
    main()
