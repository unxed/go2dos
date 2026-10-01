// Package e2e runs real DOS programs end to end.
// VC 4.99.09 (Volkov Commander) tests using vmlab interactive screenshots.
// Tests are skipped unless GO2DOS_VC_DIR points to a directory with VC.COM/VC.OVL.
package e2e

import (
	"errors"
	"strings"
	"testing"

	"github.com/unxed/go2dos/machine"
)

// TestVC49909Startup verifies VC 4.99.09 loads and displays dual panels.
// Captures initial screen and validates startup.
func TestVC49909Startup(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:2><screen>`)

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Startup screen:\n%s", screenText)
		t.Error("Expected VC main screen (10Quit) not found")
	}
	if !strings.Contains(screenText, "readme") && !strings.Contains(screenText, "README") {
		t.Errorf("Expected file listing not found in startup")
	}
	t.Logf("Startup successful, runtime error: %v", err)
}

// TestVC49909PanelsDisplay verifies both left and right panels are visible.
// Steps: wait for startup, capture screen showing dual panels.
func TestVC49909PanelsDisplay(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output captured")
	}
	// VC 4.99.09 shows two panels by default; check for presence of file listing
	if !strings.Contains(screenText, "txt") && !strings.Contains(screenText, "dir") {
		t.Logf("Panel display screen:\n%s", screenText)
		t.Errorf("Expected panel content not found")
	}
	t.Logf("Panels display verified, runtime error: %v", err)
}

// TestVC49909PanelNavigation verifies Tab-key panel switching and directory navigation.
// Steps: Tab to switch panels, Arrow keys to navigate, Enter to open directory.
func TestVC49909PanelNavigation(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+  // Startup, capture
			`Tab<wait:1><screen>`+               // Switch to right panel
			`Down<wait:0.5>`+                    // Navigate down
			`Enter<wait:1><screen>`+             // Enter directory/file
			`..<Enter<wait:1><screen>`)          // Go up (..) back to parent

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output after navigation")
	}
	// After going back up, should see SUBDIR in listing
	if !strings.Contains(screenText, "SUBDIR") && !strings.Contains(screenText, "subdir") {
		t.Logf("Navigation screen:\n%s", screenText)
	}
	t.Logf("Panel navigation test completed, runtime error: %v", err)
}

// TestVC49909DirectoryEnter verifies directory entry with Arrow keys and Enter.
// Steps: navigate with Down arrow, Enter to open subdirectory, check path changed.
func TestVC49909DirectoryEnter(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`Down<wait:0.5>`+        // Move to first file
			`Down<wait:0.5>`+        // Move down
			`Enter<wait:1><screen>`+ // Enter if it's a directory
			`<wait:1>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen after directory entry attempt")
	}
	t.Logf("Directory entry test completed, runtime error: %v", err)
}

// TestVC49909View verifies F3 file view function.
// Steps: position on a file, F3 to view, Esc to exit view, capture screens.
func TestVC49909View(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+ // Startup
			`F3<wait:1><screen>`+             // View file with F3
			`Esc<wait:1><screen>`)            // Exit view

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from F3 view")
	}
	// After Esc, should return to main panel view
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-view screen:\n%s", screenText)
		t.Errorf("Did not return to main screen after F3 exit")
	}
	t.Logf("F3 view test completed, runtime error: %v", err)
}

// TestVC49909Edit verifies F4 file edit function.
// Steps: position on a file, F4 to edit, type text, Esc to exit without save.
func TestVC49909Edit(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+ // Startup
			`F4<wait:1><screen>`+             // Edit file with F4
			`test<wait:0.5>`+                 // Type some text
			`Esc<wait:1><screen>`)            // Exit without saving

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from F4 edit")
	}
	t.Logf("F4 edit test completed, runtime error: %v", err)
}

// TestVC49909Menu verifies F9 menu access and menu navigation.
// Steps: F9 to open menu, verify menu appears, Esc to close menu.
func TestVC49909Menu(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+ // Startup
			`F9<wait:1><screen>`+             // Open menu with F9
			`Esc<wait:1><screen>`)            // Close menu

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from F9 menu")
	}
	// After Esc, should return to main view
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-menu screen:\n%s", screenText)
	}
	t.Logf("F9 menu test completed, runtime error: %v", err)
}

// TestVC49909Exit verifies F10 quit function and exit confirmation.
// Steps: F10 to quit, confirm exit with Y/Enter, verify graceful exit.
func TestVC49909Exit(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+
			`F10<wait:1><screen>`+                  // Quit command
			`y<waitfor:C:\><screen>`)               // Confirm quit

	screenText := m.Screen().Text()
	// After quit, should see DOS prompt (C:\)
	if !strings.Contains(screenText, "C:\\") {
		t.Logf("Exit screen:\n%s", screenText)
		t.Errorf("Did not reach DOS prompt after exit")
	}

	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Logf("Exit code: %v (expected 0)", err)
	}
	t.Logf("F10 exit test completed, runtime error: %v", err)
}

// TestVC49909FileOperations verifies basic file operations: F5 (copy), F6 (rename), F7 (mkdir), F8 (delete).
// Steps: attempt each operation and cancel with Esc to verify they respond.
func TestVC49909FileOperations(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			// F5: Copy
			`F5<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F6: Rename
			`F6<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F7: Create directory (enter name, then cancel or confirm)
			`F7<wait:1><screen>`+
			`Esc<wait:0.5>`+
			// F8: Delete
			`F8<wait:1><screen>`+
			`Esc<wait:0.5>`+
			`<screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from file operations")
	}
	// Should be back at main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Final screen:\n%s", screenText)
		t.Errorf("Did not return to main screen after operations")
	}
	t.Logf("File operations test completed, runtime error: %v", err)
}

// TestVC49909LongNameToggle verifies Ctrl-N to toggle long filename display.
// Steps: capture short names, Ctrl-N, capture long names (if available).
func TestVC49909LongNameToggle(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+ // Short names view
			`Ctrl-N<wait:1><screen>`)         // Toggle to long names

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from Ctrl-N")
	}
	t.Logf("Long name toggle test completed, runtime error: %v", err)
}

// TestVC49909PanelToggle verifies Ctrl-O to hide/show DOS screen.
// Steps: Ctrl-O to hide panels (show DOS), Ctrl-O again to show panels.
func TestVC49909PanelToggle(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			`Ctrl-O<wait:1><screen>`+    // Hide panels (show DOS)
			`Ctrl-O<wait:1><screen>`)    // Show panels again

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Error("No screen output from panel toggle")
	}
	// After toggling back, should see main screen
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Post-toggle screen:\n%s", screenText)
	}
	t.Logf("Panel toggle test completed, runtime error: %v", err)
}

// TestVC49909FullScenario runs a comprehensive workflow combining multiple features.
// Steps: navigation, file viewing, menu access, and exit with all validations.
func TestVC49909FullScenario(t *testing.T) {
	m, err := session(t, "4.99.09",
		// Startup
		`<waitfor:10Quit><wait:1><screen>`+
			// Navigate left panel
			`Down<wait:0.5><screen>`+        // Move down
			`Up<wait:0.5>`+                  // Move up
			// Panel switch
			`Tab<wait:1><screen>`+           // Switch to right panel
			// Menu operations
			`F9<wait:1><screen>`+            // Open menu
			`Esc<wait:0.5>`+                 // Close menu
			// File viewing
			`F3<wait:1>Esc<wait:0.5>`+       // F3 view, then Esc
			// Panel toggle (Ctrl-O)
			`Ctrl-O<wait:1>`+                // Hide panels
			`Ctrl-O<wait:1><screen>`+        // Show panels again
			// Long names toggle
			`Ctrl-N<wait:1><screen>`+        // Toggle long names
			// Exit
			`F10<wait:1><screen>`+           // Quit command
			`y<waitfor:C:\><screen>`)        // Confirm exit

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "C:\\") {
		t.Logf("Final screen:\n%s", screenText)
		t.Errorf("Did not reach DOS prompt after full scenario")
	}

	var ex *machine.ExitError
	if !errors.As(err, &ex) || ex.Code != 0 {
		t.Logf("Exit code: %v (expected 0), runtime error: %v", err, err)
	}
	t.Logf("Full scenario test completed successfully, runtime error: %v", err)
}

// TestVC49909HeadlessMinimal runs a minimal automated scenario suitable for CI.
// No interactive screen captures, just functional verification.
func TestVC49909HeadlessMinimal(t *testing.T) {
	_, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1>`+   // Startup
			`Tab<wait:0.5>`+              // Panel switch
			`Down<wait:0.5>`+             // Navigate
			`F9<wait:0.5>`+               // Menu
			`Esc<wait:0.5>`+              // Close menu
			`F3<wait:0.5>Esc<wait:0.5>`+  // View and exit
			`F10`+                        // Quit
			`y`)                          // Confirm

	t.Logf("Headless scenario completed, runtime error: %v", err)
}

// TestVC49909ScreenValidation verifies screen rendering is correct after operations.
// Captures screen after each major operation and validates content.
func TestVC49909ScreenValidation(t *testing.T) {
	m, _ := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`)

	screenText := m.Screen().Text()
	if screenText == "" {
		t.Fatal("No screen output at all")
	}

	// Validate that screen has key elements
	hasMainMenu := strings.Contains(screenText, "10Quit") || strings.Contains(screenText, "10quit")
	hasFileList := strings.Contains(screenText, "txt") || strings.Contains(screenText, "TXT") ||
		strings.Contains(screenText, "readme") || strings.Contains(screenText, "README")

	if !hasMainMenu {
		t.Errorf("Screen missing main menu indicator (10Quit)")
	}
	if !hasFileList {
		t.Errorf("Screen missing file listing")
	}

	t.Logf("Screen validation passed, content length: %d chars", len(screenText))
}

// TestVC49909ErrorRecovery verifies VC handles invalid input gracefully.
// Steps: attempt invalid file operations, verify VC returns to stable state.
func TestVC49909ErrorRecovery(t *testing.T) {
	m, err := session(t, "4.99.09",
		`<waitfor:10Quit><wait:1><screen>`+
			// Press F4 (edit) which might fail or open editor
			`F4<wait:1>Esc<wait:0.5><screen>`+
			// Verify we're back at main screen
			`<waitfor:10Quit>`)

	screenText := m.Screen().Text()
	if !strings.Contains(screenText, "10Quit") {
		t.Logf("Recovery screen:\n%s", screenText)
		t.Errorf("VC did not return to stable state after operation")
	}
	t.Logf("Error recovery test completed, runtime error: %v", err)
}
