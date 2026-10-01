# Norton Commander 5.51: Complete Testing Scenarios

This document describes the comprehensive e2e testing scenarios for Norton Commander 5.51 (the last DOS version from 1998) running under go2dos.

## Overview

Norton Commander is a legendary dual-pane file manager for DOS. The testing framework verifies that go2dos correctly emulates the runtime environment, interrupt handlers, and UI interactions required for NC to function properly.

**Key files:**
- `e2e/nc5_test.go` — Go test suite with individual scenario tests
- `NC-TESTING.md` — Manual testing checklist and scenario formats
- This document — detailed scenario specifications

**Test setup:**
- Environment variable: `GO2DOS_NC_DIR` — path to directory containing `NC.EXE` or `NCMAIN.EXE`
- Test framework: Go `testing` package with `github.com/unxed/go2dos/keys` for scripting
- Timeout: 30 seconds per test
- Codepage: 437 (IBM PC ASCII)

## Test Scenarios

### 1. Startup Test: `TestNC551Startup`

**Purpose:** Verify Norton Commander loads, displays the logo, and renders initial panels.

**Scenario:**
```
<waitfor:Norton Commander>    # Wait for application window
<wait:2>                      # Allow time for full initialization
<screen>                      # Capture initial state
```

**Expected results:**
- Text "Norton Commander" or "norton" appears on screen
- Two file panels visible with directory listings
- Menu bar (F1–F10) shown at bottom
- Current path displayed in panel headers
- No error messages

**How it works:**
- `<waitfor:...>` waits for specific text to appear (non-blocking)
- `<wait:2>` adds a 2-second delay for rendering
- `<screen>` captures a snapshot for verification

### 2. Panel Navigation Test: `TestNC551PanelNavigation`

**Purpose:** Verify basic navigation: panel switching (Tab), directory entry (Enter), and going back (..).

**Scenario:**
```
<waitfor:Norton Commander>    # Startup
<wait:2>
<screen>                      # Initial state: two panels
Tab                           # Switch to right panel
<wait:1>
<screen>                      # Verify panel focus changed
Enter                         # Enter first directory or file
<wait:1>
<screen>                      # Inside directory
..                            # Type ".." (parent directory entry)
Enter                         # Go up one level
<wait:1>
<screen>                      # Back to parent
```

**Expected results:**
- Tab toggles focus between left and right panels (visual indicator changes)
- Enter navigates into directories (path changes in panel header)
- .. entry moves to parent directory
- Path in panel header updates correctly after each navigation
- Both panels remain synchronized (when one changes, status updates)

### 3. File Viewing Test: `TestNC551ViewFile`

**Purpose:** Verify F3 (View) opens a file viewer and Esc returns to panels.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
F3                            # Open file viewer
<wait:1>
<screen>                      # Capture viewer state
Esc                           # Exit viewer
<wait:1>
<screen>                      # Back in Norton Commander
```

**Expected results:**
- F3 opens a file viewer window (typically NCVIEW or internal viewer)
- Viewer displays text file content correctly
- Esc key closes viewer without saving and returns to NC
- Panels are intact after returning

**Files used:** `README.TXT` (pre-created in temp directory)

### 4. File Editing Test: `TestNC551EditFile`

**Purpose:** Verify F4 (Edit) launches editor, allows input, and Esc exits without saving.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
F4                            # Open editor
<wait:1>
test                          # Type sample text
<wait:1>
Esc                           # Exit without saving
<wait:1>
<screen>                      # Back in Norton Commander
```

**Expected results:**
- F4 opens external editor (typically NCEDIT.EXE or EDIT.COM)
- Editor accepts keyboard input
- Esc closes editor without saving changes
- Returns to Norton Commander panels
- Original file content unchanged (not saved)

**Note:** This tests the EXEC interrupt (INT 21h AX=4B00h) and child process management.

### 5. File Operations Test: `TestNC551FileOperations`

**Purpose:** Verify F5 (Copy), F6 (Rename), F7 (MkDir), F8 (Delete) prompts appear and operations can be canceled.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
F5                            # Copy dialog
<wait:1>
<screen>                      # Capture dialog
Esc                           # Cancel copy
<wait:1>
F6                            # Rename dialog
<wait:1>
<screen>                      # Capture dialog
Esc                           # Cancel rename
<wait:1>
F7                            # Create directory
<wait:1>
<screen>                      # Capture mkdir prompt
NEWDIR                        # Type directory name
Enter                         # Confirm
<wait:1>
<screen>                      # Verify directory created
Esc                           # Return from operation
<wait:1>
F8                            # Delete file/directory
<wait:1>
<screen>                      # Capture delete prompt
Esc                           # Cancel delete
<wait:1>
```

**Expected results:**
- F5, F6, F7, F8 open appropriate dialogs
- Dialogs display destination/name/confirmation prompts
- Esc cancels operations (no changes applied)
- F7 creates a new directory `NEWDIR`
- Directory appears in panel listing after creation
- File operations interact correctly with INT 21h file system calls

### 6. Toggle Panels Test: `TestNC551TogglePanels`

**Purpose:** Verify Ctrl-O hides panels to show raw DOS screen, and pressing again restores them.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
Ctrl-O                        # Hide panels (show DOS screen)
<wait:1>
<screen>                      # Capture DOS prompt
Ctrl-O                        # Restore panels
<wait:1>
<screen>                      # Panels back
```

**Expected results:**
- First Ctrl-O clears panels, shows DOS command prompt (C:\>)
- Second Ctrl-O restores NC panels
- No loss of state; directory remains at same location
- Video memory (INT 10h) correctly toggled between text modes

### 7. Menu Test: `TestNC551Menu`

**Purpose:** Verify F9 (Menu) opens the main menu and Esc closes it.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
F9                            # Open main menu
<wait:1>
<screen>                      # Capture menu state
Esc                           # Close menu
<wait:1>
<screen>                      # Back to panels
```

**Expected results:**
- F9 opens vertical menu with options (File, View, Disk, Tree, Tools, Search, Help, Quit)
- Menu is drawn at top or side of screen
- Esc closes menu without executing any action
- Panels reappear unchanged

### 8. Disk Selection Test: `TestNC551SelectDisk`

**Purpose:** Verify Alt-F1 shows drive selection menu and Esc closes it.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
Alt-F1                        # Disk selection for left panel
<wait:1>
<screen>                      # Capture disk menu
Esc                           # Close menu
<wait:1>
<screen>                      # Back to panels
```

**Expected results:**
- Alt-F1 displays drive list (A:, B:, C:, etc.)
- Menu shows available drives detected by BIOS/DOS
- Esc closes menu without changing drives
- Current path in left panel unchanged

**Note:** Tests INT 13h (disk I/O) and BIOS drive detection via INT 10h or direct port access.

### 9. Full Scenario Test: `TestNC551FullScenario`

**Purpose:** Comprehensive workflow combining all major features in sequence.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:2>
<screen>                      # 1. Startup
Tab                           # 2. Panel switching
<wait:1>
<screen>
Enter                         # 3. Enter directory
<wait:1>
<screen>
..                            # 4. Navigate back
Enter
<wait:1>
<screen>
F9                            # 5. Menu
<wait:1>
<screen>
Esc
F5                            # 6. Copy operation
<wait:1>
Esc
<wait:1>
Tab                           # 7. Switch panel
<wait:1>
Shift-F5                       # 8. Extended operation
<wait:1>
Esc
<wait:1>
Ctrl-O                        # 9. Toggle view
<wait:1>
Ctrl-O
<wait:1>
Alt-F1                        # 10. Disk menu
<wait:1>
<screen>
Esc
<wait:1>
F10                           # 11. Quit
<wait:1>
<screen>
y                             # 12. Confirm exit
<waitfor:C:\>                 # 13. Back to DOS prompt
```

**Expected results:**
- All operations complete successfully
- No crashes or hangs
- Final screen shows DOS prompt (C:\>)
- All INT 21h, INT 10h, INT 16h (keyboard) operations work correctly
- File system state consistent throughout

### 10. Headless Minimal Test: `TestNC551HeadlessMinimal`

**Purpose:** Minimal automated scenario suitable for CI without terminal display.

**Scenario:**
```
<waitfor:Norton Commander>
<wait:1>                      # Quick startup
Tab                           # Switch panel
<wait:0.5>
Tab                           # Back to original
<wait:0.5>
F10                           # Quit
y                             # Confirm
```

**Expected results:**
- Application loads and responds to input
- Panel operations work with minimal delays
- Clean exit with confirmation
- Can run in headless mode (no terminal needed)
- Useful for CI pipelines with tight timeouts

## Implementation Details

### Key Interrupt Handlers Required

1. **INT 10h (Video BIOS):**
   - AH=00h — Set video mode (text/graphics modes 0–7)
   - AH=02h — Set cursor position
   - AH=03h — Get cursor position
   - AH=06h — Scroll window up
   - AH=09h — Write character and attribute
   - AH=0Ah — Write character only
   - AH=0Fh — Get current video mode

2. **INT 16h (Keyboard BIOS):**
   - AH=00h — Read character from keyboard
   - AH=01h — Check if character available
   - AH=03h — Set repeat rate (BIOS extensions)
   - Extended scan codes for F1–F10, arrows, Ctrl/Shift modifiers

3. **INT 21h (DOS):**
   - AX=0B00h — Check if input pending
   - AX=0Ch — Clear buffer and read character
   - AX=1300h — Read from pipe (clipboard via WinOldAp)
   - AX=1400h — Write to pipe (clipboard via WinOldAp)
   - File operations: AX=3Dxx (Open), 3Eh (Close), 3Fh (Read), 40h (Write)
   - Directory: AX=39h (MkDir), 3Ah (RmDir), 3Bh (ChDir), 4Eh (FindFirst), 4Fh (FindNext)
   - Attributes: AX=4301h (Set), 4300h (Get)
   - EXEC: AX=4B00h (Execute program)
   - Disk: AX=440Dh (Generic I/O control — drive info)
   - LFN (Long File Names): AX=71xx (if supported)

4. **INT 2Fh (Multiplex):**
   - AX=17xxh (WinOldAp — clipboard)
   - AX=1680h (Check for Windows — typically not installed)
   - AX=4Dxxh (Check environment variables)

5. **INT 33h (Mouse, optional):**
   - AX=00h — Reset mouse
   - AX=01h — Show cursor
   - AX=02h — Hide cursor
   - AX=03h — Get position and button status
   - AX=04h — Set position

### Script Syntax Reference

The scenario scripts use a simple format parsed by `github.com/unxed/go2dos/keys.Parse`:

| Syntax | Meaning |
|--------|---------|
| `<waitfor:TEXT>` | Block until TEXT appears on screen (case-sensitive) |
| `<wait:N>` | Sleep for N seconds (floating point OK: `<wait:0.5>`) |
| `<screen>` | Capture current screen state for verification |
| `Enter`, `Tab`, `Esc` | Standard keys by name |
| `F1`–`F10` | Function keys |
| `Ctrl-X`, `Shift-X`, `Alt-X` | Modifiers (X = any key) |
| `..` | Literal text (typed as-is) |
| Plain text | Typed character-by-character |

### Running Tests

**With Norton Commander installed:**
```sh
export GO2DOS_NC_DIR="/path/to/NC/directory"
go test -v -run TestNC551 ./e2e
```

**Run specific test:**
```sh
go test -v -run TestNC551Startup ./e2e
```

**Run all NC tests:**
```sh
go test -v -run TestNC551 ./e2e
```

### Expected Runtime

- Per-test timeout: 30 seconds (should complete in 2–5 seconds typically)
- Full suite (10 tests): ~60 seconds
- Headless variant (`TestNC551HeadlessMinimal`): ~5 seconds, suitable for CI

## Files and Directories

**Test environment setup (in temp directory):**
- `NC.EXE` / `NCMAIN.EXE` — Norton Commander executable (copied from `GO2DOS_NC_DIR`)
- `README.TXT` — Sample text file
- `TEST.DOC` — Test document
- `SAMPLE.BAT` — Sample batch file
- `SUBDIR/FILE.TXT` — File in subdirectory (for navigation tests)

## Troubleshooting

### Test Skipped: "GO2DOS_NC_DIR not set"
Set the environment variable to point to your Norton Commander directory:
```sh
export GO2DOS_NC_DIR="$HOME/nc/5.51"
go test -v -run TestNC551 ./e2e
```

### Test Hangs on `<waitfor:...>`
- Text may not appear due to color attributes or encoding issues
- Capture raw screen: check `machine.Screen().Text()` in debugger
- Try less specific text: `<waitfor:Norton>` instead of full string
- Increase timeout in test if needed

### File Operations Don't Work
- Verify INT 21h handlers are implemented (check `hle/int21.go`)
- Check `e2e` log output for unsupported function errors
- Common issues: LFN (71xx) not implemented, or incorrect Find{First,Next}

### Keyboard Input Not Recognized
- Verify `keys.Parse()` recognizes key names from test script
- Check `keys/` package for key name definitions
- Ensure INT 16h (keyboard) is properly implemented in BIOS layer

## Related Documents

- `NC-TESTING.md` — Manual testing checklist (for human testers)
- `NC-REQUIREMENTS.md` — Feature matrix and version differences
- `DESIGN.md` §20 — Architecture notes on Norton Commander support
- `VC-BUILD.md` — How to build and patch VC (applies to NC similarly)

## Version Compatibility

These scenarios target **Norton Commander 5.51** (1998, last DOS version):
- Executable: `NC.EXE` (~4 KB, loader) + `NCMAIN.EXE` (~228 KB, main code)
- DOS version: 3.30 or later
- Processor: 8086 or better
- Memory: 512 KB minimum

**Other versions (for reference):**
- **5.0 (1995)**: Same binary layout, simpler feature set
- **4.0 (1993)**: Reduced feature set, good for debugging
- **3.0 (1989)**: Simplest version, good for basic testing

Tests should adapt easily to other versions by changing executable names and expected text patterns.

## Future Enhancements

1. **Archive support:** Add tests for ZIP/ARC viewing and extraction
2. **Advanced menus:** Test Tree view, disk info, search functionality
3. **Configuration:** Test NC.INI parsing and menu customization
4. **Long file names:** If LFN support is added to go2dos
5. **Clipboard:** Integration with host clipboard (T06 feature)
6. **Mouse support:** Full mouse operation testing
7. **Extended video modes:** 43/50-line mode switching

## Summary

The NC5 test suite provides comprehensive coverage of Norton Commander 5.51 functionality in go2dos. Each test is independent (can run in any order) and self-contained (creates its own test environment). The scenarios follow the manual testing checklist in `NC-TESTING.md` but are automated for CI/CD integration.

Success criteria: All tests pass without timeout or crash, final screen shows clean DOS exit (C:\>), and file operations work correctly.
