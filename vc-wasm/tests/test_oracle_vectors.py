"""Сверка WAT-перевода VCSUB1.INC с настоящим VC.COM 4.05.

Векторы (vc-wasm/oracle/vectors.json) записаны программой vc-wasm/oracle:
она запускает процедуры из VC.COM в CPU go2dos. Здесь те же входы подаются в
модуль из vc-wasm/src и сравниваются выходы. Перегенерация векторов — в README.

Запуск:
    pip install wasmtime
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import json
import unittest
from pathlib import Path

from vcmod import VC

VECTORS = Path(__file__).resolve().parent.parent / "oracle" / "vectors.json"
SCR, STR = 0xB800, 0x3000  # сегменты, как в oracle/main.go


class OracleTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.vec = json.loads(VECTORS.read_text(encoding="utf-8"))

    def setUp(self):
        self.vc = VC()

    def test_source_is_real_vc(self):
        # Оракул должен быть записан с настоящего VC.COM, найденного по сигнатурам.
        self.assertEqual(len(self.vec["source"]["sha256"]), 64)
        self.assertEqual(set(self.vec["offsets"]), {"hexcod", "hexbyt", "txtnum"})

    def check_hex(self, key, proc):
        fill = bytes([self.vec["screen_fill"]]) * 0x10000
        for v in self.vec[key]:
            self.vc.write(SCR << 4, fill)
            out = self.vc.call(proc, ax=0xA500 | v["al"], bx=0x1111, cx=0x2222,
                               dx=0x3333, si=0x4444, di=v["di"], es=SCR)
            want = bytearray(fill)
            for off, val in v["diff"]:
                want[off] = val
            self.assertEqual((out["ax"] & 0xFF, out["di"]), (v["out_al"], v["out_di"]), v)
            self.assertEqual((out["ax"] >> 8, out["bx"], out["cx"], out["dx"], out["si"]),
                             (0xA5, 0x1111, 0x2222, 0x3333, 0x4444), v)
            self.assertEqual(self.vc.read(SCR << 4, 0x10000), bytes(want), v)

    def test_hexcod_against_vc(self):
        self.check_hex("hexcod", "HexCod")

    def test_hexbyt_against_vc(self):
        self.check_hex("hexbyt", "HexByt")

    def test_txtnum_against_vc(self):
        si = self.vec["txtnum_si"]
        for v in self.vec["txtnum"]:
            text = bytes.fromhex(v["text"])
            self.vc.write(STR << 4, bytes(0x100))
            self.vc.write((STR << 4) + si, text)
            out = self.vc.call("TxtNum", ax=0xBEEF, bx=0x1111, cx=0x2222,
                               dx=0x3333, di=0x4444, si=si, es=STR)
            self.assertEqual((out["ax"], out["si"], out["flags"] & 1),
                             (v["ax"], v["out_si"], v["cf"]), v)
            self.assertEqual((out["bx"], out["cx"], out["dx"], out["di"]),
                             (0x1111, 0x2222, 0x3333, 0x4444), v)
            self.assertEqual(self.vc.read((STR << 4) + si, len(text)), text, v)


if __name__ == "__main__":
    unittest.main()
