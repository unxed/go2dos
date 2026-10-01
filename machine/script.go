package machine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/unxed/go2dos/keys"
)

// ScriptOptions tunes RunScript.
type ScriptOptions struct {
	KeyDelay       time.Duration // pause after each key (default 30ms)
	WaitForTimeout time.Duration // limit for <waitfor:...> (default 15s)
	Log            io.Writer     // receives <screen> output and progress; may be nil
}

// RunScript feeds a key script to a running machine. It is meant to run in
// its own goroutine next to Run.
func (m *Machine) RunScript(ctx context.Context, steps []keys.Step, o ScriptOptions) error {
	if o.KeyDelay == 0 {
		o.KeyDelay = 30 * time.Millisecond
	}
	if o.WaitForTimeout == 0 {
		o.WaitForTimeout = 15 * time.Second
	}
	logf := func(format string, a ...any) {
		if o.Log != nil {
			fmt.Fprintf(o.Log, format, a...)
		}
	}
	sleep := func(d time.Duration) error {
		select {
		case <-time.After(d):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for _, st := range steps {
		switch {
		case st.Key != nil:
			m.PushKey(*st.Key)
			if err := sleep(o.KeyDelay); err != nil {
				return err
			}
		case st.Wait > 0:
			if err := sleep(st.Wait); err != nil {
				return err
			}
		case st.WaitFor != "":
			deadline := time.Now().Add(o.WaitForTimeout)
			for !strings.Contains(m.Screen().Text(), st.WaitFor) {
				if time.Now().After(deadline) {
					return fmt.Errorf("timed out waiting for %q; screen:\n%s", st.WaitFor, m.Screen().Text())
				}
				if err := sleep(20 * time.Millisecond); err != nil {
					return err
				}
			}
		case st.Screen:
			logf("--- screen ---\n%s\n--------------\n", m.Screen().Text())
		}
	}
	return nil
}
