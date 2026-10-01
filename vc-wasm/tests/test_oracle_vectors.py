"""Сверка WAT-перевода VCSUB1.INC с настоящим VC.COM 4.05.

Векторы (vc-wasm/oracle/vectors.json) записаны программой vc-wasm/oracle:
она запускает процедуры из VC.COM в CPU go2dos. Здесь те же входы подаются в
vc-wasm/src/vcsub1.wat и сравниваются выходы. Перегенерация векторов — в README.

Запуск:
    pip install wasmtime
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import json
import unittest
from pathlib import Path

from wasmtime import Engine, Instance, Module, Store, wat2wasm

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "src" / "vcsub1.wat"
VECTORS = ROOT / "oracle" / "vectors.json"
SCREEN = 0x10000   # окно текстового экрана (D1)
WINDOW = 0x10000   # окно ES:0000-FFFF, которое сверяет оракул
ES_STR = 0x2000    # произвольная база строки для TxtNum (в оракуле другая)


class OracleTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.vec = json.loads(VECTORS.read_text(encoding="utf-8"))
        cls.engine = Engine()
        cls.module = Module(cls.engine, bytes(wat2wasm(SRC.read_text(encoding="utf-8"))))

    def setUp(self):
        self.store = Store(self.engine)
        self.ex = Instance(self.store, self.module, []).exports(self.store)
        self.mem = self.ex["memory"]
        self.fill = bytes([self.vec["screen_fill"]]) * WINDOW

    def call(self, name, *args):
        return self.ex[name](self.store, *args)

    def check_hex(self, name):
        for v in self.vec[name]:
            self.mem.write(self.store, self.fill, SCREEN)
            out_al, out_di = self.call(name, v["al"], SCREEN, v["di"])
            want = bytearray(self.fill)
            for off, val in v["diff"]:
                want[off] = val
            got = bytes(self.mem.read(self.store, SCREEN, SCREEN + WINDOW))
            self.assertEqual((out_al, out_di), (v["out_al"], v["out_di"]), v)
            self.assertEqual(got, bytes(want), v)

    def test_source_is_real_vc(self):
        # Оракул должен быть записан с настоящего VC.COM, найденного по сигнатурам.
        src = self.vec["source"]
        self.assertEqual(len(src["sha256"]), 64)
        self.assertEqual(set(self.vec["offsets"]), {"hexcod", "hexbyt", "txtnum"})

    def test_hexcod_against_vc(self):
        self.check_hex("hexcod")

    def test_hexbyt_against_vc(self):
        self.check_hex("hexbyt")

    def test_txtnum_against_vc(self):
        si = self.vec["txtnum_si"]
        zeros = bytes(0x100)
        for v in self.vec["txtnum"]:
            text = bytes.fromhex(v["text"])
            self.mem.write(self.store, zeros, ES_STR)
            self.mem.write(self.store, text, ES_STR + si)
            ax, out_si, cf = self.call("txtnum", ES_STR, si)
            self.assertEqual((ax, out_si, cf), (v["ax"], v["out_si"], v["cf"]), v)
            self.assertEqual(bytes(self.mem.read(self.store, ES_STR + si, ES_STR + si + len(text))), text, v)


if __name__ == "__main__":
    unittest.main()
