#!/usr/bin/env python3
"""Сравнивает два бинарника VC (JWasm и TASM) по дизассемблированию.

    tools/vc-diff.py СОБРАННЫЙ ЭТАЛОН [--base 0x100] [--top 40]

Нужен ndisasm (пакет nasm). Обе программы дизассемблируются, потоки
сопоставляются по мнемоникам (difflib), а различия группируются по классам:
«opc» — та же мнемоника, но другой код операции (другая форма кодирования),
«replace/insert/delete» — другие инструкции (сдвиг данных, выровненных на
чужие байты, тоже попадает сюда: дизассемблер не отличает код от данных).
Для COM-файлов --base 0x100, для тела MZ-оверлея — 0.
Это инструмент анализа, а не проверка: код выхода всегда 0.
"""
import argparse, collections, difflib, re, subprocess


def dis(path, base):
    out = subprocess.run(["ndisasm", "-b16", "-o%d" % base, path],
                         capture_output=True, text=True, check=True).stdout
    res = []
    for line in out.splitlines():
        m = re.match(r"([0-9A-F]+)\s+([0-9A-F]+)\s+(.*)", line)
        if m:
            res.append((int(m.group(1), 16), m.group(2), m.group(3)))
    return res


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("built")
    ap.add_argument("ref")
    ap.add_argument("--base", type=lambda s: int(s, 0), default=0x100)
    ap.add_argument("--top", type=int, default=40)
    a = ap.parse_args()
    x, y = dis(a.built, a.base), dis(a.ref, a.base)
    mx = [i[2].split()[0] for i in x]
    my = [i[2].split()[0] for i in y]
    classes = collections.Counter()
    blocks = 0
    for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, mx, my, autojunk=False).get_opcodes():
        if tag == "equal":
            for k in range(i2 - i1):
                p, q = x[i1 + k], y[j1 + k]
                if p[1][:2] != q[1][:2] and p[2].split()[0] not in ("db", "dw"):
                    classes[("opc", p[1][:4] + " " + p[2].split()[0],
                             q[1][:4] + " " + q[2].split()[0])] += 1
        else:
            blocks += 1
            fmt = lambda seq: "|".join("%s %s" % (i[1][:4], i[2][:24]) for i in seq[:3])
            classes[(tag, fmt(x[i1:i2]), fmt(y[j1:j2]))] += 1
    print("инструкций: собранный %d, эталон %d; блоков различий %d; классов %d"
          % (len(x), len(y), blocks, len(classes)))
    print("число  класс (собранный -> эталон)")
    for key, n in classes.most_common(a.top):
        print("%5d  %s: %s -> %s" % ((n,) + key))


main()
