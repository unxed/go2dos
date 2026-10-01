package cpu

var parity [256]bool

func init() {
	for i := range parity {
		n := 0
		for b := i; b != 0; b >>= 1 {
			n += b & 1
		}
		parity[i] = n%2 == 0
	}
}

// width describes an 8- or 16-bit operand.
type width struct {
	mask, sign uint32
	bits       uint
}

var (
	w8  = width{0xFF, 0x80, 8}
	w16 = width{0xFFFF, 0x8000, 16}
)

func (c *CPU) setSZP(r uint32, w width) {
	r &= w.mask
	c.SetFlag(FlagZF, r == 0)
	c.SetFlag(FlagSF, r&w.sign != 0)
	c.SetFlag(FlagPF, parity[byte(r)])
}

func (c *CPU) cf() uint32 {
	if c.Flags&FlagCF != 0 {
		return 1
	}
	return 0
}

func (c *CPU) add(a, b, carry uint32, w width) uint32 {
	r := a + b + carry
	c.SetFlag(FlagCF, r > w.mask)
	c.SetFlag(FlagAF, (a^b^r)&0x10 != 0)
	c.SetFlag(FlagOF, (r^a)&(r^b)&w.sign != 0)
	r &= w.mask
	c.setSZP(r, w)
	return r
}

func (c *CPU) sub(a, b, borrow uint32, w width) uint32 {
	r := (a - b - borrow) & w.mask
	c.SetFlag(FlagCF, a < b+borrow)
	c.SetFlag(FlagAF, (a^b^r)&0x10 != 0)
	c.SetFlag(FlagOF, (a^b)&(a^r)&w.sign != 0)
	c.setSZP(r, w)
	return r
}

func (c *CPU) logic(r uint32, w width) uint32 {
	r &= w.mask
	c.Flags &^= FlagCF | FlagOF | FlagAF
	c.setSZP(r, w)
	return r
}

// alu performs one of ADD OR ADC SBB AND SUB XOR CMP. For CMP the result is
// computed but the caller must not store it.
func (c *CPU) alu(op byte, a, b uint32, w width) uint32 {
	switch op {
	case 0:
		return c.add(a, b, 0, w)
	case 1:
		return c.logic(a|b, w)
	case 2:
		return c.add(a, b, c.cf(), w)
	case 3:
		return c.sub(a, b, c.cf(), w)
	case 4:
		return c.logic(a&b, w)
	case 5, 7:
		return c.sub(a, b, 0, w)
	default: // 6
		return c.logic(a^b, w)
	}
}

func (c *CPU) inc(a uint32, w width) uint32 {
	cf := c.Flags & FlagCF
	r := c.add(a, 1, 0, w)
	c.Flags = c.Flags&^FlagCF | cf
	return r
}

func (c *CPU) dec(a uint32, w width) uint32 {
	cf := c.Flags & FlagCF
	r := c.sub(a, 1, 0, w)
	c.Flags = c.Flags&^FlagCF | cf
	return r
}

// shift implements the D0-D3/C0/C1 group. Counts are not masked (8086).
func (c *CPU) shift(op byte, v uint32, count uint, w width) uint32 {
	if count == 0 {
		return v
	}
	msb := func(x uint32) uint32 { return (x >> (w.bits - 1)) & 1 }
	cf := c.cf()
	for i := uint(0); i < count; i++ {
		switch op {
		case 0: // ROL
			cf = msb(v)
			v = (v<<1 | cf) & w.mask
		case 1: // ROR
			cf = v & 1
			v = v>>1 | cf<<(w.bits-1)
		case 2: // RCL
			n := msb(v)
			v = (v<<1 | cf) & w.mask
			cf = n
		case 3: // RCR
			n := v & 1
			v = v>>1 | cf<<(w.bits-1)
			cf = n
		case 4: // SHL
			cf = msb(v)
			v = (v << 1) & w.mask
		case 5: // SHR
			cf = v & 1
			v >>= 1
		case 6: // SETMO (undocumented on the 8086)
			cf = 0
			v = w.mask
		case 7: // SAR
			cf = v & 1
			v = v>>1 | v&w.sign
		}
	}
	c.SetFlag(FlagCF, cf != 0)
	switch op {
	case 0, 2:
		c.SetFlag(FlagOF, msb(v)^cf != 0)
	case 1, 3:
		c.SetFlag(FlagOF, msb(v)^((v>>(w.bits-2))&1) != 0)
	case 4:
		c.SetFlag(FlagOF, msb(v)^cf != 0)
		c.setSZP(v, w)
		c.Flags &^= FlagAF
	case 5:
		c.SetFlag(FlagOF, msb(v)^((v>>(w.bits-2))&1) != 0)
		c.setSZP(v, w)
		c.Flags &^= FlagAF
	case 6:
		c.Flags &^= FlagOF | FlagAF
		c.setSZP(v, w)
	case 7:
		c.Flags &^= FlagOF | FlagAF
		c.setSZP(v, w)
	}
	return v
}
