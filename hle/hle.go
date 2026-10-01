// Package hle is the shared core of the high-level-emulated firmware and DOS:
// it owns the trap registry, places stubs in the ROM area, and traces calls.
package hle

import (
	"errors"
	"fmt"
	"time"

	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/mem"
)

// Handler implements one trap. It runs with the stack frame of the INT that
// reached the stub: IP at SS:SP, CS at SS:SP+2, FLAGS at SS:SP+4.
type Handler func(e *Env) error

// ROMSeg is the segment of the BIOS ROM where stubs live.
const ROMSeg = 0xF000

// ErrUnsupported marks a function the HLE layer does not implement; it
// stops the machine (fail fast) with full diagnostics.
var ErrUnsupported = errors.New("unsupported")

// Unsupported builds an ErrUnsupported error with details.
func Unsupported(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, fmt.Sprintf(format, a...))
}

// Exit is returned (wrapped in a stop) when the top-level program terminates.
type Exit struct{ Code byte }

func (x *Exit) Error() string { return fmt.Sprintf("program exited with code %d", x.Code) }

// Env is passed to every handler.
type Env struct {
	CPU *cpu.CPU
	Mem *mem.Memory
	CP  *cp.Codepage
	Now func() time.Time

	// Idle is called when the guest is waiting (keyboard poll, INT 28h, ...).
	Idle func()
	// Stop records why the machine must stop after this trap.
	Stop error

	Trace *Tracer

	// Event, if set, is told about notable DOS events ("exec-start",
	// "exec-return"); the machine uses it to arm the post-event execution
	// log (diagnostics).
	Event func(kind string)

	handlers []entry
	romNext  uint16
	cur      *Record
}

type entry struct {
	name string
	fn   Handler
}

// New creates the environment; handlers are registered by the bios and dos packages.
func New(c *cpu.CPU, m *mem.Memory, page *cp.Codepage, now func() time.Time, tr *Tracer) *Env {
	e := &Env{CPU: c, Mem: m, CP: page, Now: now, Trace: tr, romNext: 0x0100, Idle: func() {}}
	c.Trap = e.dispatch
	return e
}

// Register adds a handler and returns its trap number.
func (e *Env) Register(name string, fn Handler) uint16 {
	e.handlers = append(e.handlers, entry{name, fn})
	return uint16(len(e.handlers) - 1)
}

// Emit places code in the ROM area and returns its offset in ROMSeg.
func (e *Env) Emit(code []byte) uint16 {
	off := e.romNext
	e.Mem.Poke(mem.Lin(ROMSeg, off), code...)
	e.romNext += uint16(len(code))
	return off
}

// Trap returns the 4-byte trap instruction for handler n.
func Trap(n uint16) []byte {
	return []byte{cpu.TrapOpcode[0], cpu.TrapOpcode[1], byte(n), byte(n >> 8)}
}

// IRETStub emits "trap n; IRET" and returns its offset.
func (e *Env) IRETStub(n uint16) uint16 {
	return e.Emit(append(Trap(n), 0xCF))
}

// SetVector points interrupt vector v at seg:off.
func (e *Env) SetVector(v byte, seg, off uint16) {
	e.Mem.W16(uint32(v)*4, off)
	e.Mem.W16(uint32(v)*4+2, seg)
}

// Vector returns the current IVT entry.
func (e *Env) Vector(v byte) (seg, off uint16) {
	return e.Mem.R16(uint32(v)*4 + 2), e.Mem.R16(uint32(v) * 4)
}

// HookInt registers handler fn and points vector v at an "trap; IRET" stub.
func (e *Env) HookInt(v byte, name string, fn Handler) {
	off := e.IRETStub(e.Register(name, fn))
	e.SetVector(v, ROMSeg, off)
}

func (e *Env) frame() uint32 {
	return e.CPU.S[cpu.SS].Base + uint32(e.CPU.R[cpu.SP])
}

// CallerFlags returns the FLAGS saved by the INT that reached the stub.
func (e *Env) CallerFlags() uint16 {
	return e.CPU.Read16(cpu.SS, e.CPU.R[cpu.SP]+4)
}

// SetFlag changes a flag in the stack frame so it survives the IRET.
func (e *Env) SetFlag(f uint16, on bool) {
	off := e.CPU.R[cpu.SP] + 4
	v := e.CPU.Read16(cpu.SS, off)
	if on {
		v |= f
	} else {
		v &^= f
	}
	e.CPU.Write16(cpu.SS, off, v)
}

// SetCF sets the carry flag returned to the caller.
func (e *Env) SetCF(on bool) { e.SetFlag(cpu.FlagCF, on) }

// SetZF sets the zero flag returned to the caller.
func (e *Env) SetZF(on bool) { e.SetFlag(cpu.FlagZF, on) }

// Caller returns CS:IP of the instruction after the INT.
func (e *Env) Caller() (cs, ip uint16) {
	return e.CPU.Read16(cpu.SS, e.CPU.R[cpu.SP]+2), e.CPU.Read16(cpu.SS, e.CPU.R[cpu.SP])
}

// Note attaches a detail (a path, a decoded argument) to the current trace record.
func (e *Env) Note(format string, a ...any) {
	if e.cur != nil {
		e.cur.Note = fmt.Sprintf(format, a...)
	}
}

// Seg returns a segment register value.
func (e *Env) Seg(i int) uint16 { return e.CPU.S[i].Sel }

func (e *Env) dispatch(c *cpu.CPU, n uint16) error {
	if int(n) >= len(e.handlers) {
		return fmt.Errorf("unknown trap %d", n)
	}
	h := e.handlers[n]
	rec := e.Trace.begin(e, h.name)
	e.cur = rec
	err := h.fn(e)
	e.cur = nil
	e.Trace.end(e, rec, err)
	if err != nil {
		return err
	}
	if e.Stop != nil {
		c.RequestStop()
	}
	return nil
}

// Linear helpers for handlers.

// Ptr returns the linear address seg:off.
func Ptr(seg, off uint16) uint32 { return mem.Lin(seg, off) }

// DSDX returns the linear address DS:DX, the most common DOS argument.
func (e *Env) DSDX() uint32 { return mem.Lin(e.Seg(cpu.DS), e.CPU.R[cpu.DX]) }
