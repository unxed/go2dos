package cpu

import "fmt"

// Bus is the CPU's view of memory: linear addresses, byte granularity.
type Bus interface {
	Read8(addr uint32) byte
	Write8(addr uint32, v byte)
}

// Ports is the CPU's view of the I/O address space.
type Ports interface {
	In8(port uint16) byte
	Out8(port uint16, v byte)
}

// StopReason says why Run returned.
type StopReason int

const (
	StopBudget StopReason = iota // instruction budget used up
	StopHalt                     // HLT executed
	StopTrap                     // an HLE handler asked to stop (see CPU.RequestStop)
	StopFault                    // unsupported instruction or HLE error; see CPU.Err
)

func (r StopReason) String() string {
	return [...]string{"budget", "halt", "trap", "fault"}[r]
}

// Executor runs code over a shared State. The interpreter is the first
// implementation; a JIT backend is meant to plug in behind the same interface.
type Executor interface {
	Run(budget int) StopReason
}

// TrapOpcode is the two-byte prefix of the HLE trap instruction
// "FE 38 lo hi": GRP4 /7 with a [BX+SI] operand, undefined on real CPUs.
// The 16-bit immediate selects the Go handler.
var TrapOpcode = [2]byte{0xFE, 0x38}

// ErrRetry may be returned by a trap handler that cannot complete yet (for
// example, waiting for a key). The trap instruction is re-executed later with
// interrupts enabled, and the run loop stops with StopTrap so the host can idle.
var ErrRetry = fmt.Errorf("retry")

// Fault describes why execution stopped abnormally.
type Fault struct {
	CS, IP uint16
	Bytes  []byte
	Msg    string
}

func (f *Fault) Error() string {
	return fmt.Sprintf("%04X:%04X [% X]: %s", f.CS, f.IP, f.Bytes, f.Msg)
}

// HistoryLen is the size of the ring buffer of recently executed addresses.
const HistoryLen = 1024

// CPU is the real-mode interpreter.
type CPU struct {
	State
	Mem Bus
	IO  Ports

	// Trap is called for the HLE trap instruction.
	Trap func(c *CPU, n uint16) error
	// IntrPending is set by the machine when an IRQ may be pending; Intr is
	// then asked for the vector whenever interrupts are enabled.
	IntrPending bool
	Intr        func() (vector byte, ok bool)

	Err      error // set when Run returns StopFault
	Executed uint64

	// History of instruction start addresses (CS<<16 | IP), most recent last.
	History    [HistoryLen]uint32
	historyPos int

	stop     bool
	halted   bool
	shadow   bool // interrupts inhibited for one instruction (MOV SS, POP SS, STI)
	segOvr   int  // segment override for the current instruction, -1 if none
	rep      byte // 0, 0xF2 or 0xF3
	opCS     uint16
	opIP     uint16
	fault    error
	trapOpIP uint16
}

// New creates a CPU in the real-mode reset state, with CS:IP at 0000:0000.
func New(mem Bus, io Ports) *CPU {
	c := &CPU{Mem: mem, IO: io}
	for i := range c.S {
		c.SetSeg(i, 0)
	}
	c.SetFlags(0)
	return c
}

// RequestStop makes Run return StopTrap after the current instruction.
func (c *CPU) RequestStop() { c.stop = true }

// HistorySnapshot returns the recorded addresses, oldest first.
func (c *CPU) HistorySnapshot() []uint32 {
	n := HistoryLen
	if c.Executed < HistoryLen {
		n = int(c.Executed)
	}
	out := make([]uint32, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, c.History[(c.historyPos-n+i+HistoryLen)%HistoryLen])
	}
	return out
}

// HistoryPos returns the index the next instruction address will be stored at.
func (c *CPU) HistoryPos() int { return c.historyPos }

// Halted reports whether the CPU is stopped on HLT.
func (c *CPU) Halted() bool { return c.halted }

// Run executes up to budget instructions.
func (c *CPU) Run(budget int) StopReason {
	c.stop = false
	for i := 0; i < budget; i++ {
		if c.IntrPending && !c.shadow && c.Flags&FlagIF != 0 {
			if v, ok := c.Intr(); ok {
				c.halted = false
				c.Interrupt(v)
			}
		}
		if c.halted {
			return StopHalt
		}
		c.Step()
		if c.fault != nil {
			c.Err = c.fault
			c.fault = nil
			return StopFault
		}
		if c.halted {
			return StopHalt
		}
		if c.stop {
			return StopTrap
		}
	}
	return StopBudget
}

// Step executes one instruction (including its prefixes).
func (c *CPU) Step() {
	c.shadow = false
	tf := c.Flags&FlagTF != 0
	c.opCS, c.opIP = c.S[CS].Sel, c.IP
	c.History[c.historyPos] = uint32(c.opCS)<<16 | uint32(c.opIP)
	c.historyPos = (c.historyPos + 1) % HistoryLen
	c.Executed++
	c.segOvr = -1
	c.rep = 0
	c.exec()
	if tf && c.fault == nil && !c.halted {
		c.Interrupt(1)
	}
}

// Interrupt delivers interrupt vector n through the IVT.
func (c *CPU) Interrupt(n byte) {
	c.push(c.Flags)
	c.Flags &^= FlagIF | FlagTF
	c.push(c.S[CS].Sel)
	c.push(c.IP)
	c.IP = c.rdLin16(uint32(n) * 4)
	c.SetSeg(CS, c.rdLin16(uint32(n)*4+2))
}

func (c *CPU) faultf(format string, a ...any) {
	if c.fault != nil {
		return
	}
	b := make([]byte, 8)
	for i := range b {
		b[i] = c.Mem.Read8(c.S[CS].Base + uint32(c.opIP+uint16(i)))
	}
	c.fault = &Fault{CS: c.opCS, IP: c.opIP, Bytes: b, Msg: fmt.Sprintf(format, a...)}
	// Leave CS:IP at the faulting instruction for diagnostics.
	c.SetSeg(CS, c.opCS)
	c.IP = c.opIP
}

// --- memory helpers -------------------------------------------------------

func (c *CPU) Read8(seg int, off uint16) byte { return c.Mem.Read8(c.S[seg].Base + uint32(off)) }

func (c *CPU) Write8(seg int, off uint16, v byte) { c.Mem.Write8(c.S[seg].Base+uint32(off), v) }

// Read16 reads a word; the offset wraps within the segment as on the 8086.
func (c *CPU) Read16(seg int, off uint16) uint16 {
	b := c.S[seg].Base
	return uint16(c.Mem.Read8(b+uint32(off))) | uint16(c.Mem.Read8(b+uint32(off+1)))<<8
}

func (c *CPU) Write16(seg int, off uint16, v uint16) {
	b := c.S[seg].Base
	c.Mem.Write8(b+uint32(off), byte(v))
	c.Mem.Write8(b+uint32(off+1), byte(v>>8))
}

func (c *CPU) rdLin16(a uint32) uint16 {
	return uint16(c.Mem.Read8(a)) | uint16(c.Mem.Read8(a+1))<<8
}

func (c *CPU) fetch8() byte {
	v := c.Read8(CS, c.IP)
	c.IP++
	return v
}

func (c *CPU) fetch16() uint16 {
	v := c.Read16(CS, c.IP)
	c.IP += 2
	return v
}

func (c *CPU) push(v uint16) {
	c.R[SP] -= 2
	c.Write16(SS, c.R[SP], v)
}

func (c *CPU) pop() uint16 {
	v := c.Read16(SS, c.R[SP])
	c.R[SP] += 2
	return v
}

// Push and Pop are exported for HLE code that builds stack frames.
func (c *CPU) Push(v uint16) { c.push(v) }
func (c *CPU) Pop() uint16   { return c.pop() }

// --- ModR/M -------------------------------------------------------------

type modrm struct {
	mod, reg, rm byte
	isReg        bool
	seg          int
	off          uint16
}

func (c *CPU) decodeModRM() modrm {
	b := c.fetch8()
	m := modrm{mod: b >> 6, reg: (b >> 3) & 7, rm: b & 7}
	if m.mod == 3 {
		m.isReg = true
		return m
	}
	seg := DS
	var off uint16
	switch m.rm {
	case 0:
		off = c.R[BX] + c.R[SI]
	case 1:
		off = c.R[BX] + c.R[DI]
	case 2:
		off, seg = c.R[BP]+c.R[SI], SS
	case 3:
		off, seg = c.R[BP]+c.R[DI], SS
	case 4:
		off = c.R[SI]
	case 5:
		off = c.R[DI]
	case 6:
		if m.mod == 0 {
			off = c.fetch16()
		} else {
			off, seg = c.R[BP], SS
		}
	case 7:
		off = c.R[BX]
	}
	switch m.mod {
	case 1:
		off += uint16(int8(c.fetch8()))
	case 2:
		off += c.fetch16()
	}
	if c.segOvr >= 0 {
		seg = c.segOvr
	}
	m.seg, m.off = seg, off
	return m
}

func (c *CPU) rm8(m *modrm) byte {
	if m.isReg {
		return c.Reg8(int(m.rm))
	}
	return c.Read8(m.seg, m.off)
}

func (c *CPU) setRM8(m *modrm, v byte) {
	if m.isReg {
		c.SetReg8(int(m.rm), v)
		return
	}
	c.Write8(m.seg, m.off, v)
}

func (c *CPU) rm16(m *modrm) uint16 {
	if m.isReg {
		return c.R[m.rm]
	}
	return c.Read16(m.seg, m.off)
}

func (c *CPU) setRM16(m *modrm, v uint16) {
	if m.isReg {
		c.R[m.rm] = v
		return
	}
	c.Write16(m.seg, m.off, v)
}

// dataSeg returns the source segment for string and moffs instructions.
func (c *CPU) dataSeg() int {
	if c.segOvr >= 0 {
		return c.segOvr
	}
	return DS
}
