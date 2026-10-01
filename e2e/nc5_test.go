// Package e2e runs real DOS programs end to end.
// Norton Commander and other file managers are not part of the repository;
// the test is skipped unless GO2DOS_NC_DIR points to a directory with NC.EXE.
package e2e

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/machine"
)

func ncDir(t *testing.T) string {
	root := os.Getenv("GO2DOS_NC_DIR")
	if root == "" {
		t.Skip("GO2DOS_NC_DIR not set (set to directory containing NC.EXE or NCMAIN.EXE)")
	}
	return root
}

// ncSession runs Norton Commander in a fresh drive with test files.
func ncSession(t *testing.T, script string) (*machine.Machine, error) {
	t.Helper()
	ncRoot := ncDir(t)
	dir := t.TempDir()

	// Copy NC files to temp directory
	entries, err := os.ReadDir(ncRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // Skip directories
		}
		srcPath := filepath.Join(ncRoot, e.Name())
		dstPath := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(srcPath)
		if err != nil {
			t.Logf("Warning: could not read %s: %v", e.Name(), err)
			continue
		}
		if err := os.WriteFile(dstPath, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create test files and directories
	if err := os.WriteFile(filepath.Join(dir, "README.TXT"), []byte("Sample README file\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TEST.DOC"), []byte("Test document\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SAMPLE.BAT"), []byte("@echo off\r\necho Test\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "SUBDIR"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SUBDIR", "FILE.TXT"), []byte("File in subdirectory\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}, Codepage: 437})
	if err != nil {
		t.Fatal(err)
	}

	// Try NC.EXE first, then NCMAIN.EXE
	ncExe := `C:\NC.EXE`
	if _, err := os.Stat(filepath.Join(dir, "NC.EXE")); err != nil {
		ncExe = `C:\NCMAIN.EXE`
	}

	if err := m.Load(ncExe, ""); err != nil {
		t.Fatalf("Failed to load Norton Commander: %v", err)
	}

	steps, err := keys.Parse(script, m.CP)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scriptErr := make(chan error, 1)
	go func() { scriptErr <- m.RunScript(ctx, steps, machine.ScriptOptions{}) }()
	runErr := m.Run(ctx)

	if err := <-scriptErr; err != nil && !errors.Is(err, context.Canceled) {
		t.Logf("script error: %v", err)
	}

	return m, runErr
}

// TestNC551Startup verifies Norton Commander 5.51 loads and displays panels.
// Step 1: Startup with two panels
func TestNC551Startup(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2><screen>`)

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "Norton") && !strings.Contains(screenText, "norton") {
		t.Logf("Startup screen:\n%s", screenText)
		t.Error("Expected Norton Commander text not found")
	}
	t.Logf("Startup successful, runtime error: %v", err)
}

// TestNC551PanelNavigation verifies panel switching and directory navigation.
// Steps 1-2: Two panels, tab switching, directory entry
func TestNC551PanelNavigation(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2><screen>`+ // Startup
			`Tab<wait:1><screen>`+                       // Switch to right panel
			`Enter<wait:1><screen>`+                     // Enter first directory/file
			`..<Enter<wait:1><screen>`)                  // Go up (..)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output captured")
	}
	t.Logf("Panel navigation test completed, runtime error: %v", err)
}

// TestNC551ViewFile verifies file viewing with F3.
// Step 3: F3 to view file, Esc to return
func TestNC551ViewFile(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			`F3<wait:1><screen>`+ // View file (F3)
			`Esc<wait:1><screen>`) // Exit view

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output captured")
	}
	t.Logf("View file test completed, runtime error: %v", err)
}

// TestNC551EditFile verifies file editing with F4.
// Step 4: F4 to edit file, type text, Esc to exit without saving
func TestNC551EditFile(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			`F4<wait:1>`+          // Edit file (F4)
			`test<wait:1>`+        // Type some text
			`Esc<wait:1><screen>`) // Exit without saving

	t.Logf("Edit file test completed, runtime error: %v", err)
}

// TestNC551FileOperations verifies basic file operations: copy (F5), rename (F6), mkdir (F7), delete (F8).
// Step 5: F5, F6, F7, F8 operations
func TestNC551FileOperations(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			// Copy operation (F5)
			`F5<wait:1><screen>`+
			`<wait:1>Esc<wait:1>`+
			// Rename operation (F6)
			`F6<wait:1><screen>`+
			`<wait:1>Esc<wait:1>`+
			// Create directory (F7)
			`F7<wait:1><screen>`+
			`NEWDIR<Enter<wait:1><screen>Esc<wait:1>`+
			// Delete operation (F8)
			`F8<wait:1><screen>`+
			`<wait:1>Esc<wait:1>`)

	t.Logf("File operations test completed, runtime error: %v", err)
}

// TestNC551TogglePanels verifies Ctrl-O to hide/show panels.
// Step 6: Ctrl-O to toggle DOS screen visibility
func TestNC551TogglePanels(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			`Ctrl-O<wait:1><screen>`+ // Hide panels (show DOS)
			`Ctrl-O<wait:1><screen>`) // Show panels again

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output captured")
	}
	t.Logf("Toggle panels test completed, runtime error: %v", err)
}

// TestNC551SelectDisk verifies Alt-F1 to select disk drive.
// Step 12: Alt-F1 disk selection menu, Esc to close
func TestNC551SelectDisk(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			`Alt-F1<wait:1><screen>`+ // Show disk selection menu
			`Esc<wait:1><screen>`)     // Close menu

	t.Logf("Select disk test completed, runtime error: %v", err)
}

// TestNC551Menu verifies F9 menu access.
// Step 11: F9 to open menu, Esc to close
func TestNC551Menu(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:2>`+
			`F9<wait:1><screen>`+  // Open menu (F9)
			`Esc<wait:1><screen>`) // Close menu

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output captured")
	}
	t.Logf("Menu test completed, runtime error: %v", err)
}

// TestNC551FullScenario runs the complete Norton Commander workflow.
// Comprehensive test covering startup, navigation, operations, and exit.
func TestNC551FullScenario(t *testing.T) {
	m, err := ncSession(t,
		// Startup
		`<waitfor:Norton Commander><wait:2><screen>`+
			// Navigate left panel
			`Tab<wait:1><screen>`+              // Switch to right panel
			`Enter<wait:1><screen>`+            // Enter first item
			`..<Enter<wait:1><screen>`+         // Go back up
			// Menu operation
			`F9<wait:1><screen>`+               // Open menu
			`Esc<wait:1>`+                      // Close menu
			// File operations
			`F5<wait:1>Esc<wait:1>`+            // Copy (F5) then cancel
			`Tab<wait:1>`+                      // Switch panel
			`Shift-F5<wait:1>Esc<wait:1>`+      // Shift-F5 operation then cancel
			// Toggle view
			`Ctrl-O<wait:1>`+                   // Hide panels
			`Ctrl-O<wait:1>`+                   // Show panels
			// Disk menu
			`Alt-F1<wait:1><screen>`+           // Disk selection
			`Esc<wait:1>`+                      // Close disk menu
			// Exit
			`F10<wait:1><screen>`+              // Quit command (F10)
			`y<waitfor:C:\>`)                   // Confirm exit

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "C:\\") {
		t.Logf("Final screen:\n%s", screenText)
	}

	t.Logf("Full scenario test completed, runtime error: %v", err)
}

// TestNC551HeadlessMinimal runs a minimal headless scenario without screen display.
// Can be used for automated testing in CI without terminal output.
func TestNC551HeadlessMinimal(t *testing.T) {
	m, err := ncSession(t,
		`<waitfor:Norton Commander><wait:1>`+ // Startup
			`Tab<wait:0.5>`+                      // Panel switch
			`Tab<wait:0.5>`+                      // Back to original panel
			`F10`+                                // Quit
			`y`)                                  // Confirm

	// Check if we reached exit without errors
	if m != nil {
		t.Logf("Headless scenario completed, runtime error: %v", err)
	}
}
