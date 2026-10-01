package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/unxed/go2dos/bios"
	"github.com/unxed/go2dos/cp"
	"github.com/unxed/vtui"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO2DOS_TEST_PTY") == "1" {
		os.Exit(ptyHelper())
	}
	os.Exit(m.Run())
}

// ptyHelper — хост для теста: настоящий host на stdin/stdout, который тест
// держит через pty. Печатает построчно, что происходит; клавиша 'a' запускает
// команду хоста через RunAttached.
func ptyHelper() int {
	h := newHost(vtui.NewSilentScreenBuf())
	h.protocols = 0 // только сырой режим: pty не отвечает на запросы протоколов
	page, _ := cp.Get(437)
	keyCh := make(chan bios.KeyEvent, 16)
	quit := make(chan bool, 1)
	h.push = func(k bios.KeyEvent) { keyCh <- k }
	h.page = page
	h.stop = func(dump bool) { quit <- dump }
	say := func(f string, a ...any) { fmt.Fprintf(os.Stdout, f+"\r\n", a...) }
	if err := h.startInput(); err != nil {
		fmt.Fprintln(os.Stderr, "startInput:", err)
		return 1
	}
	script := `echo started; stty -a | tr ' ' '\n' | grep -x -e icanon -e -icanon; IFS= read l; echo "got:$l"`
	say("READY")
	for {
		select {
		case k := <-keyCh:
			say("KEY:%02X%02X", k.Scan, k.ASCII)
			if k.ASCII == 'a' {
				say("ATTACH")
				err := h.RunAttached(exec.Command("sh", "-c", script))
				say("DETACHED err=%v attached=%v", err, h.attached.Load())
			}
		case dump := <-quit:
			say("QUIT dump=%v", dump)
			h.stopInput()
			return 0
		}
	}
}

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

// Передача терминала команде хоста в vtui-фронтенде: чтение vtinput
// останавливается (Reader.Close), команда получает обычный режим терминала и
// набранную во время неё строку, после команды клавиши снова разбираются.
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
	send("x")
	log.waitFor(t, "KEY:2D78")

	send("a")
	log.waitFor(t, "KEY:1E61")
	log.waitFor(t, "started")
	log.waitFor(t, "\r\nicanon\r\n")
	send("zz\n")
	log.waitFor(t, "got:zz")
	log.waitFor(t, "DETACHED err=<nil> attached=false")
	if strings.Contains(log.String(), "KEY:2C7A") {
		t.Errorf("the keystroke typed during the command went to the key parser:\n%q", log.String())
	}

	send("b") // после команды чтение vtinput работает заново
	log.waitFor(t, "KEY:3062")

	send("\x1dq")
	log.waitFor(t, "QUIT dump=false")
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("helper exit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not exit")
	}
}
