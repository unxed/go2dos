package frontend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf16"
)

// HostClipboardExtended reads/writes text and files to the system clipboard.
type HostClipboardExtended struct {
	mu sync.Mutex
}

// GetText returns text from the system clipboard.
func (c *HostClipboardExtended) GetText() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "windows":
		cmd = exec.Command("powershell.exe", "-NoProfile", "-Command",
			"[System.Windows.Forms.Clipboard]::GetText()")
	case "linux", "freebsd", "openbsd", "netbsd":
		text, err := tryCommand("xclip", "-selection", "clipboard", "-o")
		if err == nil {
			return text, nil
		}
		text, err = tryCommand("xsel", "--clipboard", "--output")
		if err == nil {
			return text, nil
		}
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
func (c *HostClipboardExtended) SetText(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("powershell.exe", "-NoProfile", "-Command",
			"[System.Windows.Forms.Clipboard]::SetText($input)")
	case "linux", "freebsd", "openbsd", "netbsd":
		err := trySetCommand(text, "xclip", "-selection", "clipboard")
		if err == nil {
			return nil
		}
		err = trySetCommand(text, "xsel", "--clipboard", "--input")
		if err == nil {
			return nil
		}
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

// GetFiles returns file paths from the system clipboard (CF_HDROP format).
func (c *HostClipboardExtended) GetFiles() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch runtime.GOOS {
	case "windows":
		return getFilesWindows()
	case "darwin":
		return getFilesOSX()
	case "linux", "freebsd", "openbsd", "netbsd":
		return getFilesLinux()
	default:
		return []string{}, errors.New("file clipboard not supported on " + runtime.GOOS)
	}
}

// SetFiles writes file paths to the system clipboard in CF_HDROP format.
func (c *HostClipboardExtended) SetFiles(paths []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch runtime.GOOS {
	case "windows":
		return setFilesWindows(paths)
	case "darwin":
		return setFilesOSX(paths)
	case "linux", "freebsd", "openbsd", "netbsd":
		return setFilesLinux(paths)
	default:
		return errors.New("file clipboard not supported on " + runtime.GOOS)
	}
}

// IsDragDropAvailable returns whether drag-drop is supported on this platform.
func (c *HostClipboardExtended) IsDragDropAvailable() bool {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return true
	default:
		return false
	}
}

// OnDragDrop is called when files are dragged and dropped (not implemented in basic host clipboard).
func (c *HostClipboardExtended) OnDragDrop(paths []string) bool {
	// For now, just store in clipboard
	return c.SetFiles(paths) == nil
}

// getFilesWindows retrieves file paths from Windows clipboard in CF_HDROP format.
func getFilesWindows() ([]string, error) {
	// Use PowerShell to get files from clipboard
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"$files = @(); if ([System.Windows.Forms.Clipboard]::ContainsFileDropList()) { $files = @([System.Windows.Forms.Clipboard]::GetFileDropList()); } $files -join \"`n\"")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return []string{}, nil // No files on clipboard
	}
	output := strings.TrimSuffix(out.String(), "\n")
	if output == "" {
		return []string{}, nil
	}
	return strings.Split(output, "\n"), nil
}

// setFilesWindows writes file paths to Windows clipboard in CF_HDROP format.
func setFilesWindows(paths []string) error {
	if len(paths) == 0 {
		return errors.New("no files to set")
	}
	// Build PowerShell command to set files
	var sb strings.Builder
	sb.WriteString("[System.Windows.Forms.Clipboard]::SetFileDropList(@(")
	for i, p := range paths {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(`"`)
		sb.WriteString(strings.ReplaceAll(p, `"`, `\"`))
		sb.WriteString(`"`)
	}
	sb.WriteString("))")
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", sb.String())
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getFilesOSX retrieves file paths from macOS clipboard.
func getFilesOSX() ([]string, error) {
	// Use osascript to get file list from clipboard
	cmd := exec.Command("osascript", "-e",
		`the clipboard as «class furl» as text`)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return []string{}, nil
	}
	output := strings.TrimSuffix(out.String(), "\n")
	if output == "" {
		return []string{}, nil
	}
	var paths []string
	for _, p := range strings.Split(output, "\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			// Remove "file://" prefix if present
			if strings.HasPrefix(p, "file://") {
				p = p[7:]
			}
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// setFilesOSX writes file paths to macOS clipboard.
func setFilesOSX(paths []string) error {
	if len(paths) == 0 {
		return errors.New("no files to set")
	}
	var sb strings.Builder
	sb.WriteString("set the clipboard to {")
	for i, p := range paths {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(`POSIX file "`)
		sb.WriteString(strings.ReplaceAll(p, `"`, `\"`))
		sb.WriteString(`"`)
	}
	sb.WriteString("}")
	cmd := exec.Command("osascript", "-e", sb.String())
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getFilesLinux retrieves file paths from X11/Wayland clipboard.
func getFilesLinux() ([]string, error) {
	// Try xclip with file URIs
	text, err := tryCommand("xclip", "-selection", "clipboard", "-o", "-t", "text/uri-list")
	if err == nil {
		return parseURIList(text), nil
	}
	// Fallback: try with x-special/gnome-copied-files
	text, err = tryCommand("xclip", "-selection", "clipboard", "-o", "-t", "x-special/gnome-copied-files")
	if err == nil {
		return parseGnomeCopiedFiles(text), nil
	}
	// Fallback: try plain text paths
	text, err = tryCommand("xclip", "-selection", "clipboard", "-o")
	if err == nil && text != "" {
		// Could be file paths, split by newlines
		var paths []string
		for _, p := range strings.Split(text, "\n") {
			p = strings.TrimSpace(p)
			if p != "" && strings.HasPrefix(p, "/") {
				paths = append(paths, p)
			}
		}
		if len(paths) > 0 {
			return paths, nil
		}
	}
	return []string{}, nil
}

// setFilesLinux writes file paths to X11/Wayland clipboard.
func setFilesLinux(paths []string) error {
	if len(paths) == 0 {
		return errors.New("no files to set")
	}
	// Build URI list format
	var sb strings.Builder
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			absPath = p
		}
		sb.WriteString("file://")
		sb.WriteString(strings.ReplaceAll(absPath, " ", "%20"))
		sb.WriteString("\n")
	}
	return trySetCommand(sb.String(), "xclip", "-selection", "clipboard", "-t", "text/uri-list")
}

// parseURIList parses a text/uri-list format clipboard content.
func parseURIList(text string) []string {
	var paths []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Remove file:// prefix
		if strings.HasPrefix(line, "file://") {
			path := line[7:]
			// URL-decode spaces and other characters
			path = strings.ReplaceAll(path, "%20", " ")
			path = strings.ReplaceAll(path, "%2F", "/")
			paths = append(paths, path)
		} else {
			paths = append(paths, line)
		}
	}
	return paths
}

// parseGnomeCopiedFiles parses GNOME's x-special/gnome-copied-files format.
func parseGnomeCopiedFiles(text string) []string {
	var paths []string
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return paths
	}
	// First line is operation type (copy/move/etc)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove file:// prefix
		if strings.HasPrefix(line, "file://") {
			path := line[7:]
			path = strings.ReplaceAll(path, "%20", " ")
			path = strings.ReplaceAll(path, "%2F", "/")
			paths = append(paths, path)
		} else {
			paths = append(paths, line)
		}
	}
	return paths
}

// EncodeCFHDROP encodes file paths into CF_HDROP format (Windows HDROP clipboard format).
// The format is: 4-byte offset to each file (starting from 0), followed by null-terminated filenames.
func EncodeCFHDROP(paths []string) []byte {
	if len(paths) == 0 {
		// Empty HDROP: just the double null terminator
		return []byte{0, 0, 0, 0}
	}

	var buf bytes.Buffer

	// Calculate total size: offsets + filenames
	fileDataStart := (len(paths) + 1) * 4 // offsets + final 0 offset
	offset := fileDataStart

	// Write offsets
	for _, p := range paths {
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		// UTF-16 encoded path + null terminator
		encoded := utf16.Encode([]rune(p))
		offset += (len(encoded) + 1) * 2 // +1 for null terminator
	}
	// Write final offset (0) to mark end
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Write file paths
	for _, p := range paths {
		encoded := utf16.Encode([]rune(p))
		binary.Write(&buf, binary.LittleEndian, encoded)
		binary.Write(&buf, binary.LittleEndian, uint16(0)) // null terminator
	}

	return buf.Bytes()
}

// DecodeCFHDROP decodes CF_HDROP format to file paths.
func DecodeCFHDROP(data []byte) ([]string, error) {
	if len(data) < 4 {
		return []string{}, errors.New("invalid HDROP format")
	}

	var paths []string
	var offset uint32

	// Read offsets
	for {
		if len(data) < 4 {
			break
		}
		offset = binary.LittleEndian.Uint32(data[0:4])
		data = data[4:]

		if offset == 0 {
			break // End marker
		}

		// Offset is relative to the start of the HDROP structure
		// We need to find the file at this offset
		if int(offset) >= len(data) {
			break
		}

		// Read null-terminated wide string
		var runes []rune
		fileData := data[int(offset):]
		for i := 0; i < len(fileData)-1; i += 2 {
			if i+1 < len(fileData) {
				ch := binary.LittleEndian.Uint16(fileData[i : i+2])
				if ch == 0 {
					break
				}
				runes = append(runes, rune(ch))
			}
		}
		if len(runes) > 0 {
			paths = append(paths, string(runes))
		}
	}

	return paths, nil
}
