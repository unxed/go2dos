package main

import (
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// pollInterval — как часто читающий проверяет флаг паузы.
const pollInterval = 25 * time.Millisecond

// stdinReader читает терминал так, чтобы чтение можно было поставить на
// паузу: блокированный Read украл бы у запущенной команды её первое
// нажатие (docs/HOSTEXEC.md). Ожидание данных идёт с таймаутом
// (waitReadable: poll на Unix, ожидание хэндла консоли на Windows), сам
// read выполняется только когда данные уже есть и пауза не включена.
type stdinReader struct {
	f      *os.File
	paused atomic.Bool
	mu     sync.Mutex // удерживается на время вызова read
}

func newStdinReader(f *os.File) *stdinReader { return &stdinReader{f: f} }

func (r *stdinReader) Read(p []byte) (int, error) {
	for {
		if r.paused.Load() {
			time.Sleep(pollInterval / 2)
			continue
		}
		ok, err := waitReadable(r.f, pollInterval)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		r.mu.Lock()
		if r.paused.Load() {
			r.mu.Unlock()
			continue
		}
		n, err := r.f.Read(p)
		r.mu.Unlock()
		return n, err
	}
}

// pause возвращается, когда ни один read уже не идёт и новый не начнётся.
func (r *stdinReader) pause() {
	r.paused.Store(true)
	r.mu.Lock()
	r.mu.Unlock()
}

func (r *stdinReader) resume() { r.paused.Store(false) }
