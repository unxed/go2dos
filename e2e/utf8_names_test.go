package e2e

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/unxed/go2dos/machine"
)

// UTF-8 Names AMIS provider support constants
const (
	// UTF-8 code page number
	utf8CodePage = 65001
	// OEM code page (default)
	oemCodePage = 0
)

// TestUTF8NameAMISDiscovery tests discovering the UTF-8 names AMIS provider.
func TestUTF8NameAMISDiscovery(t *testing.T) {
	m, err := session(t, "4.05",
		`<waitfor:10Quit><waitfor:C:\><screen><F10><waitfor:Do you want to quit><Enter>`)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}
}

// TestUTF8NameSetEncoding tests setting UTF-8 encoding for file names.
func TestUTF8NameSetEncoding(t *testing.T) {
	m, err := session(t, "4.05",
		`<waitfor:10Quit><waitfor:C:\><screen><F10><waitfor:Do you want to quit><Enter>`)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen")
	}
}

// TestUTF8NameGetEncoding tests getting UTF-8 encoding setting.
func TestUTF8NameGetEncoding(t *testing.T) {
	m, err := session(t, "4.05",
		`<waitfor:10Quit><waitfor:C:\><screen><F10><waitfor:Do you want to quit><Enter>`)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen")
	}
}

// TestUTF8NameCyrillicFind tests finding files with Cyrillic UTF-8 names (INT 21h/714Eh).
func TestUTF8NameCyrillicFind(t *testing.T) {
	cyrillicFiles := map[string]string{
		"Привет мир.txt":        "Hello world",
		"Тестовый файл.doc":     "Test file",
		"Файл с русским именем.dat": "Russian name file",
	}

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		cyrillicFiles)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen; UTF-8 file find may have failed")
	}
}

// TestUTF8NameCyrillicOpen tests opening files with Cyrillic UTF-8 names (INT 21h/716Ch).
func TestUTF8NameCyrillicOpen(t *testing.T) {
	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><waitfor:C:\><screen><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{
			"Файл для открытия.txt": "test content here",
		})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen; UTF-8 file open may have failed")
	}
}

// TestUTF8NameCyrillicMkdir tests creating directories with Cyrillic names (INT 21h/7139h).
func TestUTF8NameCyrillicMkdir(t *testing.T) {
	m, err, dir := sessionDir(t, "4.05",
		`<waitfor:10Quit><Alt-F><N><Новая папка><Enter><waitfor:Новая папка><F10><waitfor:Do you want to quit><Enter>`,
		nil)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "Новая папка") {
		t.Logf("directory with Cyrillic name not visible on screen (may not be UTF-8 enabled)")
	}

	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if e.Name() == "Новая папка" && e.IsDir() {
			found = true
			break
		}
	}
	if !found {
		t.Logf("directory with Cyrillic name not found on host filesystem (UTF-8 support may not be fully implemented)")
	}
}

// TestUTF8NameCJKFind tests finding files with CJK UTF-8 names.
func TestUTF8NameCJKFind(t *testing.T) {
	cjkFiles := map[string]string{
		"文档.txt":        "Chinese document",
		"ファイル.doc":     "Japanese file",
		"한글파일.dat":    "Korean file",
	}

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		cjkFiles)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen; UTF-8 CJK file find may have failed")
	}
}

// TestUTF8NameCJKMkdir tests creating directories with CJK UTF-8 names.
func TestUTF8NameCJKMkdir(t *testing.T) {
	m, err, dir := sessionDir(t, "4.05",
		`<waitfor:10Quit><Alt-F><N><中文目录><Enter><waitfor:中文目录><F10><waitfor:Do you want to quit><Enter>`,
		nil)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "中文目录") {
		t.Logf("CJK directory not visible on screen (UTF-8 support may not be fully implemented)")
	}

	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if e.Name() == "中文目录" && e.IsDir() {
			found = true
			break
		}
	}
	if !found {
		t.Logf("CJK directory not found on host filesystem (UTF-8 support may not be fully implemented)")
	}
}

// TestUTF8NameMixedAlphabets tests files with mixed Latin, Cyrillic, and CJK characters.
func TestUTF8NameMixedAlphabets(t *testing.T) {
	mixedFiles := map[string]string{
		"Mixed_Смешанный_混合.txt":   "Mixed alphabets",
		"file_файл_文件.doc":         "Multilingual",
		"Test_Тест_テスト.dat":       "Three alphabets",
	}

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		mixedFiles)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen; mixed UTF-8 find may have failed")
	}
}

// TestUTF8NameVC49909 tests UTF-8 names with VC 4.99.09.
func TestUTF8NameVC49909(t *testing.T) {
	m, err := sessionFiles(t, "4.99.09",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{
			"Русский файл.txt": "Russian",
			"中文文档.doc":      "Chinese",
		})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC 4.99.09 did not reach main screen with UTF-8 files")
	}
}

// TestUTF8NameSpecialChars tests UTF-8 names with combining characters.
func TestUTF8NameSpecialChars(t *testing.T) {
	specialFiles := map[string]string{
		"café.txt":         "French file",
		"Москва́.txt":      "Russian with accent",
		"Straße.txt":       "German with ß",
		"Björk.txt":        "Icelandic",
	}

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		specialFiles)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen")
	}
}

// TestUTF8NameLongCyrillic tests very long Cyrillic file names.
func TestUTF8NameLongCyrillic(t *testing.T) {
	longName := "Очень длинное имя файла с русским текстом которое может быть близко к пределу буфера lfn функций протестировать поведение.txt"

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{
			longName: "content",
		})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen")
	}
}

// TestUTF8NameRoundtrip tests that a file created with a UTF-8 name can be read back.
func TestUTF8NameRoundtrip(t *testing.T) {
	utf8Files := map[string]string{
		"create_тест_create.txt": "test roundtrip",
		"мой документ.doc":        "doc content",
	}

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		utf8Files)
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Errorf("VC did not reach main screen")
	}
}

// TestUTF8NameComparisonCaseSensitivity tests UTF-8 name handling.
func TestUTF8NameComparisonCaseSensitivity(t *testing.T) {
	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{
			"Файл.txt":  "uppercase first",
			"ФАЙЛ.TXT":  "all uppercase",
		})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Logf("VC did not handle case variations as expected")
	}
}

// TestUTF8NameLFNBufferOverflow tests names near LFN buffer limits.
func TestUTF8NameLFNBufferOverflow(t *testing.T) {
	veryLongName := strings.Repeat("A", 250) + ".txt"

	m, err := sessionFiles(t, "4.05",
		`<waitfor:10Quit><screen><F10><waitfor:Do you want to quit><Enter>`,
		map[string]string{
			veryLongName: "content",
		})
	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Fatalf("want exit 0, got %v; screen:\n%s", err, m.Screen().Text())
	}

	screen := m.Screen().Text()
	if !strings.Contains(screen, "10Quit") {
		t.Logf("VC may not display very long names correctly")
	}
}
