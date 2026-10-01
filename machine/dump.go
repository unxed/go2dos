package machine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/keys"
)

// Dump writes a diagnostic bundle into a new directory under parent and
// returns its path. The bundle is self-describing: report.txt explains
// every other file.
//
// It must be called when the machine is not running (after Run returned).
func (m *Machine) Dump(parent, reason string) (string, error) {
	if m.running.Load() {
		return "", fmt.Errorf("Dump called while the machine is running")
	}
	dir := filepath.Join(parent, "go2dos-dump-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	m.publishScreen(true)
	files := map[string][]byte{
		"report.txt":  []byte(m.report(reason)),
		"memory.bin":  m.Mem.RAM,
		"screen.txt":  []byte(m.Screen().Text() + "\n"),
		"screen.ans":  []byte(ANSI(m.Screen())),
		"trace.jsonl": m.traceJSON(),
		"keys.txt":    []byte(m.RecordedKeys() + "\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return dir, err
		}
	}
	return dir, nil
}

func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, dirty := "", ""
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	return fmt.Sprintf("%s %s%s (%s, %s/%s)", bi.Main.Path, rev, dirty, bi.GoVersion, runtime.GOOS, runtime.GOARCH)
}

func (m *Machine) report(reason string) string {
	var b strings.Builder
	c := m.CPU
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	w("go2dos diagnostic dump")
	w("======================")
	w("time:     %s", time.Now().Format(time.RFC3339))
	w("build:    %s", version())
	w("program:  %s", m.program)
	w("reason:   %s", reason)
	w("codepage: %d (%s) %s", m.CP.Num, m.CodepageInfo.Source, m.CodepageInfo.Note)
	for l, d := range m.cfg.Drives {
		w("drive %c:  %s", l, d)
	}
	w("executed: %d instructions", c.Executed)
	w("")
	w("registers: %s", c.State.String())
	csip := c.S[cpu.CS].Base + uint32(c.IP)
	w("bytes at CS:IP: % X", m.Mem.Bytes(csip, 16))
	w("stack (SS:SP upwards):")
	for i := 0; i < 16; i++ {
		off := c.R[cpu.SP] + uint16(i*2)
		fmt.Fprintf(&b, " %04X", c.Read16(cpu.SS, off))
	}
	w("")
	w("")
	w("last HLE calls (oldest first):")
	for _, r := range m.Env.Trace.Last() {
		w("  %s", r.String())
	}
	w("")
	w("last executed instructions (CS:IP, oldest first; disassemble memory.bin at these addresses):")
	h := c.HistorySnapshot()
	if len(h) > 128 {
		h = h[len(h)-128:]
	}
	for i, a := range h {
		fmt.Fprintf(&b, " %04X:%04X", a>>16, a&0xFFFF)
		if i%8 == 7 {
			b.WriteString("\n")
		}
	}
	w("")
	w("")
	w("files in this dump:")
	w("  memory.bin   1 MiB + HMA, linear addresses (ndisasm -b16 -o0x<lin> -e0x<lin> memory.bin)")
	w("  screen.txt   text screen in UTF-8")
	w("  screen.ans   the same with colors (cat it in a terminal)")
	w("  trace.jsonl  the last HLE calls as JSON lines")
	w("  keys.txt     keys typed in this session, as a replayable script (-keys @keys.txt)")
	return b.String()
}

func (m *Machine) traceJSON() []byte {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	for _, r := range m.Env.Trace.Last() {
		enc.Encode(r)
	}
	return []byte(b.String())
}

// RecordedKeys returns the keys pushed so far as a key script, with the
// pauses between them.
func (m *Machine) RecordedKeys() string {
	m.recMu.Lock()
	defer m.recMu.Unlock()
	var b strings.Builder
	var last time.Duration
	for _, r := range m.recorded {
		if gap := r.at - last; gap > 300*time.Millisecond {
			fmt.Fprintf(&b, "<wait:%s>", gap.Round(100*time.Millisecond))
		}
		last = r.at
		b.WriteString(keys.Format(r.key, m.CP))
	}
	return b.String()
}

// ANSI renders a screen snapshot with ANSI colors.
func ANSI(s *bios.Screen) string {
	var b strings.Builder
	last := -1
	for y := 0; y < s.Rows; y++ {
		for x := 0; x < s.Cols; x++ {
			c := s.Cells[y*s.Cols+x]
			if int(c.Attr) != last {
				b.WriteString(SGR(c.Attr))
				last = int(c.Attr)
			}
			b.WriteRune(c.Rune)
		}
		b.WriteString("\x1b[0m\n")
		last = -1
	}
	return b.String()
}

// cgaToANSI maps CGA color numbers to ANSI color indexes.
var cgaToANSI = [8]int{0, 4, 2, 6, 1, 5, 3, 7}

// SGR returns the escape sequence selecting a text attribute (blink bit
// shown as bright background, as VGA does with blinking disabled).
func SGR(attr byte) string {
	fg := int(attr & 0x0F)
	bg := int(attr >> 4 & 0x0F)
	f := 30 + cgaToANSI[fg&7]
	if fg >= 8 {
		f = 90 + cgaToANSI[fg&7]
	}
	g := 40 + cgaToANSI[bg&7]
	if bg >= 8 {
		g = 100 + cgaToANSI[bg&7]
	}
	return fmt.Sprintf("\x1b[0;%d;%dm", f, g)
}
