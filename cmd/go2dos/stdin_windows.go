//go:build windows

package main

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// waitReadable ждёт сигнала хэндла консольного ввода не дольше d.
// Хэндл сигналит и на события без клавиши (мышь, фокус) — тогда следующий
// read может задержаться до нажатия; пауза в таком случае наступит позже
// (на Windows не проверялось).
func waitReadable(f *os.File, d time.Duration) (bool, error) {
	ev, err := windows.WaitForSingleObject(windows.Handle(f.Fd()), uint32(d.Milliseconds()))
	if err != nil {
		return false, err
	}
	return ev == windows.WAIT_OBJECT_0, nil
}
