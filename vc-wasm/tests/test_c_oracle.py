"""Сверка перевода в C (vc-wasm/c, собран в wasm) с векторами из VC.COM 4.05.

Тот же эталон, что у test_oracle_vectors.py (WAT): векторы
vc-wasm/oracle/vectors.json. Состояние передаётся через экспортированные
регистры (Regs, vc.h) и арену; модуль собирает vc-wasm/c/build.sh.

Запуск:
    sh vc-wasm/c/build.sh
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import json
import struct
import unittest
from pathlib import Path

from wasmtime import Engine, Instance, Module, Store

ROOT = Path(__file__).resolve().parent.parent
WASM = ROOT / "c" / "build" / "vc.wasm"
VECTORS = ROOT / "oracle" / "vectors.json"
REGS = ("ax", "cx", "dx", "bx", "sp", "bp", "si", "di", "es", "cs", "ss", "ds", "flags")
CODE, SCR, STR = 0x1000, 0xB800, 0x3000  # сегменты, как в oracle/main.go
RET_ADDR = 0xFF00


@unittest.skipUnless(WASM.exists(), "нет vc-wasm/c/build/vc.wasm: sh vc-wasm/c/build.sh")
class COracleTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.vec = json.loads(VECTORS.read_text(encoding="utf-8"))
        cls.engine = Engine()
        cls.module = Module(cls.engine, WASM.read_bytes())

    def setUp(self):
        self.store = Store(self.engine)
        self.ex = Instance(self.store, self.module, []).exports(self.store)
        self.mem = self.ex["memory"]
        self.regs = self.ex["vc_regs"](self.store)
        self.arena = self.ex["vc_arena"](self.store)

    def set_regs(self, **kw):
        vals = dict(ax=0, cx=0, dx=0, bx=0, sp=0xFFF0, bp=0, si=0, di=0,
                    es=CODE, cs=CODE, ss=CODE, ds=CODE, flags=0x0202)
        vals.update(kw)
        self.mem.write(self.store, struct.pack("<13H", *(vals[k] for k in REGS)), self.regs)

    def get_regs(self):
        raw = bytes(self.mem.read(self.store, self.regs, self.regs + 26))
        return dict(zip(REGS, struct.unpack("<13H", raw)))

    def run_proc(self, name, **kw):
        # Как в оракуле: адрес возврата в стеке, вход по CALL.
        self.set_regs(sp=0xFFEE, **kw)
        self.mem.write(self.store, struct.pack("<H", RET_ADDR), self.arena + (CODE << 4) + 0xFFEE)
        self.ex[name](self.store)
        out = self.get_regs()
        self.assertEqual(out["sp"], 0xFFF0, f"{name}: баланс стека")
        return out

    def check_hex(self, key, proc):
        fill = bytes([self.vec["screen_fill"]]) * 0x10000
        base = self.arena + (SCR << 4)
        for v in self.vec[key]:
            self.mem.write(self.store, fill, base)
            out = self.run_proc(proc, ax=0xA500 | v["al"], bx=0x1111, cx=0x2222,
                                dx=0x3333, si=0x4444, di=v["di"], es=SCR)
            want = bytearray(fill)
            for off, val in v["diff"]:
                want[off] = val
            got = bytes(self.mem.read(self.store, base, base + 0x10000))
            self.assertEqual((out["ax"] & 0xFF, out["di"]), (v["out_al"], v["out_di"]), v)
            self.assertEqual(out["ax"] >> 8, 0xA5, v)
            self.assertEqual((out["bx"], out["cx"], out["dx"], out["si"]),
                             (0x1111, 0x2222, 0x3333, 0x4444), v)
            self.assertEqual(got, bytes(want), v)

    def test_hexcod(self):
        self.check_hex("hexcod", "HexCod")

    def test_hexbyt(self):
        self.check_hex("hexbyt", "HexByt")

    def test_txtnum(self):
        si = self.vec["txtnum_si"]
        base = self.arena + (STR << 4)
        for v in self.vec["txtnum"]:
            text = bytes.fromhex(v["text"])
            self.mem.write(self.store, bytes(0x100), base)
            self.mem.write(self.store, text, base + si)
            out = self.run_proc("TxtNum", ax=0xBEEF, bx=0x1111, cx=0x2222,
                                dx=0x3333, di=0x4444, si=si, es=STR)
            self.assertEqual((out["ax"], out["si"], out["flags"] & 1),
                             (v["ax"], v["out_si"], v["cf"]), v)
            self.assertEqual((out["bx"], out["cx"], out["dx"], out["di"]),
                             (0x1111, 0x2222, 0x3333, 0x4444), v)


if __name__ == "__main__":
    unittest.main()
