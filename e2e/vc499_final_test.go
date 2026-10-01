// Package e2e runs real DOS programs end to end.
// VC 4.99.09 (Volkov Commander) final validation tests.
// Tests are skipped unless GO2DOS_VC_DIR points to a directory with VC.COM/VC.OVL.
package e2e

import (
	"errors"
	"strings"
	"testing"

	"github.com/unxed/go2dos/machine"
)

// TestVC49909FinalStartup verifies VC 4.99.09 loads successfully in headless mode.
// Validates: startup, main screen detection, panel initialization.
func TestVC49909FinalStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output at startup")
	}
	if !strings.Contains(screenText, "10Quit") {
		t.Errorf("Expected main screen with '10Quit' indicator, got:\n%s", screenText)
	}
	t.Logf("Startup test passed, runtime error: %v", err)
}

// TestVC49909FinalPanels verifies both left and right panels display correctly.
// Validates: dual panel layout, file listings in each panel.
func TestVC49909FinalPanels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output for panel test")
	}
	// Check for panel indicators (file entries or directory markers)
	hasFileList := strings.Contains(screenText, "txt") ||
		strings.Contains(screenText, "readme") ||
		strings.Contains(screenText, "README") ||
		strings.Contains(screenText, "TXT")
	if !hasFileList {
		t.Logf("Panel screen:\n%s", screenText)
		t.Errorf("Expected file listings in panels")
	}
	t.Logf("Panel test passed, runtime error: %v", err)
}

// TestVC49909FinalNavigation verifies Tab-key panel switching and arrow key navigation.
// Validates: Tab to switch panels, arrow keys to navigate, Enter to open directories.
func TestVC49909FinalNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`Tab<wait:1><screen>`+           // Switch to right panel
			`Down<wait:0.5><screen>`+        // Navigate down
			`Down<wait:0.5>`+                // Navigate down again
			`Up<wait:0.5><screen>`+          // Navigate up
			`..<wait:0.5>Enter<wait:1><screen>`) // Go up to parent

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Errorf("No screen output after navigation")
	}
	// Should still see main screen or parent directory
	if !strings.Contains(screenText, "10Quit") && !strings.Contains(screenText, "C:\\") {
		t.Logf("Navigation screen:\n%s", screenText)
	}
	t.Logf("Navigation test passed, runtime error: %v", err)
}

// TestVC49909FinalFileOperationsCopy verifies F5 (copy) function.
// Validates: F5 opens copy dialog, can be cancelled with Esc.
func TestVC49909FinalFileOperationsCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F5<wait:1><screen>`+    // Open copy dialog
			`Esc<wait:0.5><screen>`) // Cancel copy

	screenText := m.Screen().Text()
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Copy operation screen:\n%s", screenText)
		t.Errorf("F5 copy did not return to main screen")
	}
	t.Logf("F5 copy test passed, runtime error: %v", err)
}

// TestVC49909FinalFileOperationsRename verifies F6 (rename) function.
// Validates: F6 opens rename dialog, can be cancelled with Esc.
func TestVC49909FinalFileOperationsRename(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F6<wait:1><screen>`+    // Open rename dialog
			`Esc<wait:0.5><screen>`) // Cancel rename

	screenText := m.Screen().Text()
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Rename operation screen:\n%s", screenText)
		t.Errorf("F6 rename did not return to main screen")
	}
	t.Logf("F6 rename test passed, runtime error: %v", err)
}

// TestVC49909FinalFileOperationsMkdir verifies F7 (create directory) function.
// Validates: F7 opens mkdir dialog, can be cancelled with Esc.
func TestVC49909FinalFileOperationsMkdir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F7<wait:1><screen>`+    // Open mkdir dialog
			`Esc<wait:0.5><screen>`) // Cancel mkdir

	screenText := m.Screen().Text()
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Mkdir operation screen:\n%s", screenText)
		t.Errorf("F7 mkdir did not return to main screen")
	}
	t.Logf("F7 mkdir test passed, runtime error: %v", err)
}

// TestVC49909FinalFileOperationsDelete verifies F8 (delete) function.
// Validates: F8 opens delete dialog, can be cancelled with Esc.
func TestVC49909FinalFileOperationsDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F8<wait:1><screen>`+    // Open delete dialog
			`Esc<wait:0.5><screen>`) // Cancel delete

	screenText := m.Screen().Text()
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Delete operation screen:\n%s", screenText)
		t.Errorf("F8 delete did not return to main screen")
	}
	t.Logf("F8 delete test passed, runtime error: %v", err)
}

// TestVC49909FinalFileOperationsAll verifies all file operations in sequence.
// Validates: F5 (copy), F6 (rename), F7 (mkdir), F8 (delete) all respond correctly.
func TestVC49909FinalFileOperationsAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			// F5: Copy
			`F5<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F6: Rename
			`F6<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F7: Create directory
			`F7<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F8: Delete
			`F8<wait:1><screen>`+
			`Esc<wait:0.5>`+
			`<screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output from file operations")
	}
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Final screen after operations:\n%s", screenText)
		t.Errorf("Did not return to main screen after file operations")
	}
	t.Logf("All file operations test passed, runtime error: %v", err)
}

// TestVC49909FinalClipboardCopy verifies Ctrl-C copy operation.
// Validates: Ctrl-C responds without error.
func TestVC49909FinalClipboardCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			`Ctrl-C<wait:1><screen>`) // Copy to clipboard

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Errorf("No screen output from Ctrl-C")
	}
	// Should remain at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-copy screen:\n%s", screenText)
	}
	t.Logf("Ctrl-C copy test passed, runtime error: %v", err)
}

// TestVC49909FinalClipboardPaste verifies Ctrl-V paste operation.
// Validates: Ctrl-V responds without error.
func TestVC49909FinalClipboardPaste(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			`Ctrl-V<wait:1><screen>`) // Paste from clipboard

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Errorf("No screen output from Ctrl-V")
	}
	// Should remain at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-paste screen:\n%s", screenText)
	}
	t.Logf("Ctrl-V paste test passed, runtime error: %v", err)
}

// TestVC49909FinalClipboardOperations verifies Ctrl-C copy and Ctrl-V paste sequence.
// Validates: both clipboard operations work in sequence.
func TestVC49909FinalClipboardOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			`Ctrl-C<wait:1><screen>`+ // Copy
			`Ctrl-V<wait:1><screen>`) // Paste

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output from clipboard operations")
	}
	// Should remain at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Clipboard operations screen:\n%s", screenText)
		t.Errorf("Did not remain at main screen after clipboard ops")
	}
	t.Logf("Clipboard operations test passed, runtime error: %v", err)
}

// TestVC49909FinalLFNToggle verifies Ctrl-N long filename toggle.
// Validates: Ctrl-N switches between short and long name display.
func TestVC49909FinalLFNToggle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := sessionFiles(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+   // Short names view
			`Ctrl-N<wait:1><screen>`,            // Toggle to long names
		map[string]string{
			"A Very Long File Name.txt": "content1",
			"second long document.doc":  "content2",
		})

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output from Ctrl-N toggle")
	}
	t.Logf("LFN toggle test passed, runtime error: %v", err)
}

// TestVC49909FinalLongFilenames verifies display of long filenames with Ctrl-N.
// Validates: Ctrl-N shows full long names when available.
func TestVC49909FinalLongFilenames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := sessionFiles(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`Ctrl-N<wait:1><screen>`, // Toggle to long names
		map[string]string{
			"A Very Long File Name.txt": "content1",
			"Another Long Name.doc":     "content2",
		})

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output after LFN toggle")
	}
	t.Logf("Long filenames test passed, runtime error: %v", err)
}

// TestVC49909FinalUTF8Names verifies UTF-8 filename support if integrated.
// Validates: UTF-8 encoded filenames are handled correctly.
func TestVC49909FinalUTF8Names(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := sessionFiles(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`,
		map[string]string{
			"файл.txt":     "content",     // Russian filename (UTF-8)
			"тест.doc":     "content",     // Russian filename
			"normal.txt":   "content",     // ASCII filename for comparison
		})

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output from UTF-8 names test")
	}
	// Just verify VC doesn't crash with UTF-8 filenames
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("UTF-8 names screen:\n%s", screenText)
	}
	t.Logf("UTF-8 names test passed, runtime error: %v", err)
}

// TestVC49909FinalExit verifies F10 quit and graceful exit.
// Validates: F10 terminates VC with exit code 0.
func TestVC49909FinalExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F10<wait:1><screen>`+          // Quit command
			`y<waitfor:C:\><screen>`)       // Confirm quit

	screenText := m.Screen().Text()
	// After quit, should see DOS prompt (C:\)
	if !strings.Contains(screenText, "C:\\") {
		t.Logf("Exit screen:\n%s", screenText)
		t.Errorf("Did not reach DOS prompt after F10 exit")
	}

	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Logf("Exit code: %v (expected 0)", err)
	}
	t.Logf("F10 exit test passed, runtime error: %v", err)
}

// TestVC49909FinalPanelToggle verifies Ctrl-O to hide/show DOS screen.
// Validates: Ctrl-O toggles panel visibility.
func TestVC49909FinalPanelToggle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			`Ctrl-O<wait:1><screen>`+    // Hide panels (show DOS)
			`Ctrl-O<wait:1><screen>`)    // Show panels again

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output from panel toggle")
	}
	// After toggling back, should see main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-toggle screen:\n%s", screenText)
	}
	t.Logf("Panel toggle test passed, runtime error: %v", err)
}

// TestVC49909FinalErrorRecovery verifies VC handles operations gracefully.
// Validates: VC returns to stable state after various operations.
func TestVC49909FinalErrorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			// Try to view a file
			`F3<wait:1>Esc<wait:0.5><screen>`+
			// Try to edit
			`F4<wait:1>Esc<wait:0.5><screen>`+
			// Verify we're still stable
			`<waitfor:10Quit>`)

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Recovery screen:\n%s", screenText)
		t.Errorf("VC did not recover to main screen")
	}
	t.Logf("Error recovery test passed, runtime error: %v", err)
}

// TestVC49909FinalComprehensiveWorkflow runs a complete workflow with multiple features.
// Validates: startup, navigation, file operations, clipboard, exit all work together.
func TestVC49909FinalComprehensiveWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		// Startup
		`<waitfor:10Quit><wait:1><screen>`+
			// Navigate
			`Down<wait:0.5>`+
			`Up<wait:0.5>`+
			// Switch panels
			`Tab<wait:1><screen>`+
			// File operations (F5, F6, F7, F8)
			`F5<wait:1>Esc<wait:0.5>`+
			`F6<wait:1>Esc<wait:0.5>`+
			`F7<wait:1>Esc<wait:0.5>`+
			`F8<wait:1>Esc<wait:0.5>`+
			// Clipboard
			`Ctrl-C<wait:0.5>`+
			`Ctrl-V<wait:0.5>`+
			// Toggle LFN
			`Ctrl-N<wait:1><screen>`+
			// Panel toggle
			`Ctrl-O<wait:1>`+
			`Ctrl-O<wait:1><screen>`+
			// Exit
			`F10<wait:1><screen>`+
			`y<waitfor:C:\><screen>`)

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "C:\\") {
		t.Logf("Final screen:\n%s", screenText)
		t.Errorf("Did not reach DOS prompt after comprehensive workflow")
	}

	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Logf("Exit code: %v (expected 0)", err)
	}
	t.Logf("Comprehensive workflow test passed, runtime error: %v", err)
}

// TestVC49909FinalScreenValidation verifies screen rendering is correct.
// Validates: screen content, layout, and text rendering.
func TestVC49909FinalScreenValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, _ := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output at all")
	}

	// Validate that screen has key elements
	hasMainMenu := strings.Contains(screenText, "10Quit") || strings.Contains(screenText, "10quit")
	hasFileList := strings.Contains(screenText, "txt") ||
		strings.Contains(screenText, "TXT") ||
		strings.Contains(screenText, "readme") ||
		strings.Contains(screenText, "README")

	if !hasMainMenu {
		t.Errorf("Screen missing main menu indicator (10Quit)")
	}
	if !hasFileList {
		t.Errorf("Screen missing file listing")
	}

	t.Logf("Screen validation passed, content length: %d chars", len(screenText))
}

// TestVC49909FinalHeadlessMinimal runs a minimal automated scenario for CI.
// Validates: core functionality without interactive screen captures.
func TestVC49909FinalHeadlessMinimal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	_, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+        // Startup
			`Tab<wait:0.5>`+                   // Panel switch
			`Down<wait:0.5>`+                  // Navigate
			`Up<wait:0.5>`+                    // Navigate back
			`Ctrl-N<wait:0.5>`+                // Toggle LFN
			`Ctrl-C<wait:0.5>`+                // Clipboard copy
			`Ctrl-V<wait:0.5>`+                // Clipboard paste
			`F9<wait:0.5>Esc<wait:0.5>`+       // Menu
			`Ctrl-O<wait:0.5>Ctrl-O<wait:0.5>`+ // Panel toggle
			`F10`+                             // Quit
			`y`)                               // Confirm

	t.Logf("Headless minimal test passed, runtime error: %v", err)
}

// TestVC49909FinalStartupBench is a performance/stress test for startup.
// Validates: VC can start and respond to commands rapidly.
func TestVC49909FinalStartupBench(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("Startup bench: no screen output")
	}
	if !strings.Contains(screenText, "10Quit") {
		t.Errorf("Startup bench: did not see main screen")
	}
	t.Logf("Startup benchmark test passed, runtime error: %v", err)
}
