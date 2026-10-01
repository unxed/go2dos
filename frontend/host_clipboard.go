package frontend

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// HostClipboard reads/writes to the system clipboard using OS-specific tools.
type HostClipboard struct {
	mu sync.Mutex
}

// GetText returns text from the system clipboard.
func (c *HostClipboard) GetText() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS: use pbpaste
		cmd = exec.Command("pbpaste")
	case "windows":
		// Windows: use PowerShell
		cmd = exec.Command("powershell.exe", "-NoProfile", "-Command", "[System.Windows.Forms.Clipboard]::GetText()")
	case "linux", "freebsd", "openbsd", "netbsd":
		// Linux/BSD: try xclip, then xsel
		text, err := tryCommand("xclip", "-selection", "clipboard", "-o")
		if err == nil {
			return text, nil
		}
		text, err = tryCommand("xsel", "--clipboard", "--output")
		if err == nil {
			return text, nil
		}
		// Fall back to wl-paste for Wayland
		text, err = tryCommand("wl-paste")
		if err == nil {
			return text, nil
		}
		return "", errors.New("clipboard not available (install xclip, xsel, or wl-paste)")
	default:
		return "", errors.New("clipboard not supported on " + runtime.GOOS)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// SetText writes text to the system clipboard.
func (c *HostClipboard) SetText(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// macOS: use pbcopy
		cmd = exec.Command("pbcopy")
	case "windows":
		// Windows: use PowerShell
		cmd = exec.Command("powershell.exe", "-NoProfile", "-Command",
			"[System.Windows.Forms.Clipboard]::SetText($input)")
	case "linux", "freebsd", "openbsd", "netbsd":
		// Linux/BSD: try xclip, then xsel
		err := trySetCommand(text, "xclip", "-selection", "clipboard")
		if err == nil {
			return nil
		}
		err = trySetCommand(text, "xsel", "--clipboard", "--input")
		if err == nil {
			return nil
		}
		// Fall back to wl-copy for Wayland
		err = trySetCommand(text, "wl-copy")
		if err == nil {
			return nil
		}
		return errors.New("clipboard not available (install xclip, xsel, or wl-copy)")
	default:
		return errors.New("clipboard not supported on " + runtime.GOOS)
	}

	cmd.Stdin = strings.NewReader(text)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// tryCommand runs a command and returns its output if successful.
func tryCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// trySetCommand runs a command with stdin set to text.
func trySetCommand(text string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
