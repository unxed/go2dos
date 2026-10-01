package machine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// DOS gives the parent back its registers when an EXEC'ed child ends
// (MS-DOS 4.0 DISP.ASM restore_world, called from CTRLC.ASM reset_return).
// VC 4.99.09 relies on DS after EXEC of VC.OVL (docs/DOUBTS.md). The parent
// sets CX, SI, DI, BP, runs a child that overwrites all of them and DS, ES,
// and checks that they, DS and ES are back. Exit code 0 means they are.
func TestExecRestoresRegisters(t *testing.T) {
	const (
		nameOff = 0x0200 // "C:\CHILD.COM"
		epbOff  = 0x0220 // parameter block (14 bytes)
		tailOff = 0x0230 // command tail: length 0, CR
		fcbOff  = 0x0240 // zeroed area for both FCB pointers
	)
	var code []byte
	emit := func(b ...byte) { code = append(code, b...) }
	w := func(v int) (byte, byte) { return byte(v), byte(v >> 8) }
	var jumps []int // positions of rel8 operands that must reach "bad"
	jne := func() { emit(0x75, 0); jumps = append(jumps, len(code)-1) }

	emit(0xBB, 0x00, 0x10) // mov bx,1000h
	emit(0xB4, 0x4A)       // mov ah,4Ah
	emit(0xCD, 0x21)       // int 21h (shrink the block so that EXEC has memory)
	for _, o := range []int{epbOff + 4, epbOff + 8, epbOff + 12} {
		lo, hi := w(o)
		emit(0x8C, 0x0E, lo, hi) // mov [o],cs
	}
	movw := func(addr, imm int) { // mov word [addr],imm
		al, ah := w(addr)
		il, ih := w(imm)
		emit(0xC7, 0x06, al, ah, il, ih)
	}
	movw(epbOff+2, tailOff)
	movw(epbOff+6, fcbOff)
	movw(epbOff+10, fcbOff)
	lo, hi := w(nameOff)
	emit(0xBA, lo, hi) // mov dx,name
	lo, hi = w(epbOff)
	emit(0xBB, lo, hi)     // mov bx,epb
	emit(0xB9, 0x11, 0x22) // mov cx,2211h
	emit(0xBE, 0x55, 0xAA) // mov si,0AA55h
	emit(0xBF, 0x66, 0xBB) // mov di,0BB66h
	emit(0xBD, 0x77, 0xCC) // mov bp,0CC77h
	emit(0xB8, 0x00, 0x4B) // mov ax,4B00h
	emit(0xCD, 0x21)       // int 21h
	emit(0x72, 0)          // jc bad
	jumps = append(jumps, len(code)-1)
	emit(0x8C, 0xD8) // mov ax,ds
	emit(0x8C, 0xCB) // mov bx,cs
	emit(0x39, 0xD8) // cmp ax,bx
	jne()
	emit(0x8C, 0xC0) // mov ax,es
	emit(0x39, 0xD8) // cmp ax,bx
	jne()
	emit(0x81, 0xF9, 0x11, 0x22) // cmp cx,2211h
	jne()
	emit(0x81, 0xFE, 0x55, 0xAA) // cmp si,0AA55h
	jne()
	emit(0x81, 0xFF, 0x66, 0xBB) // cmp di,0BB66h
	jne()
	emit(0x81, 0xFD, 0x77, 0xCC) // cmp bp,0CC77h
	jne()
	emit(0xB8, 0x00, 0x4C) // mov ax,4C00h
	emit(0xCD, 0x21)
	bad := len(code)
	emit(0xB8, 0x01, 0x4C) // bad: mov ax,4C01h
	emit(0xCD, 0x21)
	for _, j := range jumps {
		code[j] = byte(bad - (j + 1))
	}
	parent := make([]byte, fcbOff+32-0x100)
	copy(parent, code)
	copy(parent[nameOff-0x100:], `C:\CHILD.COM`)
	parent[tailOff-0x100+1] = 0x0D

	child := []byte{
		0xBB, 0x34, 0x12, // mov bx,1234h
		0xB8, 0x00, 0xB8, // mov ax,0B800h
		0x8E, 0xD8, // mov ds,ax
		0x8E, 0xC0, // mov es,ax
		0xB9, 0x00, 0x00, // mov cx,0
		0xBE, 0x01, 0x00, // mov si,1
		0xBF, 0x02, 0x00, // mov di,2
		0xBD, 0x03, 0x00, // mov bp,3
		0xB8, 0x00, 0x4C, // mov ax,4C00h
		0xCD, 0x21, // int 21h
	}

	dir := t.TempDir()
	for name, b := range map[string][]byte{"PARENT.COM": parent, "CHILD.COM": child} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Load(`C:\PARENT.COM`, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := exitCode(t, m.Run(ctx)); c != 0 {
		t.Errorf("parent exit code %d: registers were not restored after EXEC", c)
	}
}
