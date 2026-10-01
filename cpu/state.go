// Package cpu implements the x86 processor state and a real-mode interpreter.
//
// The CPU model is an 8086 with the 80186 instruction extensions (the same
// combination as the NEC V20/V30): 8086 semantics where the two differ
// (PUSH SP pushes the decremented value, FLAGS bits 12-15 read as 1, shift
// counts are not masked), plus PUSHA/POPA, ENTER/LEAVE, BOUND, INS/OUTS,
// PUSH imm, IMUL imm and shifts by imm8.
package cpu

import "fmt"

// General register indices, in instruction-encoding order.
const (
	AX = iota
	CX
	DX
	BX
	SP
	BP
	SI
	DI
)

// Segment register indices, in instruction-encoding order.
const (
	ES = iota
	CS
	SS
	DS
	FS
	GS
)

// FLAGS bits.
const (
	FlagCF uint16 = 1 << 0
	FlagPF uint16 = 1 << 2
	FlagAF uint16 = 1 << 4
	FlagZF uint16 = 1 << 6
	FlagSF uint16 = 1 << 7
	FlagTF uint16 = 1 << 8
	FlagIF uint16 = 1 << 9
	FlagDF uint16 = 1 << 10
	FlagOF uint16 = 1 << 11

	flagsWritable uint16 = 0x0FD5 // CF PF AF ZF SF TF IF DF OF
	flagsFixed    uint16 = 0xF002 // 8086: bits 12-15 and bit 1 read as 1
)

// Segment is a segment register with its descriptor cache. In real mode the
// cache is derived from the selector; protected mode will fill it from a
// descriptor without changing how addresses are computed.
type Segment struct {
	Sel   uint16
	Base  uint32
	Limit uint32
}

// State is the architectural state shared by every Executor.
type State struct {
	R     [8]uint16
	S     [6]Segment
	IP    uint16
	Flags uint16
	CR0   uint32 // present for protected mode, unused in real mode
}

// SetSeg loads a segment register in real mode.
func (s *State) SetSeg(i int, sel uint16) {
	s.S[i] = Segment{Sel: sel, Base: uint32(sel) << 4, Limit: 0xFFFF}
}

// Seg returns the selector of segment register i.
func (s *State) Seg(i int) uint16 { return s.S[i].Sel }

// SetFlags stores a FLAGS value, normalizing the fixed bits.
func (s *State) SetFlags(v uint16) { s.Flags = v&flagsWritable | flagsFixed }

// Flag reports whether all bits in f are set.
func (s *State) Flag(f uint16) bool { return s.Flags&f == f }

// SetFlag sets or clears the bits in f.
func (s *State) SetFlag(f uint16, on bool) {
	if on {
		s.Flags |= f
	} else {
		s.Flags &^= f
	}
}

// Reg8 returns an 8-bit register by encoding (AL CL DL BL AH CH DH BH).
func (s *State) Reg8(i int) byte {
	if i < 4 {
		return byte(s.R[i])
	}
	return byte(s.R[i-4] >> 8)
}

// SetReg8 stores an 8-bit register by encoding.
func (s *State) SetReg8(i int, v byte) {
	if i < 4 {
		s.R[i] = s.R[i]&0xFF00 | uint16(v)
	} else {
		s.R[i-4] = s.R[i-4]&0x00FF | uint16(v)<<8
	}
}

// Convenience accessors used by the HLE layers.
func (s *State) AL() byte { return byte(s.R[AX]) }
func (s *State) AH() byte { return byte(s.R[AX] >> 8) }
func (s *State) BL() byte { return byte(s.R[BX]) }
func (s *State) BH() byte { return byte(s.R[BX] >> 8) }
func (s *State) CL() byte { return byte(s.R[CX]) }
func (s *State) CH() byte { return byte(s.R[CX] >> 8) }
func (s *State) DL() byte { return byte(s.R[DX]) }
func (s *State) DH() byte { return byte(s.R[DX] >> 8) }

func (s *State) SetAL(v byte) { s.SetReg8(0, v) }
func (s *State) SetAH(v byte) { s.SetReg8(4, v) }
func (s *State) SetBL(v byte) { s.SetReg8(3, v) }
func (s *State) SetBH(v byte) { s.SetReg8(7, v) }
func (s *State) SetCL(v byte) { s.SetReg8(1, v) }
func (s *State) SetCH(v byte) { s.SetReg8(5, v) }
func (s *State) SetDL(v byte) { s.SetReg8(2, v) }
func (s *State) SetDH(v byte) { s.SetReg8(6, v) }

// String formats the registers for diagnostics.
func (s *State) String() string {
	return fmt.Sprintf("AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X SP=%04X "+
		"DS=%04X ES=%04X SS=%04X CS=%04X IP=%04X FL=%04X",
		s.R[AX], s.R[BX], s.R[CX], s.R[DX], s.R[SI], s.R[DI], s.R[BP], s.R[SP],
		s.S[DS].Sel, s.S[ES].Sel, s.S[SS].Sel, s.S[CS].Sel, s.IP, s.Flags)
}
