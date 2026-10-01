package cpu

// Runs the SingleStepTests 8088 v2 suite (https://github.com/SingleStepTests/8088)
// when GO2DOS_SST_DIR points at a directory with metadata.json and the
// *.json.gz test files; tools/fetch-sst.sh downloads them. Without the
// variable the test is skipped.

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type flatMem [1 << 20]byte

func (m *flatMem) Read8(a uint32) byte     { return m[a&0xFFFFF] }
func (m *flatMem) Write8(a uint32, v byte) { m[a&0xFFFFF] = v }
func (m *flatMem) word(a uint32) uint16    { return uint16(m[a]) | uint16(m[a+1])<<8 }

type nullPorts struct{}

func (nullPorts) In8(uint16) byte   { return 0xFF }
func (nullPorts) Out8(uint16, byte) {}

type sstState struct {
	Regs map[string]int `json:"regs"`
	RAM  [][2]int       `json:"ram"`
}

type sstTest struct {
	Name    string   `json:"name"`
	Bytes   []int    `json:"bytes"`
	Initial sstState `json:"initial"`
	Final   sstState `json:"final"`
}

type sstEntry struct {
	Status    string              `json:"status"`
	FlagsMask *int                `json:"flags-mask"`
	Reg       map[string]sstEntry `json:"reg"`
}

var regNames = map[string]int{"ax": AX, "bx": BX, "cx": CX, "dx": DX, "sp": SP, "bp": BP, "si": SI, "di": DI}
var segNames = map[string]int{"cs": CS, "ss": SS, "ds": DS, "es": ES}

// skipped lists opcodes deliberately not matching the 8088: 60-6F, C0, C1,
// C8 and C9 are 80186 instructions here instead of 8088 aliases.
func skipped(op string) bool {
	n, _ := strconv.ParseUint(op[:2], 16, 8)
	return (n >= 0x60 && n <= 0x6F) || n == 0xC0 || n == 0xC1 || n == 0xC8 || n == 0xC9
}

func TestSingleStep8088(t *testing.T) {
	dir := os.Getenv("GO2DOS_SST_DIR")
	if dir == "" {
		t.Skip("GO2DOS_SST_DIR not set")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Opcodes map[string]sstEntry `json:"opcodes"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json.gz"))
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no test files")
	}
	limit := 0
	if s := os.Getenv("GO2DOS_SST_LIMIT"); s != "" {
		limit, _ = strconv.Atoi(s)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json.gz")
		if skipped(name) {
			continue
		}
		e, ok := meta.Opcodes[strings.ToUpper(name[:2])]
		if !ok {
			continue
		}
		if len(name) > 2 {
			e = e.Reg[name[3:]]
		}
		switch e.Status {
		case "normal", "undocumented", "alias":
		default:
			continue
		}
		mask := 0xFFFF
		if e.FlagsMask != nil {
			mask = *e.FlagsMask
		}
		t.Run(name, func(t *testing.T) { runSSTFile(t, f, uint16(mask), limit) })
	}
}

func runSSTFile(t *testing.T, path string, mask uint16, limit int) {
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	gz, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	var tests []sstTest
	if err := json.NewDecoder(gz).Decode(&tests); err != nil {
		t.Fatal(err)
	}
	if limit > 0 && len(tests) > limit {
		tests = tests[:limit]
	}
	mem := new(flatMem)
	fails := 0
	for i := range tests {
		if msg := runSST(mem, &tests[i], mask); msg != "" {
			fails++
			if fails <= 3 {
				t.Errorf("#%d %s [% X]: %s", i, tests[i].Name, tests[i].Bytes, msg)
			}
		}
	}
	if fails > 3 {
		t.Errorf("%d of %d tests failed", fails, len(tests))
	}
}

func runSST(mem *flatMem, tc *sstTest, mask uint16) string {
	for _, r := range tc.Initial.RAM {
		mem[r[0]] = byte(r[1])
	}
	c := New(mem, nullPorts{})
	for k, v := range tc.Initial.Regs {
		switch {
		case k == "ip":
			c.IP = uint16(v)
		case k == "flags":
			c.SetFlags(uint16(v))
		case segNames[k] != 0 || k == "es":
			c.SetSeg(segNames[k], uint16(v))
		default:
			c.R[regNames[k]] = uint16(v)
		}
	}
	c.Step()
	if c.fault != nil {
		return c.fault.Error()
	}
	var errs []string
	want := func(k string) int {
		if v, ok := tc.Final.Regs[k]; ok {
			return v
		}
		return tc.Initial.Regs[k]
	}
	for k, idx := range regNames {
		if got, w := c.R[idx], uint16(want(k)); got != w {
			errs = append(errs, fmt.Sprintf("%s=%04X want %04X", k, got, w))
		}
	}
	for k, idx := range segNames {
		if got, w := c.S[idx].Sel, uint16(want(k)); got != w {
			errs = append(errs, fmt.Sprintf("%s=%04X want %04X", k, got, w))
		}
	}
	if got, w := c.IP, uint16(want("ip")); got != w {
		errs = append(errs, fmt.Sprintf("ip=%04X want %04X", got, w))
	}
	if got, w := c.Flags&mask, uint16(want("flags"))&mask; got != w {
		errs = append(errs, fmt.Sprintf("flags=%04X want %04X (diff %04X)", got, w, got^w))
	}
	// After a divide error the 8088 pushes FLAGS left in an undefined state
	// by the aborted division; compare that stack word under the flags mask.
	flagsAt := -1
	if c.IP == mem.word(0) && c.S[CS].Sel == mem.word(2) && want("ip") == int(c.IP) {
		flagsAt = int(c.S[SS].Base+uint32(c.R[SP]+4)) & 0xFFFFF
	}
	for _, r := range tc.Final.RAM {
		got := mem[r[0]]
		if r[0] == flagsAt {
			got, r[1] = got&byte(mask), r[1]&int(byte(mask))
		} else if r[0] == flagsAt+1 {
			got, r[1] = got&byte(mask>>8), r[1]&int(byte(mask>>8))
		}
		if got != byte(r[1]) {
			errs = append(errs, fmt.Sprintf("[%05X]=%02X want %02X", r[0], got, r[1]))
		}
	}
	return strings.Join(errs, " ")
}
