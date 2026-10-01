package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unxed/go2dos/dos"
	"github.com/unxed/go2dos/machine"
)

// VC 4.05 with the clipboard client (third_party/vc/patches/vc-4.05-clip.patch,
// built by tools/build-vc405-pts.sh into bin/4.05-clip): in line editing and on
// the command line Ctrl-Ins copies the line, Shift-Ins pastes the first line of
// the clipboard text, Shift-Del cuts the line. The server is WinOldAp
// (INT 2Fh AX=17xxh) over dos.MemClipboard.
func clipSession(t *testing.T, script string, clip dos.Clipboard) {
	t.Helper()
	clipSessionFiles(t, script, clip, nil)
}

// clipSessionFiles is clipSession with extra files in the drive; it returns the
// host directory of C:.
func clipSessionFiles(t *testing.T, script string, clip dos.Clipboard, extra map[string]string) string {
	t.Helper()
	if _, err := os.Stat(vcDir(t, "4.05-clip")); err != nil {
		t.Skip("no VC 4.05 with the clipboard client (tools/build-vc.sh builds bin/4.05-clip)")
	}
	m, err, dir := sessionOpts(t, "4.05-clip", script, extra, machine.Config{Clipboard: clip})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}
	return dir
}

const clipQuit = `<F10><waitfor:Do you want to quit><Enter>`

func TestVC405ClipCopy(t *testing.T) {
	clip := &dos.MemClipboard{Text: "old"}
	clipSession(t, `<waitfor:10Quit>hello world<Ctrl-Ins>`+clipQuit, clip)
	if clip.Text != "hello world" {
		t.Errorf("clipboard %q, want the command line", clip.Text)
	}
}

func TestVC405ClipPaste(t *testing.T) {
	clip := &dos.MemClipboard{Text: "from clip\nsecond line"}
	// only the first line goes to the command line; then the line is copied back
	clipSession(t, `<waitfor:10Quit><Shift-Ins><waitfor:from clip>x<Ctrl-Ins>`+clipQuit, clip)
	if clip.Text != "from clipx" {
		t.Errorf("clipboard %q, want %q", clip.Text, "from clipx")
	}
}

// Cut: the line goes to the clipboard and is cleared. Pasting it back and
// typing more shows both: "abc" once (cleared), and in the clipboard (copied).
func TestVC405ClipCut(t *testing.T) {
	clip := &dos.MemClipboard{}
	clipSession(t, `<waitfor:10Quit>abc<Shift-Del><Shift-Ins>xyz<Ctrl-Ins>`+clipQuit, clip)
	if clip.Text != "abcxyz" {
		t.Errorf("clipboard %q, want %q", clip.Text, "abcxyz")
	}
}

// Del without Shift still deletes a character.
func TestVC405ClipDelStillDeletes(t *testing.T) {
	clip := &dos.MemClipboard{}
	clipSession(t, `<waitfor:10Quit>abc<Ctrl-S><Del><Ctrl-Ins>`+clipQuit, clip)
	if clip.Text != "ab" {
		t.Errorf("clipboard %q, want %q", clip.Text, "ab")
	}
}

// Without a clipboard server VC works as before: the keys do nothing, and
// Shift-Del only deletes a character instead of losing the line.
func TestVC405ClipNoServer(t *testing.T) {
	clipSession(t, `<waitfor:10Quit>abc<Ctrl-Ins><Shift-Ins><Ctrl-S><Shift-Del>x<waitfor:abx>`+clipQuit, nil)
}

// Without VCEXT.BIN next to VC.COM VC works as before: the keys do nothing, Shift-Del
// only deletes a character and the clipboard is not touched (G8).
func TestVC405ClipNoModule(t *testing.T) {
	src := vcDir(t, "4.05-clip")
	if _, err := os.Stat(src); err != nil {
		t.Skip("no VC 4.05 with the clipboard client")
	}
	root := t.TempDir()
	dst := filepath.Join(root, "4.05-clip")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() || e.Name() == "VCEXT.BIN" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GO2DOS_VC_DIR", root)
	clip := &dos.MemClipboard{Text: "old"}
	clipSession(t, `<waitfor:10Quit>abc<Ctrl-Ins><Shift-Ins><Ctrl-S><Shift-Del>x<waitfor:abx>`+clipQuit, clip)
	if clip.Text != "old" {
		t.Errorf("clipboard %q, want it untouched", clip.Text)
	}
}

// After a program has run, VC starts again through Init11 and must load the module
// again: pasting works after a command.
func TestVC405ClipAfterExec(t *testing.T) {
	clip := &dos.MemClipboard{Text: "after exec"}
	clipSession(t, `<waitfor:10Quit>ver<Enter><wait:1s><Shift-Ins><waitfor:after exec>`+clipQuit, clip)
}

// A text of 1.5 KB (more than a screen row, less than a screen): the first line is
// pasted, the rest is ignored.
func TestVC405ClipPasteBig(t *testing.T) {
	big := "first line"
	for i := 0; i < 20; i++ {
		big += "\n" + strings.Repeat("x", 74)
	}
	clip := &dos.MemClipboard{Text: big}
	clipSession(t, `<waitfor:10Quit><Shift-Ins><waitfor:first line>`+clipQuit, clip)
}

// The text editor (F4): Ctrl-Ins copies the current line, Shift-Ins inserts the first
// line of the clipboard (with its line end) at the cursor, Shift-Del cuts the line.
const edLines = "one\r\ntwo\r\nthree\r\n"

// edScript opens ZLINES.TXT (the last file of the panel) in the editor, runs keys, saves
// (F2) and leaves the editor.
func edScript(keys string) string {
	keys = strings.ReplaceAll(keys, "<w>", "<wait:300ms>") // VC clears the type-ahead after each key of the editor
	return `<waitfor:10Quit><waitfor:zlines   txt><End><F4><waitfor:one>` + keys + `<F2><wait:300ms><Esc><wait:300ms>` + clipQuit
}

func TestVC405ClipEditorCopyPaste(t *testing.T) {
	clip := &dos.MemClipboard{}
	dir := clipSessionFiles(t, edScript(`<Down><w><Ctrl-Ins><w><Down><w><Shift-Ins><w>`), clip, map[string]string{"ZLINES.TXT": edLines})
	if clip.Text != "two" {
		t.Errorf("clipboard %q, want %q", clip.Text, "two")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "ZLINES.TXT"))
	if string(b) != "one\r\ntwo\r\ntwo\r\nthree\r\n" {
		t.Errorf("file %q", b)
	}
}

func TestVC405ClipEditorCut(t *testing.T) {
	clip := &dos.MemClipboard{}
	dir := clipSessionFiles(t, edScript(`<Down><w><Shift-Del><w>`), clip, map[string]string{"ZLINES.TXT": edLines})
	if clip.Text != "two" {
		t.Errorf("clipboard %q, want %q", clip.Text, "two")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "ZLINES.TXT"))
	if string(b) != "one\r\nthree\r\n" {
		t.Errorf("file %q", b)
	}
}

// Paste of text from outside, and Del without Shift still deletes a character.
func TestVC405ClipEditorPasteOuterAndDel(t *testing.T) {
	clip := &dos.MemClipboard{Text: "from clip\nsecond"}
	dir := clipSessionFiles(t, edScript(`<Del><w><Shift-Ins><w>`), clip, map[string]string{"ZLINES.TXT": edLines})
	b, _ := os.ReadFile(filepath.Join(dir, "ZLINES.TXT"))
	if string(b) != "from clip\r\nne\r\ntwo\r\nthree\r\n" {
		t.Errorf("file %q", b)
	}
}
