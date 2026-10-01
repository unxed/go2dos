"""Тесты перевода VCSUB1.INC (vc-wasm/src/vcsub1.wat).

Ожидания выведены чтением ассемблерного исходника, а не сверкой с VC.COM:
до итерации 2 (дифференциальный прогон в CPU go2dos) это гипотеза.

Запуск:
    pip install wasmtime
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import unittest
from pathlib import Path

from wasmtime import Engine, Instance, Module, Store, wat2wasm

SRC = Path(__file__).resolve().parent.parent / "src" / "vcsub1.wat"
SCREEN = 0x10000  # окно текстового экрана (D1)
ATTR = 0x07       # заливка: по ней видно, что байты атрибутов не тронуты


class Vcsub1Test(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.wasm = bytes(wat2wasm(SRC.read_text(encoding="utf-8")))
        cls.engine = Engine()
        cls.module = Module(cls.engine, cls.wasm)

    def setUp(self):
        self.store = Store(self.engine)
        self.ex = Instance(self.store, self.module, []).exports(self.store)
        self.mem = self.ex["memory"]

    def call(self, name, *args):
        return self.ex[name](self.store, *args)

    def put(self, addr, data):
        self.mem.write(self.store, data, addr)

    def get(self, addr, n):
        return bytes(self.mem.read(self.store, addr, addr + n))

    def test_size_report(self):
        # Метрика эксперимента: размер wasm-модуля (сравнение с x86 — итерация 3).
        print(f"\nvcsub1.wasm: {len(self.wasm)} байт")
        self.assertGreater(len(self.wasm), 8)

    def test_hexcod(self):
        cases = [(0x0, b"0"), (0x9, b"9"), (0xA, b"A"), (0xF, b"F"),
                 (0x1F, b"F"), (0xA3, b"3")]  # старшие биты AL отбрасываются
        for al, ch in cases:
            with self.subTest(al=al):
                self.put(SCREEN, bytes([ATTR] * 4))
                out_al, out_di = self.call("hexcod", al, SCREEN, 0)
                self.assertEqual(out_al, ch[0])
                self.assertEqual(out_di, 2)
                self.assertEqual(self.get(SCREEN, 4),
                                 bytes([ch[0], ATTR, ATTR, ATTR]))

    def test_hexbyt(self):
        cases = [(0x00, b"00"), (0x09, b"09"), (0x5A, b"5A"),
                 (0xF0, b"F0"), (0xFF, b"FF"), (0xAB, b"AB")]
        for di in (0, 10):
            for al, ch in cases:
                with self.subTest(al=al, di=di):
                    self.put(SCREEN, bytes([ATTR] * 20))
                    out_al, out_di = self.call("hexbyt", al, SCREEN, di)
                    self.assertEqual(out_al, ch[1])  # AL = последний символ
                    self.assertEqual(out_di, di + 4)
                    want = bytearray([ATTR] * 20)
                    want[di] = ch[0]
                    want[di + 2] = ch[1]
                    self.assertEqual(self.get(SCREEN, 20), bytes(want))

    def test_txtnum(self):
        # (строка, число в AX, прочитано цифр, CF)
        cases = [
            (b"123 ", 123, 3, 0),
            (b"0", 0, 1, 0),             # ноль — это число, CF=0
            (b"007", 7, 3, 0),
            (b"12a34", 12, 2, 0),        # остановка на нецифре
            (b"65535x", 65535, 5, 0),
            (b"65536", 65535, 5, 0),     # перенос при ADD -> насыщение
            (b"99999999", 65535, 8, 0),  # переполнение при MUL -> насыщение
            (b"700000x", 65535, 6, 0),   # насыщение держится до конца цифр
            (b"abc", 0, 0, 1),           # цифр нет -> CF=1
            (b"\x00", 0, 0, 1),
            (b"/5", 0, 0, 1),            # '/' = 2Fh < '0'
            (b":", 0, 0, 1),             # ':' = 3Ah > '9'
            (b"\xff", 0, 0, 1),
        ]
        si = 0x10
        for es in (0x0000, 0x2000):
            for text, ax, used, cf in cases:
                with self.subTest(text=text, es=es):
                    self.put(es + si, text)
                    out_ax, out_si, out_cf = self.call("txtnum", es, si)
                    self.assertEqual(out_ax, ax)
                    self.assertEqual(out_si, si + used)
                    self.assertEqual(out_cf, cf)
                    self.assertEqual(self.get(es + si, len(text)), text)
                    self.put(es + si, bytes(len(text)))  # очистка для следующего


if __name__ == "__main__":
    unittest.main()
