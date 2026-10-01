"""Тесты среды rt.wat: INT через хост (регистровый блок, кадр, IRET) и
правило «память — только через rt.wat» (PLAN §3, П8; на нём держится
возможность позже ввести API экрана, не трогая переведённый код).

Запуск:
    python3 -m unittest discover -s vc-wasm/tests -v
"""
import re
import unittest
from pathlib import Path

from vcmod import CODE, REGS, SP0, VC

SRC = Path(__file__).resolve().parent.parent / "src"

# Тестовая «процедура»: INT 21h с адресом следующей команды 1234h, затем RET.
T_INT = """
(func (export "t_int") (param $L i32)
  (call $int (i32.const 0x21) (i32.const 0x1234))
  (call $ret (i32.const 0)) (return))
"""


class IntTest(unittest.TestCase):
    def setUp(self):
        self.vc = VC(T_INT)
        self.regs = self.vc.ex["vc_regs"](self.vc.store)

    def test_frame_block_iret(self):
        seen = {}

        def host(vc, n):
            seen["n"] = n
            seen["block"] = {k: vc.word(self.regs + 2 * i) for i, k in enumerate(REGS)}
            sp = seen["block"]["sp"]
            base = (seen["block"]["ss"] << 4)
            seen["frame"] = [vc.word(base + sp + d) for d in (0, 2, 4)]  # IP, CS, FLAGS
            vc.set_word(self.regs + 0, 0x4C00 | 0x55)                    # AX от хоста
            vc.set_word(self.regs + 6, 0xBEEF)                           # BX от хоста
            vc.set_word(base + sp + 4, seen["frame"][2] | 1)             # CF=1 в кадре

        self.vc.host_int = host
        out = self.vc.call("t_int", ax=0x3D00, bx=0x1111, dx=0x0080, ds=0x2000)
        self.assertEqual(seen["n"], 0x21)
        b = seen["block"]
        self.assertEqual((b["ax"], b["bx"], b["dx"], b["ds"], b["cs"]),
                         (0x3D00, 0x1111, 0x0080, 0x2000, CODE))
        self.assertEqual(b["sp"], SP0 - 2 - 6)                    # кадр INT: 3 слова
        self.assertEqual(seen["frame"], [0x1234, CODE, 0xF202])   # IP, CS, FLAGS (8086: биты 12-15)
        self.assertEqual((out["ax"], out["bx"]), (0x4C55, 0xBEEF))
        self.assertEqual(out["flags"] & 1, 1)                     # CF из кадра (IRET)
        self.assertEqual(out["cs"], CODE)

    def test_cf_cleared_by_host(self):
        def host(vc, n):
            sp = vc.word(self.regs + 8)
            ss = vc.word(self.regs + 20)
            vc.set_word((ss << 4) + sp + 4, vc.word((ss << 4) + sp + 4) & ~1)

        self.vc.host_int = host
        out = self.vc.call("t_int", flags=0x0203)
        self.assertEqual(out["flags"] & 1, 0)


class MemoryChokePointTest(unittest.TestCase):
    def test_no_raw_memory_access_outside_rt(self):
        raw = re.compile(r"\b(i32|i64)\.(load|store)\w*|\bmemory\.(copy|fill|init|grow)\b")
        bad = []
        for p in sorted(SRC.glob("*.wat")):
            if p.name == "rt.wat":
                continue
            for no, line in enumerate(p.read_text(encoding="utf-8").splitlines(), 1):
                code = line.split(";;", 1)[0]
                if raw.search(code):
                    bad.append(f"{p.name}:{no}: {line.strip()}")
        self.assertEqual(bad, [], "память — только через $rb/$wb/$rw/$ww и помощники rt.wat")


if __name__ == "__main__":
    unittest.main()
