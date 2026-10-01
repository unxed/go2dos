package cpu

func (c *CPU) jumpIf(cond bool) {
	d := int8(c.fetch8())
	if cond {
		c.IP += uint16(d)
	}
}

func (c *CPU) cond(n byte) bool {
	f := c.Flags
	cf, zf, sf, of, pf := f&FlagCF != 0, f&FlagZF != 0, f&FlagSF != 0, f&FlagOF != 0, f&FlagPF != 0
	var r bool
	switch n >> 1 {
	case 0:
		r = of
	case 1:
		r = cf
	case 2:
		r = zf
	case 3:
		r = cf || zf
	case 4:
		r = sf
	case 5:
		r = pf
	case 6:
		r = sf != of
	case 7:
		r = zf || sf != of
	}
	if n&1 != 0 {
		r = !r
	}
	return r
}

func (c *CPU) exec() {
	for {
		op := c.fetch8()
		switch op {
		case 0x26, 0x2E, 0x36, 0x3E:
			c.segOvr = int(op>>3) & 3
			continue
		case 0xF0, 0xF1: // LOCK (F1 is an alias on the 8086)
			continue
		case 0xF2, 0xF3:
			c.rep = op
			continue
		}
		c.execOp(op)
		return
	}
}

func (c *CPU) execOp(op byte) {
	switch {
	case op < 0x40 && op&7 < 6:
		c.aluForm(op>>3, op&7)
		return
	case op >= 0x40 && op <= 0x47:
		c.R[op&7] = uint16(c.inc(uint32(c.R[op&7]), w16))
		return
	case op >= 0x48 && op <= 0x4F:
		c.R[op&7] = uint16(c.dec(uint32(c.R[op&7]), w16))
		return
	case op >= 0x50 && op <= 0x57:
		if op == 0x54 { // 8086: pushes the decremented SP
			c.R[SP] -= 2
			c.Write16(SS, c.R[SP], c.R[SP])
			return
		}
		c.push(c.R[op&7])
		return
	case op >= 0x58 && op <= 0x5F:
		v := c.pop()
		c.R[op&7] = v
		return
	case op >= 0x70 && op <= 0x7F:
		c.jumpIf(c.cond(op & 0xF))
		return
	case op >= 0x91 && op <= 0x97:
		c.R[AX], c.R[op&7] = c.R[op&7], c.R[AX]
		return
	case op >= 0xB0 && op <= 0xB7:
		c.SetReg8(int(op&7), c.fetch8())
		return
	case op >= 0xB8 && op <= 0xBF:
		c.R[op&7] = c.fetch16()
		return
	case op >= 0xD8 && op <= 0xDF: // ESC: no FPU present, operand is decoded and ignored
		c.decodeModRM()
		return
	}

	switch op {
	case 0x06, 0x0E, 0x16, 0x1E:
		c.push(c.S[op>>3].Sel)
	case 0x07, 0x17, 0x1F:
		c.SetSeg(int(op>>3), c.pop())
		if op == 0x17 {
			c.shadow = true
		}
	case 0x0F: // POP CS (8086)
		c.SetSeg(CS, c.pop())
	case 0x27:
		c.daa()
	case 0x2F:
		c.das()
	case 0x37:
		c.aaa()
	case 0x3F:
		c.aas()

	case 0x60: // PUSHA
		t := c.R[SP]
		for i := 0; i < 8; i++ {
			if i == SP {
				c.push(t)
			} else {
				c.push(c.R[i])
			}
		}
	case 0x61: // POPA
		for i := 7; i >= 0; i-- {
			v := c.pop()
			if i != SP {
				c.R[i] = v
			}
		}
	case 0x62: // BOUND
		m := c.decodeModRM()
		if m.isReg {
			c.faultf("BOUND with register operand")
			return
		}
		lo, hi := int16(c.Read16(m.seg, m.off)), int16(c.Read16(m.seg, m.off+2))
		if v := int16(c.R[m.reg]); v < lo || v > hi {
			c.IP = c.opIP
			c.Interrupt(5)
		}
	case 0x68:
		c.push(c.fetch16())
	case 0x6A:
		c.push(uint16(int8(c.fetch8())))
	case 0x69, 0x6B:
		m := c.decodeModRM()
		a := int32(int16(c.rm16(&m)))
		var b int32
		if op == 0x69 {
			b = int32(int16(c.fetch16()))
		} else {
			b = int32(int8(c.fetch8()))
		}
		r := a * b
		c.R[m.reg] = uint16(r)
		c.SetFlag(FlagCF|FlagOF, r != int32(int16(r)))
	case 0x6C, 0x6D, 0x6E, 0x6F:
		c.stringOp(op)

	case 0x80, 0x81, 0x82, 0x83:
		m := c.decodeModRM()
		if op&1 == 0 {
			a := uint32(c.rm8(&m))
			r := c.alu(m.reg, a, uint32(c.fetch8()), w8)
			if m.reg != 7 {
				c.setRM8(&m, byte(r))
			}
		} else {
			a := uint32(c.rm16(&m))
			var b uint32
			if op == 0x81 {
				b = uint32(c.fetch16())
			} else {
				b = uint32(uint16(int8(c.fetch8())))
			}
			r := c.alu(m.reg, a, b, w16)
			if m.reg != 7 {
				c.setRM16(&m, uint16(r))
			}
		}
	case 0x84:
		m := c.decodeModRM()
		c.logic(uint32(c.rm8(&m))&uint32(c.Reg8(int(m.reg))), w8)
	case 0x85:
		m := c.decodeModRM()
		c.logic(uint32(c.rm16(&m))&uint32(c.R[m.reg]), w16)
	case 0x86:
		m := c.decodeModRM()
		a := c.rm8(&m)
		c.setRM8(&m, c.Reg8(int(m.reg)))
		c.SetReg8(int(m.reg), a)
	case 0x87:
		m := c.decodeModRM()
		a := c.rm16(&m)
		c.setRM16(&m, c.R[m.reg])
		c.R[m.reg] = a
	case 0x88:
		m := c.decodeModRM()
		c.setRM8(&m, c.Reg8(int(m.reg)))
	case 0x89:
		m := c.decodeModRM()
		c.setRM16(&m, c.R[m.reg])
	case 0x8A:
		m := c.decodeModRM()
		c.SetReg8(int(m.reg), c.rm8(&m))
	case 0x8B:
		m := c.decodeModRM()
		c.R[m.reg] = c.rm16(&m)
	case 0x8C:
		m := c.decodeModRM()
		c.setRM16(&m, c.S[m.reg&3].Sel)
	case 0x8D:
		m := c.decodeModRM()
		if m.isReg {
			c.faultf("LEA with register operand")
			return
		}
		c.R[m.reg] = m.off
	case 0x8E:
		m := c.decodeModRM()
		s := int(m.reg & 3)
		c.SetSeg(s, c.rm16(&m))
		if s == SS {
			c.shadow = true
		}
	case 0x8F:
		m := c.decodeModRM()
		if m.reg != 0 {
			c.faultf("undefined 8F /%d", m.reg)
			return
		}
		v := c.pop()
		c.setRM16(&m, v)

	case 0x90:
	case 0x98:
		c.R[AX] = uint16(int8(c.AL()))
	case 0x99:
		if c.R[AX]&0x8000 != 0 {
			c.R[DX] = 0xFFFF
		} else {
			c.R[DX] = 0
		}
	case 0x9A:
		ip := c.fetch16()
		cs := c.fetch16()
		c.push(c.S[CS].Sel)
		c.push(c.IP)
		c.SetSeg(CS, cs)
		c.IP = ip
	case 0x9B: // WAIT
	case 0x9C:
		c.push(c.Flags)
	case 0x9D:
		c.SetFlags(c.pop())
	case 0x9E:
		c.SetFlags(c.Flags&0xFF00 | uint16(c.AH()))
	case 0x9F:
		c.SetAH(byte(c.Flags))

	case 0xA0:
		c.SetAL(c.Read8(c.dataSeg(), c.fetch16()))
	case 0xA1:
		c.R[AX] = c.Read16(c.dataSeg(), c.fetch16())
	case 0xA2:
		c.Write8(c.dataSeg(), c.fetch16(), c.AL())
	case 0xA3:
		c.Write16(c.dataSeg(), c.fetch16(), c.R[AX])
	case 0xA4, 0xA5, 0xA6, 0xA7, 0xAA, 0xAB, 0xAC, 0xAD, 0xAE, 0xAF:
		c.stringOp(op)
	case 0xA8:
		c.logic(uint32(c.AL())&uint32(c.fetch8()), w8)
	case 0xA9:
		c.logic(uint32(c.R[AX])&uint32(c.fetch16()), w16)

	case 0xC0, 0xC1, 0xD0, 0xD1, 0xD2, 0xD3:
		m := c.decodeModRM()
		var n uint
		switch op {
		case 0xC0, 0xC1:
			n = uint(c.fetch8())
		case 0xD0, 0xD1:
			n = 1
		default:
			n = uint(c.CL())
		}
		if op&1 == 0 {
			c.setRM8(&m, byte(c.shift(m.reg, uint32(c.rm8(&m)), n, w8)))
		} else {
			c.setRM16(&m, uint16(c.shift(m.reg, uint32(c.rm16(&m)), n, w16)))
		}
	case 0xC2, 0xC3:
		ip := c.pop()
		if op == 0xC2 {
			n := c.fetch16()
			c.R[SP] += n
		}
		c.IP = ip
	case 0xC4, 0xC5:
		m := c.decodeModRM()
		if m.isReg {
			c.faultf("LES/LDS with register operand")
			return
		}
		c.R[m.reg] = c.Read16(m.seg, m.off)
		s := ES
		if op == 0xC5 {
			s = DS
		}
		c.SetSeg(s, c.Read16(m.seg, m.off+2))
	case 0xC6:
		m := c.decodeModRM()
		if m.reg != 0 {
			c.faultf("undefined C6 /%d", m.reg)
			return
		}
		c.setRM8(&m, c.fetch8())
	case 0xC7:
		m := c.decodeModRM()
		if m.reg != 0 {
			c.faultf("undefined C7 /%d", m.reg)
			return
		}
		c.setRM16(&m, c.fetch16())
	case 0xC8: // ENTER
		size := c.fetch16()
		level := c.fetch8() & 0x1F
		c.push(c.R[BP])
		frame := c.R[SP]
		if level > 0 {
			for i := byte(1); i < level; i++ {
				c.R[BP] -= 2
				c.push(c.Read16(SS, c.R[BP]))
			}
			c.push(frame)
		}
		c.R[BP] = frame
		c.R[SP] -= size
	case 0xC9: // LEAVE
		c.R[SP] = c.R[BP]
		c.R[BP] = c.pop()
	case 0xCA, 0xCB:
		ip := c.pop()
		cs := c.pop()
		if op == 0xCA {
			c.R[SP] += c.fetch16()
		}
		c.IP = ip
		c.SetSeg(CS, cs)
	case 0xCC:
		c.Interrupt(3)
	case 0xCD:
		c.Interrupt(c.fetch8())
	case 0xCE:
		if c.Flags&FlagOF != 0 {
			c.Interrupt(4)
		}
	case 0xCF:
		c.IP = c.pop()
		c.SetSeg(CS, c.pop())
		c.SetFlags(c.pop())

	case 0xD4: // AAM
		d := c.fetch8()
		if d == 0 {
			c.setSZP(0, w8) // measured 8088 behaviour
			c.Interrupt(0)
			return
		}
		al := c.AL()
		c.SetAH(al / d)
		c.SetAL(al % d)
		c.logic(uint32(c.AL()), w8)
	case 0xD5: // AAD
		d := c.fetch8()
		r := c.add(uint32(c.AL()), uint32(c.AH())*uint32(d)&0xFF, 0, w8)
		c.R[AX] = uint16(r)
	case 0xD6: // SALC
		if c.Flags&FlagCF != 0 {
			c.SetAL(0xFF)
		} else {
			c.SetAL(0)
		}
	case 0xD7: // XLAT
		c.SetAL(c.Read8(c.dataSeg(), c.R[BX]+uint16(c.AL())))

	case 0xE0, 0xE1, 0xE2:
		d := int8(c.fetch8())
		c.R[CX]--
		ok := c.R[CX] != 0
		if op == 0xE0 {
			ok = ok && c.Flags&FlagZF == 0
		} else if op == 0xE1 {
			ok = ok && c.Flags&FlagZF != 0
		}
		if ok {
			c.IP += uint16(d)
		}
	case 0xE3:
		c.jumpIf(c.R[CX] == 0)
	case 0xE4:
		c.SetAL(c.IO.In8(uint16(c.fetch8())))
	case 0xE5:
		p := uint16(c.fetch8())
		c.R[AX] = uint16(c.IO.In8(p)) | uint16(c.IO.In8(p+1))<<8
	case 0xE6:
		c.IO.Out8(uint16(c.fetch8()), c.AL())
	case 0xE7:
		p := uint16(c.fetch8())
		c.IO.Out8(p, c.AL())
		c.IO.Out8(p+1, c.AH())
	case 0xE8:
		d := c.fetch16()
		c.push(c.IP)
		c.IP += d
	case 0xE9:
		d := c.fetch16()
		c.IP += d
	case 0xEA:
		ip := c.fetch16()
		cs := c.fetch16()
		c.IP = ip
		c.SetSeg(CS, cs)
	case 0xEB:
		c.jumpIf(true)
	case 0xEC:
		c.SetAL(c.IO.In8(c.R[DX]))
	case 0xED:
		c.R[AX] = uint16(c.IO.In8(c.R[DX])) | uint16(c.IO.In8(c.R[DX]+1))<<8
	case 0xEE:
		c.IO.Out8(c.R[DX], c.AL())
	case 0xEF:
		c.IO.Out8(c.R[DX], c.AL())
		c.IO.Out8(c.R[DX]+1, c.AH())

	case 0xF4:
		c.halted = true
	case 0xF5:
		c.Flags ^= FlagCF
	case 0xF6, 0xF7:
		c.group3(op)
	case 0xF8:
		c.Flags &^= FlagCF
	case 0xF9:
		c.Flags |= FlagCF
	case 0xFA:
		c.Flags &^= FlagIF
	case 0xFB:
		if c.Flags&FlagIF == 0 {
			c.shadow = true
		}
		c.Flags |= FlagIF
	case 0xFC:
		c.Flags &^= FlagDF
	case 0xFD:
		c.Flags |= FlagDF
	case 0xFE:
		c.group4()
	case 0xFF:
		c.group5()
	default:
		c.faultf("unsupported opcode %02X", op)
	}
}

func (c *CPU) aluForm(aop, form byte) {
	switch form {
	case 0, 2:
		m := c.decodeModRM()
		a, b := uint32(c.rm8(&m)), uint32(c.Reg8(int(m.reg)))
		if form == 2 {
			a, b = b, a
		}
		r := c.alu(aop, a, b, w8)
		if aop != 7 {
			if form == 0 {
				c.setRM8(&m, byte(r))
			} else {
				c.SetReg8(int(m.reg), byte(r))
			}
		}
	case 1, 3:
		m := c.decodeModRM()
		a, b := uint32(c.rm16(&m)), uint32(c.R[m.reg])
		if form == 3 {
			a, b = b, a
		}
		r := c.alu(aop, a, b, w16)
		if aop != 7 {
			if form == 1 {
				c.setRM16(&m, uint16(r))
			} else {
				c.R[m.reg] = uint16(r)
			}
		}
	case 4:
		r := c.alu(aop, uint32(c.AL()), uint32(c.fetch8()), w8)
		if aop != 7 {
			c.SetAL(byte(r))
		}
	case 5:
		r := c.alu(aop, uint32(c.R[AX]), uint32(c.fetch16()), w16)
		if aop != 7 {
			c.R[AX] = uint16(r)
		}
	}
}

func (c *CPU) group3(op byte) {
	m := c.decodeModRM()
	if op == 0xF6 {
		v := uint32(c.rm8(&m))
		switch m.reg {
		case 0, 1:
			c.logic(v&uint32(c.fetch8()), w8)
		case 2:
			c.setRM8(&m, ^byte(v))
		case 3:
			c.setRM8(&m, byte(c.sub(0, v, 0, w8)))
		case 4:
			r := uint16(c.AL()) * uint16(v)
			c.R[AX] = r
			c.SetFlag(FlagCF|FlagOF, r>>8 != 0)
			c.setSZP(uint32(r>>8), w8)
		case 5:
			r := int16(int8(c.AL())) * int16(int8(v))
			c.R[AX] = uint16(r)
			c.SetFlag(FlagCF|FlagOF, r != int16(int8(r)))
			c.setSZP(uint32(uint16(r)>>8), w8)
		case 6:
			if v == 0 {
				c.Interrupt(0)
				return
			}
			q, r := c.R[AX]/uint16(v), c.R[AX]%uint16(v)
			if q > 0xFF {
				c.Interrupt(0)
				return
			}
			c.SetAL(byte(q))
			c.SetAH(byte(r))
		case 7:
			if v == 0 {
				c.Interrupt(0)
				return
			}
			a, d := int16(c.R[AX]), int16(int8(v))
			q, r := a/d, a%d
			if c.rep != 0 { // 8088 quirk: a REP prefix negates the quotient
				q = -q
			}
			if q > 127 || q < -127 || (a == -32768 && d == -1) {
				c.Interrupt(0)
				return
			}
			c.SetAL(byte(q))
			c.SetAH(byte(r))
		}
		return
	}
	v := uint32(c.rm16(&m))
	switch m.reg {
	case 0, 1:
		c.logic(v&uint32(c.fetch16()), w16)
	case 2:
		c.setRM16(&m, ^uint16(v))
	case 3:
		c.setRM16(&m, uint16(c.sub(0, v, 0, w16)))
	case 4:
		r := uint32(c.R[AX]) * v
		c.R[AX], c.R[DX] = uint16(r), uint16(r>>16)
		c.SetFlag(FlagCF|FlagOF, c.R[DX] != 0)
		c.setSZP(uint32(c.R[DX]), w16)
	case 5:
		r := int32(int16(c.R[AX])) * int32(int16(v))
		c.R[AX], c.R[DX] = uint16(r), uint16(uint32(r)>>16)
		c.SetFlag(FlagCF|FlagOF, r != int32(int16(r)))
		c.setSZP(uint32(c.R[DX]), w16)
	case 6:
		if v == 0 {
			c.Interrupt(0)
			return
		}
		a := uint32(c.R[DX])<<16 | uint32(c.R[AX])
		q, r := a/v, a%v
		if q > 0xFFFF {
			c.Interrupt(0)
			return
		}
		c.R[AX], c.R[DX] = uint16(q), uint16(r)
	case 7:
		if v == 0 {
			c.Interrupt(0)
			return
		}
		a := int32(uint32(c.R[DX])<<16 | uint32(c.R[AX]))
		d := int32(int16(v))
		if a == -2147483648 && d == -1 {
			c.Interrupt(0)
			return
		}
		q, r := a/d, a%d
		if c.rep != 0 {
			q = -q
		}
		if q > 32767 || q < -32767 {
			c.Interrupt(0)
			return
		}
		c.R[AX], c.R[DX] = uint16(q), uint16(r)
	}
}

func (c *CPU) group4() {
	start := c.IP
	m := c.decodeModRM()
	switch m.reg {
	case 0:
		c.setRM8(&m, byte(c.inc(uint32(c.rm8(&m)), w8)))
	case 1:
		c.setRM8(&m, byte(c.dec(uint32(c.rm8(&m)), w8)))
	case 7:
		if c.Read8(CS, start) == TrapOpcode[1] && c.Trap != nil && c.segOvr < 0 {
			c.IP = start + 1
			n := c.fetch16()
			c.trapOpIP = c.opIP
			if err := c.Trap(c, n); err != nil {
				if err == ErrRetry {
					c.IP = c.opIP
					c.Flags |= FlagIF
					c.stop = true
					return
				}
				if c.fault == nil {
					c.faultf("%v", err)
				}
			}
			return
		}
		fallthrough
	default:
		c.faultf("undefined FE /%d", m.reg)
	}
}

func (c *CPU) group5() {
	m := c.decodeModRM()
	switch m.reg {
	case 0:
		c.setRM16(&m, uint16(c.inc(uint32(c.rm16(&m)), w16)))
	case 1:
		c.setRM16(&m, uint16(c.dec(uint32(c.rm16(&m)), w16)))
	case 2:
		t := c.rm16(&m)
		c.push(c.IP)
		c.IP = t
	case 3, 5:
		if m.isReg {
			c.faultf("far CALL/JMP with register operand")
			return
		}
		ip, cs := c.Read16(m.seg, m.off), c.Read16(m.seg, m.off+2)
		if m.reg == 3 {
			c.push(c.S[CS].Sel)
			c.push(c.IP)
		}
		c.IP = ip
		c.SetSeg(CS, cs)
	case 4:
		c.IP = c.rm16(&m)
	case 6, 7:
		if m.isReg && m.rm == SP { // 8086: pushes the decremented SP
			c.R[SP] -= 2
			c.Write16(SS, c.R[SP], c.R[SP])
			return
		}
		v := c.rm16(&m)
		c.push(v)
	}
}

// stringOp executes MOVS/CMPS/STOS/LODS/SCAS/INS/OUTS with an optional REP prefix.
func (c *CPU) stringOp(op byte) {
	word := op&1 == 1
	var step uint16 = 1
	if word {
		step = 2
	}
	if c.Flags&FlagDF != 0 {
		step = -step
	}
	src := c.dataSeg()
	once := func() {
		switch op {
		case 0xA4:
			c.Write8(ES, c.R[DI], c.Read8(src, c.R[SI]))
			c.R[SI] += step
			c.R[DI] += step
		case 0xA5:
			c.Write16(ES, c.R[DI], c.Read16(src, c.R[SI]))
			c.R[SI] += step
			c.R[DI] += step
		case 0xA6:
			c.sub(uint32(c.Read8(src, c.R[SI])), uint32(c.Read8(ES, c.R[DI])), 0, w8)
			c.R[SI] += step
			c.R[DI] += step
		case 0xA7:
			c.sub(uint32(c.Read16(src, c.R[SI])), uint32(c.Read16(ES, c.R[DI])), 0, w16)
			c.R[SI] += step
			c.R[DI] += step
		case 0xAA:
			c.Write8(ES, c.R[DI], c.AL())
			c.R[DI] += step
		case 0xAB:
			c.Write16(ES, c.R[DI], c.R[AX])
			c.R[DI] += step
		case 0xAC:
			c.SetAL(c.Read8(src, c.R[SI]))
			c.R[SI] += step
		case 0xAD:
			c.R[AX] = c.Read16(src, c.R[SI])
			c.R[SI] += step
		case 0xAE:
			c.sub(uint32(c.AL()), uint32(c.Read8(ES, c.R[DI])), 0, w8)
			c.R[DI] += step
		case 0xAF:
			c.sub(uint32(c.R[AX]), uint32(c.Read16(ES, c.R[DI])), 0, w16)
			c.R[DI] += step
		case 0x6C:
			c.Write8(ES, c.R[DI], c.IO.In8(c.R[DX]))
			c.R[DI] += step
		case 0x6D:
			c.Write16(ES, c.R[DI], uint16(c.IO.In8(c.R[DX]))|uint16(c.IO.In8(c.R[DX]+1))<<8)
			c.R[DI] += step
		case 0x6E:
			c.IO.Out8(c.R[DX], c.Read8(src, c.R[SI]))
			c.R[SI] += step
		case 0x6F:
			v := c.Read16(src, c.R[SI])
			c.IO.Out8(c.R[DX], byte(v))
			c.IO.Out8(c.R[DX]+1, byte(v>>8))
			c.R[SI] += step
		}
	}
	if c.rep == 0 {
		once()
		return
	}
	cmp := op == 0xA6 || op == 0xA7 || op == 0xAE || op == 0xAF
	for c.R[CX] != 0 {
		once()
		c.R[CX]--
		if cmp {
			zf := c.Flags&FlagZF != 0
			if (c.rep == 0xF3 && !zf) || (c.rep == 0xF2 && zf) {
				break
			}
		}
	}
}

// daa and das follow the measured 8088 behaviour: the +/-60h step triggers
// above 9Fh instead of 99Fh when AF was set on input (SingleStepTests v2).
func (c *CPU) daa() {
	al, cf, af := c.AL(), c.Flags&FlagCF != 0, c.Flags&FlagAF != 0
	if al&0x0F > 9 || af {
		c.SetAL(al + 6)
		c.Flags |= FlagAF
	} else {
		c.Flags &^= FlagAF
	}
	thr := byte(0x99)
	if af {
		thr = 0x9F
	}
	adj := al > thr || cf
	if adj {
		c.SetAL(c.AL() + 0x60)
	}
	c.SetFlag(FlagCF, adj)
	c.setSZP(uint32(c.AL()), w8)
}

func (c *CPU) das() {
	al, cf, af := c.AL(), c.Flags&FlagCF != 0, c.Flags&FlagAF != 0
	if al&0x0F > 9 || af {
		c.SetAL(al - 6)
		c.Flags |= FlagAF
	} else {
		c.Flags &^= FlagAF
	}
	thr := byte(0x99)
	if af {
		thr = 0x9F
	}
	adj := al > thr || cf
	if adj {
		c.SetAL(c.AL() - 0x60)
	}
	c.SetFlag(FlagCF, adj)
	c.setSZP(uint32(c.AL()), w8)
}

func (c *CPU) aaa() {
	if c.AL()&0x0F > 9 || c.Flags&FlagAF != 0 {
		c.SetAL(c.AL() + 6)
		c.SetAH(c.AH() + 1)
		c.Flags |= FlagAF | FlagCF
	} else {
		c.Flags &^= FlagAF | FlagCF
	}
	c.SetAL(c.AL() & 0x0F)
}

func (c *CPU) aas() {
	if c.AL()&0x0F > 9 || c.Flags&FlagAF != 0 {
		c.SetAL(c.AL() - 6)
		c.SetAH(c.AH() - 1)
		c.Flags |= FlagAF | FlagCF
	} else {
		c.Flags &^= FlagAF | FlagCF
	}
	c.SetAL(c.AL() & 0x0F)
}
