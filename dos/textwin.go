package dos

import (
	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// AMIS provider DOS-HOST/TEXTWIN (docs/TEXTWIN.md, version 1.0): the size of
// the text window, notification of size changes and the role of the screen
// (docs/SCREEN.md).
//
//	AL=10h: window size. Out: AL=FFh, CX=columns, DX=rows, BX=most cells.
//	AL=11h: BL=1 tell me of size changes, BL=0 stop. Out: AL=FFh, BL=previous;
//	        AL=00h if BL is neither (nothing changes). When the size changes the
//	        emulator updates the BDA and CRTC and puts the word FF00h
//	        (bios.ResizeKey) in the keyboard buffer.
//	AL=12h: role of the screen, BL=0 console (a stream), BL=1 full-screen
//	        interface. Out: AL=FFh, BL=previous; AL=00h for another BL.

func (d *DOS) installTextWin() {
	d.RegisterAMIS(AMISProvider{
		Manufacturer: "DOS-HOST",
		Product:      "TEXTWIN",
		Description:  "text window size, size events, screen role",
		Version:      0x0100,
		Call:         d.textWinCall,
	})
}

// AMISMux returns the multiplex number of a registered provider.
func (d *DOS) AMISMux(manufacturer, product string) (byte, bool) {
	for mux, ent := range d.amis {
		if pad8(ent.p.Manufacturer) == pad8(manufacturer) && pad8(ent.p.Product) == pad8(product) {
			return mux, true
		}
	}
	return 0, false
}

// WatchingResize reports whether a program asked to be told of size changes.
func (d *DOS) WatchingResize() bool { return d.winWatch }

func (d *DOS) textWinCall(e *hle.Env, fn byte) (bool, error) {
	c := e.CPU
	switch fn {
	case 0x10:
		cols, rows := d.b.Video.Size()
		c.SetAL(0xFF)
		c.R[cpu.CX], c.R[cpu.DX], c.R[cpu.BX] = uint16(cols), uint16(rows), bios.MaxCells
	case 0x11:
		if bl := c.BL(); bl > 1 {
			c.SetAL(0x00)
		} else {
			prev := d.winWatch
			d.winWatch = bl == 1
			c.SetAL(0xFF)
			c.SetBL(b2u(prev))
		}
	case 0x12:
		if bl := c.BL(); bl > 1 {
			c.SetAL(0x00)
		} else {
			prev := d.winGrid
			d.winGrid = bl == 1
			if d.OnRole != nil {
				d.OnRole(d.winGrid)
			}
			c.SetAL(0xFF)
			c.SetBL(b2u(prev))
		}
	default:
		return false, nil
	}
	return true, nil
}

func b2u(b bool) byte {
	if b {
		return 1
	}
	return 0
}
