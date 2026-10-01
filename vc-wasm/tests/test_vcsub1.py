"""Тесты перевода VCSUB1.INC (vc-wasm/src/vcsub1.wat) — ручные случаи.

Ожидания здесь выведены чтением ассемблерного исходника; сверка с настоящим
VC.COM — в test_oracle_vectors.py (векторы из CPU go2dos).

Запуск:
    pip install wasmtime
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import unittest

from vcmod import VC, module

SCR = 0xB800   # текстовый экран
ATTR = 0x07    # заливка: по ней видно, что байты атрибутов не тронуты


class Vcsub1Test(unittest.TestCase):
    def setUp(self):
        self.vc = VC()

    def test_build(self):
        module()  # модуль собирается и проходит валидацию

    def test_hexcod(self):
        cases = [(0x0, b"0"), (0x9, b"9"), (0xA, b"A"), (0xF, b"F"),
                 (0x1F, b"F"), (0xA3, b"3")]  # старшие биты AL отбрасываются
        for al, ch in cases:
            with self.subTest(al=al):
                self.vc.write(SCR << 4, bytes([ATTR] * 4))
                out = self.vc.call("HexCod", ax=al, es=SCR, di=0)
                self.assertEqual((out["ax"] & 0xFF, out["di"]), (ch[0], 2))
                self.assertEqual(self.vc.read(SCR << 4, 4), bytes([ch[0], ATTR, ATTR, ATTR]))

    def test_hexcod_df(self):
        # STOSB при DF=1 идёт назад: DI = 10 - 1 + 1 = 10
        out = self.vc.call("HexCod", ax=0x5, es=SCR, di=10, flags=0x0602)
        self.assertEqual(out["di"], 10)
        self.assertEqual(self.vc.read((SCR << 4) + 10, 1), b"5")

    def test_hexbyt(self):
        cases = [(0x00, b"00"), (0x09, b"09"), (0x5A, b"5A"),
                 (0xF0, b"F0"), (0xFF, b"FF"), (0xAB, b"AB")]
        for di in (0, 10):
            for al, ch in cases:
                with self.subTest(al=al, di=di):
                    self.vc.write(SCR << 4, bytes([ATTR] * 20))
                    out = self.vc.call("HexByt", ax=0x3300 | al, cx=0x1234, es=SCR, di=di)
                    self.assertEqual(out["ax"], 0x3300 | ch[1])  # AH сохранён, AL — последняя цифра
                    self.assertEqual((out["di"], out["cx"]), (di + 4, 0x1234))
                    want = bytearray([ATTR] * 20)
                    want[di], want[di + 2] = ch[0], ch[1]
                    self.assertEqual(self.vc.read(SCR << 4, 20), bytes(want))

    def test_txtnum(self):
        # (строка, AX, прочитано цифр, CF)
        cases = [
            (b"123 ", 123, 3, 0), (b"0", 0, 1, 0), (b"007", 7, 3, 0),
            (b"12a34", 12, 2, 0), (b"65535x", 65535, 5, 0),
            (b"65536", 65535, 5, 0),      # перенос при ADD -> насыщение
            (b"99999999", 65535, 8, 0),   # переполнение при MUL -> насыщение
            (b"abc", 0, 0, 1), (b"\x00", 0, 0, 1), (b"/5", 0, 0, 1), (b":", 0, 0, 1),
        ]
        seg, si = 0x3000, 0x10
        for text, ax, used, cf in cases:
            with self.subTest(text=text):
                self.vc.write(seg << 4, bytes(0x40))
                self.vc.write((seg << 4) + si, text)
                out = self.vc.call("TxtNum", es=seg, si=si, bx=0x1111, dx=0x2222, flags=0x0602)
                self.assertEqual((out["ax"], out["si"], out["flags"] & 1), (ax, si + used, cf))
                self.assertEqual((out["bx"], out["dx"]), (0x1111, 0x2222))
                self.assertEqual(out["flags"] & 0x400, 0)  # CLD


if __name__ == "__main__":
    unittest.main()
