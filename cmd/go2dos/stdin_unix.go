//go:build !windows

package main

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// waitReadable ждёт данные на f не дольше d. Обрыв и ошибка считаются
// готовностью: следующий read вернёт EOF или ошибку.
func waitReadable(f *os.File, d time.Duration) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(f.Fd()), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, int(d.Milliseconds()))
	if err == unix.EINTR {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0 && fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0, nil
}
