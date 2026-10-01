package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
)

func init() { ptyHelperMain = ptyHelper }

// ptyHelper — «go2dos» для теста: настоящий termHost на stdin/stdout, который
// тест держит через pty. Печатает построчно, что происходит; клавиши
// 'a' и 'c' запускают команды хоста через RunAttached.
func ptyHelper() int {
	h := newTermHost(os.Stdout)
	var outMu sync.Mutex
	say := func(f string, a ...any) {
		outMu.Lock()
		defer outMu.Unlock()
		fmt.Fprintf(os.Stdout, f+"\r\n", a...)
	}
	restore, err := h.setup(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		return 1
	}
	defer restore()
	page, _ := cp.Get(437)
	keyCh := make(chan bios.KeyEvent, 16)
	cmdCh := make(chan termCmd, 1)
	p := &inputParser{page: page, push: func(k bios.KeyEvent) { keyCh <- k }, cmd: func(c termCmd) { cmdCh <- c }}
	go p.run(h.in)
	// как в frontend.Run: Ctrl-C во время команды хоста принадлежит команде
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for range sig {
			if !h.Attached() {
				say("HELPER-GOT-SIGINT")
			}
		}
	}()
	scripts := map[byte]string{
		'a': `echo started; stty -a | tr ' ' '\n' | grep -x -e icanon -e -icanon; IFS= read l; echo "got:$l"`,
		'c': `trap 'echo child-int; exit 7' INT; echo ready2; while :; do sleep 0.1; done`,
	}
	say("READY")
	for {
		select {
		case k := <-keyCh:
			say("KEY:%02X%02X", k.Scan, k.ASCII)
			if script, ok := scripts[k.ASCII]; ok {
				say("ATTACH")
				err := h.RunAttached(exec.Command("sh", "-c", script))
				say("DETACHED err=%v", err)
			}
		case c := <-cmdCh:
			say("CMD:%d", c)
			return 0
		}
	}
}

// openPty открывает пару pty (Linux): ведущую и ведомую стороны.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skip("no /dev/ptmx:", err)
	}
	if err := unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	s, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	return m, s
}

// screenLog собирает всё, что процесс напечатал в pty.
type screenLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *screenLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *screenLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *screenLog) waitFor(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(l.String(), text) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %q in the output:\n%q", text, l.String())
}

// Передача терминала команде хоста: поток ввода на паузе (нажатие, набранное
// во время команды, достаётся команде, а не разборщику клавиш), у команды
// обычный режим терминала (icanon) и свой Ctrl-C, после команды разбор клавиш
// продолжается.
func TestRunAttachedPty(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	master, slave := openPty(t)
	defer master.Close()
	log := &screenLog{}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			log.Write(buf[:n])
			if err != nil {
				return
			}
		}
	}()

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "GO2DOS_TEST_PTY=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer cmd.Process.Kill()

	send := func(s string) {
		t.Helper()
		if _, err := master.WriteString(s); err != nil {
			t.Fatal(err)
		}
	}

	log.waitFor(t, "READY")
	send("x") // обычная клавиша разбирается
	log.waitFor(t, "KEY:2D78")

	send("a") // разбирается, затем запускается команда
	log.waitFor(t, "KEY:1E61")
	log.waitFor(t, "started")
	log.waitFor(t, "\r\nicanon\r\n") // у команды обычный режим терминала
	send("zz\n")                     // достаётся команде
	log.waitFor(t, "got:zz")
	log.waitFor(t, "DETACHED err=<nil>")
	if strings.Contains(log.String(), "KEY:2C7A") {
		t.Errorf("the keystroke typed during the command went to the key parser:\n%q", log.String())
	}

	send("b") // после команды разбор клавиш продолжается
	log.waitFor(t, "KEY:3062")

	send("c") // Ctrl-C принадлежит команде
	log.waitFor(t, "ready2")
	send("\x03")
	log.waitFor(t, "child-int")
	log.waitFor(t, "DETACHED err=exit status 7")
	if strings.Contains(log.String(), "HELPER-GOT-SIGINT") {
		t.Errorf("SIGINT during the command was not left to the command:\n%q", log.String())
	}

	send("\x1dq") // служебная клавиша: выход
	log.waitFor(t, "CMD:1")
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("helper exit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not exit")
	}
}
