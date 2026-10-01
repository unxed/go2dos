package dos

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/unxed/go2dos/hle"
)

// START of the built-in COMMAND.COM (T21, step 4): the host's side of a file
// manager. Needs -host-exec (it leaves the sandbox like "!").
//
//	START file.pdf        open the file with the host's opener (xdg-open, open,
//	                      cmd start; -open-cmd replaces it)
//	START host command    run it detached: no waiting, no output capture, so
//	                      a GUI program does not block the emulator (a plain
//	                      "!command" waits up to hostExecTimeout and shows output)
//	START hello           a DOS program: runs as usual
//
// The line goes to the host shell whole (pipes, redirects: the host's).

// startArg returns the argument of START ("" and false if the line is not START).
func startArg(line string) (string, bool) {
	if len(line) < 5 || upperASCII(line[:5]) != "START" {
		return "", false
	}
	if len(line) > 5 && line[5] != ' ' && line[5] != '\t' {
		return "", false
	}
	return strings.TrimSpace(line[5:]), true
}

// shStart runs START; true if it prepared an EXEC (a DOS program).
func (d *DOS) shStart(e *hle.Env, st *shellState, arg string) bool {
	if arg == "" {
		d.shPrint("Usage: START file | START host-command | START dos-program" + crlf)
		st.level = 255
		return false
	}
	if !d.hostExec {
		d.shPrint("Host commands are disabled (go2dos -host-exec)" + crlf)
		st.level = 255
		return false
	}
	// A DOS program: as if typed without START.
	if _, kind := d.findProgram(firstWord(arg)); kind != "" {
		return d.shellRun(e, st, arg)
	}
	// An existing file or directory of the DOS side: the host's opener.
	if a := shArgs(arg); len(a) == 1 && !strings.ContainsAny(a[0], "*?") {
		if r, errc := d.fs.lfnResolve([]byte(a[0])); errc == 0 && r.exists {
			d.shStartCmd(st, d.openCommand(r.host))
			return false
		}
	}
	d.shStartCmd(st, d.hostShellCmd(d.e.CP.Decode([]byte(arg))))
	return false
}

// shStartCmd starts c detached; errors go to the screen, ERRORLEVEL is 0/255.
func (d *DOS) shStartCmd(st *shellState, c *exec.Cmd) {
	if c == nil {
		d.shPrint("No opener for this host: set -open-cmd" + crlf)
		st.level = 255
		return
	}
	if c.Dir == "" {
		c.Dir = d.hostDir()
	}
	c.Env = os.Environ()
	d.e.Note("host start: %v in %s", c.Args, c.Dir)
	if err := c.Start(); err != nil {
		d.shPrint("Cannot start: " + err.Error() + crlf)
		st.level = 255
		return
	}
	go c.Wait() // reap; the output is dropped (nil → the null device)
	st.level = 0
}

// hostShellCmd is the host shell running a line.
func (d *DOS) hostShellCmd(line string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", line)
	}
	return exec.Command("sh", "-c", line)
}

// openCommand builds the opener for a host path; nil if there is none.
func (d *DOS) openCommand(path string) *exec.Cmd {
	if d.openCmd != "" { // the user's: the path is its last argument
		if runtime.GOOS == "windows" {
			return exec.Command("cmd", "/c", d.openCmd+` "`+path+`"`)
		}
		return exec.Command("sh", "-c", d.openCmd+` "$1"`, "sh", path)
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		return exec.Command("open", path)
	}
	if _, err := exec.LookPath("xdg-open"); err != nil {
		return nil
	}
	return exec.Command("xdg-open", path)
}
