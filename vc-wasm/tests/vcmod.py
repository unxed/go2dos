"""Общий для тестов доступ к модулю: сборка, регистры, память, вызов процедуры.

Процедура вызывается так же, как в оракуле (oracle/main.go): адрес возврата
RET_ADDR лежит в стеке по SS:SP, процедура входит по CALL.
"""
import sys
from pathlib import Path

from wasmtime import Engine, Instance, Module, Store

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "tools"))
import build  # noqa: E402

REGS = ("ax", "cx", "dx", "bx", "sp", "bp", "si", "di", "es", "cs", "ss", "ds", "flags")
CODE = 0x1000      # сегмент кода, как в oracle/main.go
RET_ADDR = 0xFF00
SP0 = 0xFFF0       # SP вызывающего; при входе SP = SP0 - 2

_engine = Engine()
_module = None


def module():
    global _module
    if _module is None:
        _module = Module(_engine, build.build())
    return _module


class VC:
    def __init__(self):
        self.store = Store(_engine)
        self.ex = Instance(self.store, module(), []).exports(self.store)
        self.mem = self.ex["memory"]

    def reg(self, name):
        return self.ex[name].value(self.store)

    def set_reg(self, name, v):
        self.ex[name].set_value(self.store, v)

    def write(self, lin, data):
        self.mem.write(self.store, data, lin)

    def read(self, lin, n):
        return bytes(self.mem.read(self.store, lin, lin + n))

    def call(self, proc, **regs):
        """Вызывает процедуру; возвращает регистры после возврата."""
        vals = dict(ax=0, cx=0, dx=0, bx=0, sp=SP0 - 2, bp=0, si=0, di=0,
                    es=CODE, cs=CODE, ss=CODE, ds=CODE, flags=0x0202)
        vals.update(regs)
        for k in REGS:
            self.set_reg(k, vals[k])
        self.write((vals["ss"] << 4) + vals["sp"], RET_ADDR.to_bytes(2, "little"))
        self.ex[proc](self.store, 0)
        out = {k: self.reg(k) for k in REGS}
        if out["sp"] != SP0:
            raise AssertionError(f"{proc}: баланс стека, SP={out['sp']:#x}")
        return out
