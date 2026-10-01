// Command oracle runs procedures from a real VC.COM 4.05 in the go2dos CPU and
// records input/output vectors for the WAT translation (vc-wasm/src).
//
// The procedures are located by byte signatures (the build is not required to
// carry a map); every match must be unique, and the two CALLs inside HexByt
// must target HexCod, otherwise the tool stops (fail fast).
//
//	sh tools/fetch-vc.sh .cache/vc
//	go run ./vc-wasm/oracle -com .cache/vc/4.05/VC.COM -out vc-wasm/oracle/vectors.json
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"

	"github.com/unxed/go2dos/cpu"
)

type flat [1 << 20]byte

func (m *flat) Read8(a uint32) byte     { return m[a&0xFFFFF] }
func (m *flat) Write8(a uint32, v byte) { m[a&0xFFFFF] = v }

type noPorts struct{}

func (noPorts) In8(uint16) byte   { return 0xFF }
func (noPorts) Out8(uint16, byte) {}

const (
	segCode = 0x1000 // PSP segment; a .COM image starts at offset 0x100
	retAddr = 0xFF00 // sentinel return address inside the code segment
	segScr  = 0xB800 // text screen
	segStr  = 0x3000 // segment of the TxtNum input string
	siStart = 0x20   // offset of the string in segStr
	scrFill = 0x07   // fill of the screen window, shows untouched bytes
)

// HexVec is one HexCod/HexByt run. Diff lists the bytes of the 64 KiB screen
// window (ES:0000-FFFF) that differ from the fill value after the run.
type HexVec struct {
	AL    int      `json:"al"`
	DI    int      `json:"di"`
	OutAL int      `json:"out_al"`
	OutDI int      `json:"out_di"`
	Diff  [][2]int `json:"diff"`
}

// TxtVec is one TxtNum run. Text is hex; it is placed at ES:SI with zeros around.
type TxtVec struct {
	Text  string `json:"text"`
	AX    int    `json:"ax"`
	OutSI int    `json:"out_si"`
	CF    int    `json:"cf"`
}

type file struct {
	Source struct {
		File   string `json:"file"`
		SHA256 string `json:"sha256"`
		Size   int    `json:"size"`
	} `json:"source"`
	Offsets   map[string]int `json:"offsets"` // procedure offsets in the code segment
	ScreenFil int            `json:"screen_fill"`
	TxtNumSI  int            `json:"txtnum_si"`
	Hexcod    []HexVec       `json:"hexcod"`
	Hexbyt    []HexVec       `json:"hexbyt"`
	Txtnum    []TxtVec       `json:"txtnum"`
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "oracle: "+format+"\n", a...)
	os.Exit(1)
}

// find returns the unique offset (in the code segment) of pat; -1 is a wildcard.
func find(com []byte, name string, pat []int) int {
	hit := -1
	for i := 0; i+len(pat) <= len(com); i++ {
		ok := true
		for j, b := range pat {
			if b >= 0 && com[i+j] != byte(b) {
				ok = false
				break
			}
		}
		if ok {
			if hit >= 0 {
				die("%s: signature is not unique (offsets %#x and %#x)", name, hit+0x100, i+0x100)
			}
			hit = i
		}
	}
	if hit < 0 {
		die("%s: signature not found", name)
	}
	return hit + 0x100
}

type env struct {
	mem *flat
	com []byte
}

// call runs the procedure at entry until it returns to the sentinel.
func (e *env) call(entry int, setup func(c *cpu.CPU)) *cpu.CPU {
	c := cpu.New(e.mem, noPorts{})
	for _, s := range []int{cpu.ES, cpu.CS, cpu.SS, cpu.DS} {
		c.SetSeg(s, segCode)
	}
	c.SetFlags(0x0202) // IF=1, DF=0
	c.R[cpu.SP] = 0xFFF0
	c.Push(retAddr)
	c.IP = uint16(entry)
	setup(c)
	for i := 0; i < 1_000_000; i++ {
		if c.IP == retAddr && c.S[cpu.CS].Sel == segCode {
			return c
		}
		c.Step()
	}
	die("procedure at %#x did not return", entry)
	return nil
}

func checkSame(name string, before, after [8]uint16, regs ...int) {
	for _, r := range regs {
		if before[r] != after[r] {
			die("%s: register %d changed (%#x -> %#x); the WAT contract (D2) assumes it is preserved", name, r, before[r], after[r])
		}
	}
}

func main() {
	comPath := flag.String("com", ".cache/vc/4.05/VC.COM", "path to VC.COM 4.05")
	outPath := flag.String("out", "vc-wasm/oracle/vectors.json", "output JSON")
	flag.Parse()

	com, err := os.ReadFile(*comPath)
	if err != nil {
		die("%v", err)
	}
	e := &env{mem: new(flat), com: com}
	copy(e.mem[segCode<<4+0x100:], com)

	hexcod := find(com, "HexCod", []int{0x24, 0x0F, 0x04, 0x30, 0x3C, 0x39, 0x76, 0x02, 0x04, 0x07, 0xAA, 0x47, 0xC3})
	hexbyt := find(com, "HexByt", []int{0x51, 0x50, 0xB1, 0x04, 0xD2, 0xE8, 0xE8, -1, -1, 0x58, 0xE8, -1, -1, 0x59, 0xC3})
	txtnum := find(com, "TxtNum", []int{0x52, 0x53, 0xFC, 0x56, -1, -1, 0x26, 0xAC, 0x2C, 0x30, 0x72, -1, 0x3C, 0x09, 0x77, -1, 0x50, 0xB8, 0x0A, 0x00, 0xF7, 0xE3})
	for _, at := range []int{hexbyt + 6, hexbyt + 10} {
		rel := int(int16(uint16(com[at-0x100+1]) | uint16(com[at-0x100+2])<<8))
		if target := (at + 3 + rel) & 0xFFFF; target != hexcod {
			die("HexByt: CALL at %#x targets %#x, expected HexCod %#x", at, target, hexcod)
		}
	}

	var f file
	f.Source.File = "VC.COM 4.05"
	sum := sha256.Sum256(com)
	f.Source.SHA256 = hex.EncodeToString(sum[:])
	f.Source.Size = len(com)
	f.Offsets = map[string]int{"hexcod": hexcod, "hexbyt": hexbyt, "txtnum": txtnum}
	f.ScreenFil = scrFill
	f.TxtNumSI = siStart

	scr := e.mem[segScr<<4 : segScr<<4+0x10000]
	runHex := func(entry int, name string, al, di int) HexVec {
		for i := range scr {
			scr[i] = scrFill
		}
		var before [8]uint16
		c := e.call(entry, func(c *cpu.CPU) {
			c.SetSeg(cpu.ES, segScr)
			c.R[cpu.AX] = 0xA500 | uint16(al) // AH is a canary: both procedures keep it
			c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX], c.R[cpu.SI] = 0x1111, 0x2222, 0x3333, 0x4444
			c.R[cpu.DI] = uint16(di)
			before = c.R
		})
		checkSame(name, before, c.R, cpu.BX, cpu.CX, cpu.DX, cpu.SI)
		if c.R[cpu.AX]>>8 != 0xA5 {
			die("%s: AH changed", name)
		}
		v := HexVec{AL: al, DI: di, OutAL: int(c.R[cpu.AX] & 0xFF), OutDI: int(c.R[cpu.DI]), Diff: [][2]int{}}
		for i, b := range scr {
			if b != scrFill {
				v.Diff = append(v.Diff, [2]int{i, int(b)})
			}
		}
		return v
	}
	for al := 0; al < 256; al++ {
		for _, di := range []int{0, 0xFFFF} {
			f.Hexcod = append(f.Hexcod, runHex(hexcod, "HexCod", al, di))
		}
		for _, di := range []int{0, 0x0A, 0xFFFE, 0xFFFF} {
			f.Hexbyt = append(f.Hexbyt, runHex(hexbyt, "HexByt", al, di))
		}
	}

	// TxtNum corpus: all strings up to 3 characters over a mixed alphabet,
	// numbers around the 16-bit limits, and seeded random strings.
	alpha := []byte{'0', '1', '5', '9', 'a', '/', ':', 0xFF, 0x00, ' '}
	var texts [][]byte
	var gen func(prefix []byte, left int)
	gen = func(prefix []byte, left int) {
		texts = append(texts, append([]byte(nil), prefix...))
		if left == 0 {
			return
		}
		for _, b := range alpha {
			gen(append(prefix, b), left-1)
		}
	}
	gen(nil, 3)
	for _, n := range []int{0, 1, 9, 10, 99, 100, 255, 256, 999, 1000, 6553, 6554, 9999, 10000, 32767, 32768,
		65534, 65535, 65536, 65539, 65540, 99999, 100000, 655350, 655359, 655360, 6553599} {
		for _, tail := range []string{"", " ", "x", "\x00"} {
			texts = append(texts, []byte(fmt.Sprintf("%d%s", n, tail)))
		}
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		t := make([]byte, 1+rng.Intn(10))
		for j := range t {
			if rng.Intn(5) > 0 {
				t[j] = byte('0' + rng.Intn(10))
			} else {
				t[j] = alpha[rng.Intn(len(alpha))]
			}
		}
		texts = append(texts, t)
	}

	str := e.mem[segStr<<4 : segStr<<4+0x100]
	for _, t := range texts {
		for i := range str {
			str[i] = 0
		}
		copy(str[siStart:], t)
		var before [8]uint16
		c := e.call(txtnum, func(c *cpu.CPU) {
			c.SetSeg(cpu.ES, segStr)
			c.R[cpu.AX] = 0xBEEF
			c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX], c.R[cpu.DI] = 0x1111, 0x2222, 0x3333, 0x4444
			c.R[cpu.SI] = siStart
			before = c.R
		})
		checkSame("TxtNum", before, c.R, cpu.BX, cpu.CX, cpu.DX, cpu.DI)
		want := make([]byte, len(str))
		copy(want[siStart:], t)
		if !bytes.Equal(str, want) {
			die("TxtNum wrote to the string %x", t)
		}
		cf := 0
		if c.Flag(cpu.FlagCF) {
			cf = 1
		}
		f.Txtnum = append(f.Txtnum, TxtVec{Text: hex.EncodeToString(t), AX: int(c.R[cpu.AX]), OutSI: int(c.R[cpu.SI]), CF: cf})
	}

	if !bytes.Equal(e.mem[segCode<<4+0x100:segCode<<4+0x100+len(com)], com) {
		die("the code image was modified while running")
	}
	data, err := json.Marshal(&f)
	if err != nil {
		die("%v", err)
	}
	if err := os.WriteFile(*outPath, append(data, '\n'), 0o644); err != nil {
		die("%v", err)
	}
	fmt.Printf("oracle: HexCod@%#x HexByt@%#x TxtNum@%#x; %d+%d+%d vectors -> %s (%d bytes)\n",
		hexcod, hexbyt, txtnum, len(f.Hexcod), len(f.Hexbyt), len(f.Txtnum), *outPath, len(data))
}
