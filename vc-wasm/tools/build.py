"""Склеивает vc-wasm/src/*.wat в один модуль и собирает vc.wasm.

    python3 vc-wasm/tools/build.py        # -> vc-wasm/build/vc.wat, vc.wasm

Файлы src/*.wat — поля модуля без обёртки (module ...); порядок полей в WAT
не важен, rt.wat идёт первым только ради читаемости склейки.
Нужен пакет wasmtime (pip install wasmtime).
"""
from pathlib import Path

from wasmtime import wat2wasm

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "src"
OUT = ROOT / "build"


def module_text(extra=""):
    """Текст модуля; extra — дополнительные поля (только для тестов)."""
    files = [SRC / "rt.wat"] + sorted(p for p in SRC.glob("*.wat") if p.name != "rt.wat")
    parts = [f";; ===== {p.name}\n{p.read_text(encoding='utf-8')}" for p in files]
    return "(module\n" + "\n".join(parts) + "\n" + extra + "\n)\n"


def build():
    text = module_text()
    wasm = bytes(wat2wasm(text))
    OUT.mkdir(exist_ok=True)
    (OUT / "vc.wat").write_text(text, encoding="utf-8")
    (OUT / "vc.wasm").write_bytes(wasm)
    return wasm


if __name__ == "__main__":
    print(f"build.py: {len(build())} байт -> {OUT / 'vc.wasm'}")
