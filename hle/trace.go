package hle

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/unxed/go2dos/cpu"
)

// Regs is the subset of registers recorded per call.
type Regs struct {
	AX, BX, CX, DX, SI, DI, BP, DS, ES uint16
}

func regsOf(c *cpu.CPU) Regs {
	return Regs{c.R[cpu.AX], c.R[cpu.BX], c.R[cpu.CX], c.R[cpu.DX], c.R[cpu.SI],
		c.R[cpu.DI], c.R[cpu.BP], c.S[cpu.DS].Sel, c.S[cpu.ES].Sel}
}

func (r Regs) String() string {
	return fmt.Sprintf("AX=%04X BX=%04X CX=%04X DX=%04X SI=%04X DI=%04X BP=%04X DS=%04X ES=%04X",
		r.AX, r.BX, r.CX, r.DX, r.SI, r.DI, r.BP, r.DS, r.ES)
}

// Record is one traced HLE call.
type Record struct {
	Seq    uint64 `json:"seq"`
	Name   string `json:"call"`
	Caller string `json:"caller"`
	In     Regs   `json:"in"`
	Out    Regs   `json:"out"`
	CF     bool   `json:"cf"`
	Note   string `json:"note,omitempty"`
	Err    string `json:"err,omitempty"`
}

func (r *Record) String() string {
	s := fmt.Sprintf("#%d %s @%s in[%s] out[%s] CF=%v", r.Seq, r.Name, r.Caller, r.In, r.Out, r.CF)
	if r.Note != "" {
		s += " " + r.Note
	}
	if r.Err != "" {
		s += " ERROR: " + r.Err
	}
	return s
}

// Tracer keeps the last calls in memory and optionally logs every call
// (or only those matching Filter) as JSON lines.
type Tracer struct {
	mu     sync.Mutex
	ring   []Record
	pos    int
	seq    uint64
	out    io.Writer
	enc    *json.Encoder
	filter []string
}

// NewTracer creates a tracer keeping the last n records. If w is non-nil,
// calls whose name starts with one of filter (all calls if filter is empty)
// are written to w.
func NewTracer(n int, w io.Writer, filter []string) *Tracer {
	t := &Tracer{ring: make([]Record, 0, n), out: w, filter: filter}
	if w != nil {
		t.enc = json.NewEncoder(w)
	}
	return t
}

func (t *Tracer) begin(e *Env, name string) *Record {
	cs, ip := e.Caller()
	t.seq++
	return &Record{Seq: t.seq, Name: name, Caller: fmt.Sprintf("%04X:%04X", cs, ip), In: regsOf(e.CPU)}
}

func (t *Tracer) end(e *Env, r *Record, err error) {
	r.Out = regsOf(e.CPU)
	r.CF = e.CallerFlags()&cpu.FlagCF != 0
	if err != nil && err != cpu.ErrRetry {
		r.Err = err.Error()
	}
	if err == cpu.ErrRetry {
		return // waiting calls would flood the log
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.ring) < cap(t.ring) {
		t.ring = append(t.ring, *r)
	} else {
		t.ring[t.pos] = *r
		t.pos = (t.pos + 1) % len(t.ring)
	}
	if t.enc != nil && t.match(r.Name) {
		_ = t.enc.Encode(r)
	}
}

func (t *Tracer) match(name string) bool {
	if len(t.filter) == 0 {
		return true
	}
	for _, f := range t.filter {
		if strings.HasPrefix(name, f) {
			return true
		}
	}
	return false
}

// Last returns the recorded calls, oldest first.
func (t *Tracer) Last() []Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Record, 0, len(t.ring))
	out = append(out, t.ring[t.pos:]...)
	return append(out, t.ring[:t.pos]...)
}

// Stream writes a diagnostic line (name, msg) to the JSON log only, without
// putting it into the ring of recent calls shown in dumps. It is meant for
// high-volume instrumentation such as the post-event execution log.
func (t *Tracer) Stream(name, msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.enc == nil || !t.match(name) {
		return
	}
	t.seq++
	_ = t.enc.Encode(&Record{Seq: t.seq, Name: name, Note: msg})
}

// Port records an I/O port event (first access to a port without a device).
func (t *Tracer) Port(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq++
	r := Record{Seq: t.seq, Name: "port", Note: msg}
	if len(t.ring) < cap(t.ring) {
		t.ring = append(t.ring, r)
	} else {
		t.ring[t.pos] = r
		t.pos = (t.pos + 1) % len(t.ring)
	}
	if t.enc != nil && t.match(r.Name) {
		_ = t.enc.Encode(r)
	}
}
