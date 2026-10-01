// Package machine assembles the CPU, memory, devices, BIOS and DOS into a
// runnable PC and exposes the API for embedding applications: run a
// program, feed keys, take text-screen snapshots, write diagnostic dumps.
package machine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Config configures a machine.
type Config struct {
	// Drives maps drive letters to host directories; at least one is needed.
	Drives map[byte]string
	// Drive is the current drive letter (default C).
	Drive byte
	// Codepage is the OEM code page; 0 picks it from the host locale.
	Codepage int
	// Env is the DOS environment (NAME=VALUE).
	Env []string
	// Now returns the wall-clock time (default time.Now).
	Now func() time.Time
	// TraceLog receives every HLE call as a JSON line; nil disables it.
	TraceLog io.Writer
	// TraceFilter limits TraceLog to calls whose name starts with one of
	// these prefixes ("int21", "int10", ...). Empty means all.
	TraceFilter []string
	// IdlePolls is how many empty keyboard polls in a row count as idling.
	IdlePolls int
	// OnScreen is called from the machine goroutine whenever the text
	// screen changes (at most every FrameInterval).
	OnScreen func(*bios.Screen)
	// FrameInterval limits OnScreen calls (default 15ms).
	FrameInterval time.Duration
	// Watch lists linear addresses whose writes are logged to the trace
	// together with the writing instruction (diagnostics).
	Watch []uint32
	// ExecTrace, if > 0, logs the next ExecTrace executed instructions
	// (address, bytes, registers) to the trace after every EXEC start and
	// return (diagnostics; call name "exec"). It is single-stepped, so it
	// is slow, and is also set by the GO2DOS_EXEC_TRACE environment variable.
	ExecTrace int
}

// Machine is an emulated PC running a DOS program.
type Machine struct {
	cfg  Config
	Mem  *mem.Memory
	CPU  *cpu.CPU
	Env  *hle.Env
	BIOS *bios.BIOS
	DOS  *dos.DOS
	CP   *cp.Codepage
	// CodepageInfo tells where the code page came from.
	CodepageInfo cp.Detection

	pic pic
	pit pit

	keys     chan bios.KeyEvent
	recMu    sync.Mutex
	recorded []recordedKey
	start    time.Time

	execLeft   int // instructions still to log (Config.ExecTrace)
	nextTick   time.Time
	screen     atomic.Pointer[bios.Screen]
	lastVer    uint32
	lastFrame  time.Time
	program    string
	exitCode   int
	running    atomic.Bool
	unknownIO  map[uint16]bool
	idleWanted bool
}

type recordedKey struct {
	at  time.Duration
	key bios.KeyEvent
}

// TickPeriod is the PIT channel 0 period at the default divisor.
const TickPeriod = 65536 * time.Second / 1193182

// New creates a machine.
func New(cfg Config) (*Machine, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.IdlePolls == 0 {
		cfg.IdlePolls = 50
	}
	if cfg.FrameInterval == 0 {
		cfg.FrameInterval = 15 * time.Millisecond
	}
	if cfg.Drive == 0 {
		cfg.Drive = 'C'
	}
	det, err := cp.Detect(cfg.Codepage)
	if err != nil {
		return nil, err
	}
	page, err := cp.Get(det.Num)
	if err != nil {
		return nil, err
	}
	if v, err := strconv.Atoi(os.Getenv("GO2DOS_EXEC_TRACE")); err == nil && cfg.ExecTrace == 0 {
		cfg.ExecTrace = v
	}
	if spec := os.Getenv("GO2DOS_WATCH"); spec != "" {
		ws, err := ParseWatch(spec)
		if err != nil {
			return nil, err
		}
		cfg.Watch = append(cfg.Watch, ws...)
	}
	m := &Machine{cfg: cfg, CP: page, CodepageInfo: det, keys: make(chan bios.KeyEvent, 256),
		unknownIO: map[uint16]bool{}, start: time.Now()}
	m.Mem = mem.New()
	m.CPU = cpu.New(m.Mem, m)
	if len(cfg.Watch) > 0 {
		m.Mem.Watched = map[uint32]bool{}
		for _, a := range cfg.Watch {
			m.Mem.Watched[a] = true
		}
		m.Mem.Watch = func(a uint32, old, v byte) {
			at := m.CPU.History[(m.CPUHistoryPos()+cpu.HistoryLen-1)%cpu.HistoryLen]
			m.Env.Trace.Port(fmt.Sprintf("watch %05X: %02X -> %02X by instruction at %04X:%04X", a, old, v, at>>16, at&0xFFFF))
		}
	}
	tr := hle.NewTracer(256, cfg.TraceLog, cfg.TraceFilter)
	m.Env = hle.New(m.CPU, m.Mem, page, cfg.Now, tr)
	m.Env.Idle = m.idle
	if cfg.ExecTrace > 0 {
		m.Env.Event = func(string) {
			m.execLeft = cfg.ExecTrace
			m.CPU.RequestStop() // leave the slice so single-stepping starts at once
		}
	}
	m.BIOS = bios.New(m.Env)
	m.BIOS.IdlePolls = cfg.IdlePolls
	m.BIOS.IRQ1 = func() { m.pic.raise(1); m.CPU.IntrPending = true }
	env := cfg.Env
	if env == nil {
		env = []string{`COMSPEC=C:\COMMAND.COM`, `PATH=C:\`, `PROMPT=$P$G`}
	}
	m.DOS, err = dos.New(m.Env, m.BIOS, dos.Config{Drives: cfg.Drives, Current: cfg.Drive, Env: env})
	if err != nil {
		return nil, err
	}
	m.pic.imr = 0
	m.CPU.Intr = m.pic.ack
	return m, nil
}

// Load loads a program (a DOS path such as C:\VC.COM) with a command tail.
func (m *Machine) Load(path, tail string) error {
	if _, err := m.DOS.Load(path, tail); err != nil {
		return err
	}
	m.program = path
	return nil
}

// PushKey queues a keystroke; safe to call from any goroutine.
func (m *Machine) PushKey(k bios.KeyEvent) {
	m.recMu.Lock()
	m.recorded = append(m.recorded, recordedKey{time.Since(m.start), k})
	m.recMu.Unlock()
	m.keys <- k
}

// Screen returns the latest text-screen snapshot; safe from any goroutine.
func (m *Machine) Screen() *bios.Screen {
	if s := m.screen.Load(); s != nil {
		return s
	}
	return &bios.Screen{}
}

// ExitError is returned by Run when the program terminates normally.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit code %d", e.Code) }

// FaultError is returned by Run when the machine stops on an unsupported
// instruction or service. Diagnostics are available through Dump.
type FaultError struct {
	Err error
	At  string // CS:IP
}

func (e *FaultError) Error() string { return fmt.Sprintf("stopped at %s: %v", e.At, e.Err) }
func (e *FaultError) Unwrap() error { return e.Err }

// Run executes the loaded program until it exits, faults, or ctx ends.
func (m *Machine) Run(ctx context.Context) error {
	m.running.Store(true)
	defer m.running.Store(false)
	m.nextTick = time.Now().Add(TickPeriod)
	m.publishScreen(true)
	const slice = 20000
	for {
		if err := ctx.Err(); err != nil {
			m.publishScreen(true)
			return err
		}
		m.pollHost()
		m.idleWanted = false
		budget := slice
		if m.execLeft > 0 {
			budget = 1
		}
		before := m.CPU.Executed
		reason := m.CPU.Run(budget)
		if m.execLeft > 0 && m.CPU.Executed != before {
			m.logExec()
		}
		if stop := m.Env.Stop; stop != nil {
			m.Env.Stop = nil
			m.publishScreen(true)
			var ex *hle.Exit
			if errors.As(stop, &ex) {
				return &ExitError{Code: int(ex.Code)}
			}
			return stop
		}
		switch reason {
		case cpu.StopFault:
			m.publishScreen(true)
			return &FaultError{Err: m.CPU.Err, At: fmt.Sprintf("%04X:%04X", m.CPU.S[cpu.CS].Sel, m.CPU.IP)}
		case cpu.StopHalt:
			if m.CPU.Flags&cpu.FlagIF == 0 {
				m.publishScreen(true)
				return &FaultError{Err: errors.New("HLT with interrupts disabled"),
					At: fmt.Sprintf("%04X:%04X", m.CPU.S[cpu.CS].Sel, m.CPU.IP)}
			}
			m.wait(ctx)
		case cpu.StopTrap:
			if m.idleWanted {
				m.wait(ctx)
			}
		}
		m.publishScreen(false)
	}
}

// pollHost moves host events into the machine: keys and timer ticks.
func (m *Machine) pollHost() {
	for {
		select {
		case k := <-m.keys:
			m.BIOS.PushKey(k)
			continue
		default:
		}
		break
	}
	m.BIOS.PumpKeyboard()
	now := time.Now()
	if !now.Before(m.nextTick) {
		m.pic.raise(0)
		m.CPU.IntrPending = true
		m.nextTick = m.nextTick.Add(TickPeriod)
		if now.Sub(m.nextTick) > 5*TickPeriod {
			m.nextTick = now.Add(TickPeriod)
		}
	}
	if m.pic.irr != 0 {
		m.CPU.IntrPending = true
	}
}

// idle is called by HLE handlers when the guest waits for input.
func (m *Machine) idle() {
	m.idleWanted = true
	m.CPU.RequestStop()
}

// wait sleeps until a key arrives or the next timer tick is due.
func (m *Machine) wait(ctx context.Context) {
	m.publishScreen(true)
	if m.BIOS.KeysPending() || m.pic.irr != 0 {
		return
	}
	d := time.Until(m.nextTick)
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case k := <-m.keys:
		m.BIOS.PushKey(k)
	case <-t.C:
	case <-ctx.Done():
	}
}

func (m *Machine) publishScreen(force bool) {
	v := m.BIOS.Video
	now := time.Now()
	if !force && now.Sub(m.lastFrame) < m.cfg.FrameInterval {
		return
	}
	s := v.Snapshot()
	if s.Version == m.lastVer && m.screen.Load() != nil {
		return
	}
	m.lastVer = s.Version
	m.lastFrame = now
	m.screen.Store(s)
	if m.cfg.OnScreen != nil {
		m.cfg.OnScreen(s)
	}
}

// --- I/O ports ----------------------------------------------------------------

// In8 implements cpu.Ports.
func (m *Machine) In8(port uint16) byte {
	if v, ok := m.BIOS.Video.In(port); ok {
		return v
	}
	if v, ok := m.BIOS.In(port); ok {
		return v
	}
	switch port {
	case 0x20:
		return m.pic.irr
	case 0x21:
		return m.pic.imr
	case 0x40, 0x41, 0x42:
		return m.pit.read(int(port-0x40), time.Since(m.start))
	case 0x61:
		m.pit.port61 ^= 0x10 // refresh toggle used by delay loops
		return m.pit.port61
	case 0x92:
		if m.Mem.A20() {
			return 2
		}
		return 0
	}
	m.unknownPort(port, "in")
	return 0xFF
}

// Out8 implements cpu.Ports.
func (m *Machine) Out8(port uint16, v byte) {
	if m.BIOS.Video.Out(port, v) {
		return
	}
	switch port {
	case 0x20:
		if v&0x20 != 0 {
			m.pic.eoi()
		}
		return
	case 0x21:
		m.pic.imr = v
		return
	case 0x40, 0x41, 0x42:
		m.pit.write(int(port-0x40), v)
		return
	case 0x43:
		m.pit.control(v, time.Since(m.start))
		return
	case 0x61:
		m.pit.port61 = v
		return
	case 0x92:
		m.Mem.SetA20(v&2 != 0)
		return
	case 0x80: // POST code / delay port
		return
	}
	m.unknownPort(port, "out")
}

func (m *Machine) unknownPort(port uint16, dir string) {
	if !m.unknownIO[port] {
		m.unknownIO[port] = true
		m.Env.Trace.Port(fmt.Sprintf("%s %04Xh: no device (reads FFh, writes ignored)", dir, port))
	}
}

// --- 8259 PIC (master only) ---------------------------------------------------------

type pic struct {
	irr, isr, imr byte
}

func (p *pic) raise(irq int) { p.irr |= 1 << irq }

func (p *pic) ack() (byte, bool) {
	pending := p.irr &^ p.imr
	for i := 0; i < 8; i++ {
		bit := byte(1) << i
		if p.isr&bit != 0 {
			return 0, false // a higher-priority interrupt is in service
		}
		if pending&bit != 0 {
			p.irr &^= bit
			p.isr |= bit
			return byte(8 + i), true
		}
	}
	return 0, false
}

func (p *pic) eoi() {
	for i := 0; i < 8; i++ {
		if p.isr&(1<<i) != 0 {
			p.isr &^= 1 << i
			return
		}
	}
}

// --- 8253 PIT (reads only) ----------------------------------------------------------

type pit struct {
	latched [3]bool
	latch   [3]uint16
	hiByte  [3]bool
	port61  byte
}

func counterAt(d time.Duration) uint16 {
	ticks := uint64(d.Nanoseconds()) * 1193182 / 1000000000
	return uint16(0xFFFF - ticks%0x10000)
}

func (p *pit) control(v byte, d time.Duration) {
	ch := int(v >> 6)
	if ch > 2 {
		return
	}
	if v&0x30 == 0 { // latch command
		p.latch[ch] = counterAt(d)
		p.latched[ch] = true
		p.hiByte[ch] = false
	}
}

func (p *pit) read(ch int, d time.Duration) byte {
	v := counterAt(d)
	if p.latched[ch] {
		v = p.latch[ch]
	}
	if p.hiByte[ch] {
		p.hiByte[ch] = false
		p.latched[ch] = false
		return byte(v >> 8)
	}
	p.hiByte[ch] = true
	return byte(v)
}

func (p *pit) write(ch int, v byte) {
	// Reprogramming the divisor is accepted; the timer keeps 18.2 Hz.
}

// CPUHistoryPos returns the index of the next history slot.
func (m *Machine) CPUHistoryPos() int { return m.CPU.HistoryPos() }

// logExec writes the instruction just executed to the trace (Config.ExecTrace).
func (m *Machine) logExec() {
	m.execLeft--
	c := m.CPU
	at := c.History[(c.HistoryPos()+cpu.HistoryLen-1)%cpu.HistoryLen]
	cs, ip := uint16(at>>16), uint16(at)
	m.Env.Trace.Stream("exec", fmt.Sprintf("%04X:%04X [% X] AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X DS=%04X ES=%04X SS:SP=%04X:%04X",
		cs, ip, m.Mem.Bytes(mem.Lin(cs, ip), 6), c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX],
		c.R[cpu.SI], c.R[cpu.DI], c.R[cpu.BP], c.S[cpu.DS].Sel, c.S[cpu.ES].Sel, c.S[cpu.SS].Sel, c.R[cpu.SP]))
}

// ParseWatch parses a comma-separated list of watch addresses: a linear hex
// address (22CD) or SEG:OFF (1234:0010), each optionally followed by /N
// (the number of bytes, hex; default 1).
func ParseWatch(spec string) ([]uint32, error) {
	var out []uint32
	for _, w := range strings.Split(spec, ",") {
		w = strings.TrimSpace(strings.ToLower(w))
		if w == "" {
			continue
		}
		n := uint64(1)
		if i := strings.IndexByte(w, '/'); i >= 0 {
			v, err := strconv.ParseUint(w[i+1:], 16, 16)
			if err != nil || v == 0 {
				return nil, fmt.Errorf("bad watch length in %q", w)
			}
			n, w = v, w[:i]
		}
		var a uint32
		if i := strings.IndexByte(w, ':'); i >= 0 {
			seg, err1 := strconv.ParseUint(strings.TrimPrefix(w[:i], "0x"), 16, 16)
			off, err2 := strconv.ParseUint(strings.TrimPrefix(w[i+1:], "0x"), 16, 16)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("bad watch address %q", w)
			}
			a = mem.Lin(uint16(seg), uint16(off))
		} else {
			v, err := strconv.ParseUint(strings.TrimPrefix(w, "0x"), 16, 32)
			if err != nil {
				return nil, fmt.Errorf("bad watch address %q", w)
			}
			a = uint32(v)
		}
		for i := uint64(0); i < n; i++ {
			out = append(out, a+uint32(i))
		}
	}
	return out, nil
}
