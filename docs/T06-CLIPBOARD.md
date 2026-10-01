# T06: Clipboard and Terminal Integration

Status: **Complete**

Date: 2026-10-01

## Overview

Task T06 implements clipboard integration and terminal-based text selection in the go2dos frontend. Users can copy text from the screen using keyboard shortcuts (Shift+Arrow) and paste from the host clipboard using Ctrl-V.

## Features Implemented

### 1. Text Selection Interface (Commit 1/8)
**File**: `dos/select.go`

- **TextSelection API**: Represents a rectangular selection with start (StartX, StartY) and end (EndX, EndY) coordinates
- **GetSelectedText()**: Extracts text from screen snapshot
  - Handles single-line and multi-line selections
  - Properly clamps coordinates to screen bounds
  - Removes trailing whitespace from each line
  - Returns text with newlines between selected lines
- **IsValid()** / **IsEmpty()**: Check selection state
- **SetFromCoords()**: Programmatic coordinate setting
- Test coverage: `dos/select_test.go` (7 test cases)

### 2. Paste from Selection to Clipboard (Commit 2/8)
**File**: `dos/select.go`

- **PasteFromSelection()**: Puts selected text into DOS WinOldAp clipboard
  - Validates clipboard is open and format is supported (CF_TEXT=1, CF_OEMTEXT=7)
  - Handles multi-line selections with proper line breaks
  - Returns byte count written to clipboard
  - Error handling: ErrClipboardNotOpen, ErrUnsupportedFormat
- Test coverage: `dos/select_test.go` (4 test cases)
  - Success case with format validation
  - Rejection when clipboard closed
  - Unsupported format rejection
  - Empty/whitespace-only selections

### 3. Frontend Selection Handler (Commit 3/8)
**File**: `cmd/go2dos/selection.go`

- **selectionHandler struct**: Manages text selection state in terminal frontend
  - Mode tracking: off → active → ended
  - Methods:
    - `startSelection(x, y)`: Begin selection at screen coordinates
    - `extendSelection(x, y)`: Extend selection to new coordinates
    - `endSelection()`: Stop active selection
    - `toggleSelectionMode()`: Switch selection mode on/off
    - `getSelectedText()`: Retrieve selected text from current screen
    - `updateScreen(screen)`: Update internal screen snapshot
    - `isActive()`: Check selection mode state
- Test coverage: `cmd/go2dos/selection_test.go` (7 test cases)

### 4. Terminal Integration - Shift+Arrow Selection (Commit 4/8)
**File**: `cmd/go2dos/term.go`

- **termHost struct**: Implements frontend.Host interface with selection support
  - Added fields: `sel` (selectionHandler), `clipboard` (frontend.Clipboard)
- **handleTerminalIntegration()**: Processes terminal integration keys
  - Detects Shift+Arrow scan codes (0x48, 0x50, 0x4B, 0x4D)
  - Manages selection state: off → active → ended
  - Updates selection coordinates based on screen cursor position
  - Returns true for handled keys, false to pass to machine
- **ESC key**: Ends selection and returns to normal mode
- Integration: `termHost.Draw()` updates screen snapshot in selectionHandler

### 5. Paste by Key - Ctrl-V Support (Commit 5/8)
**File**: `cmd/go2dos/term.go`, `frontend/host_clipboard.go`

- **pasteClipboard()**: Reads text from host clipboard and converts to keystrokes
  - Converts text to appropriate DOS keypresses
  - Newlines converted to Enter key presses
  - Handles multi-line text with proper line breaks
  - Sends keystrokes in portions (max 15 per BIOS buffer)
  - 5ms delay between portions to allow processing
- **HostClipboard class**: System clipboard access
  - **GetText()**: Read from system clipboard
  - **SetText()**: Write to system clipboard
  - Platform support:
    - Linux/BSD: xclip, xsel, or wl-paste
    - macOS: pbpaste/pbcopy
    - Windows: PowerShell
  - Thread-safe with mutex protection
  - Graceful fallback on unsupported platforms
- **Ctrl-V Detection**: scan 0x2F with ModCtrl

### 6. Tests (Commit 6/8)
**File**: `cmd/go2dos/term_test.go`, `frontend/host_clipboard_test.go`

Test infrastructure includes:
- MockClipboard: In-memory clipboard for testing
- Unit tests for clipboard operations, key detection
- Regression tests for regular key handling
- Large text pasting (multiple portions)
- Newline to Enter conversion
- Boundary condition handling

### 7. End-to-end Tests (Commit 7/8)
**File**: `e2e/t06_clipboard_test.go`

- TestT06ClipboardIntegration: Clipboard paste via Ctrl-V
- TestT06SelectionKeyboardIntegration: Shift+Arrow selection
- TestT06ClipboardPasteMultiline: Multi-line text with Enter keys

Tests run machine for 2 seconds, verify key injection and state changes.

### 8. Documentation (Commit 8/8)
**File**: `docs/T06-CLIPBOARD.md` (this file)

Complete reference documentation for T06 implementation.

## Architecture

### Data Flow: Text Selection

```
User presses Shift+Arrow
    ↓
inputParser.parse() → bios.KeyEvent with ModLShift
    ↓
termHost.handleTerminalIntegration() detects Shift+Arrow
    ↓
selectionHandler.startSelection() / extendSelection()
    ↓
Selection coordinates updated in TextSelection
    ↓
termHost.Draw() displays selected region (future visual highlighting)
```

### Data Flow: Clipboard Paste

```
User presses Ctrl-V
    ↓
inputParser.parse() → bios.KeyEvent (Scan=0x2F, Mods=ModCtrl)
    ↓
termHost.handleTerminalIntegration() detects Ctrl-V
    ↓
termHost.pasteClipboard() reads HostClipboard.GetText()
    ↓
Text → Keystrokes conversion (Char, Newline → Enter)
    ↓
PushKey() in portions (max 15) with 5ms delays
    ↓
DOS machine receives keystrokes in BIOS buffer
```

### Interfaces

**frontend.Clipboard** (existing, used by both clipboard server and frontend)
```go
type Clipboard interface {
    GetText() (string, error)
    SetText(string) error
}
```

**dos.HostClipboard** (compatibility layer for DOS clipboard server)
```go
type HostClipboard interface {
    GetText() (string, error)
    SetText(string) error
}
```

## Usage

### Terminal Frontend

**Selecting Text:**
1. Press Shift+Up/Down/Left/Right to extend selection
2. Press ESC to end selection and return to normal mode
3. (Future: Mouse selection with visual feedback)

**Pasting Text:**
1. Copy text in host (e.g., `Ctrl-C` in terminal)
2. Press Ctrl-V in go2dos
3. Text is injected as keystrokes to DOS program

### DOS Programs

**Via WinOldAp API (INT 2Fh/17xx):**
- Implemented in `dos/int21.go` (T05 complete)
- Allows DOS programs to access clipboard
- Formats: CF_TEXT (1), CF_OEMTEXT (7)

## Configuration

No special configuration needed. Clipboard support is enabled by default.

**Optional**: Use `frontend.MemoryClipboard` for testing (doesn't access system clipboard)

## Testing

### Unit Tests
```bash
go test ./cmd/go2dos -v
go test ./dos -v
go test ./frontend -v
```

### End-to-End Tests
```bash
go test ./e2e -v -run T06
```

### Manual Testing
1. Run: `./go2dos -keys '@test.keys' program.com`
2. In test.keys: `<key:Shift+Up>`, `<key:Ctrl-V>`
3. Verify: Text selection and pasting work

## Limitations and Future Work

### Current Limitations
- Selection is logical (coordinates only), no visual highlighting on screen
- Paste only supports ASCII and OEM code page characters
- No mouse support for selection (future T14 vtui frontend)
- Single-line / rectangular selection only (no free-form)

### Future Enhancements (T14 frontend)
- Visual selection highlighting (invert colors, highlight bar)
- Mouse-based selection (Shift+click to select)
- vtui integration with GUI support (Wayland, X11, Win32)
- Extended clipboard format support

### Known Issues
- Large pastes (>100 lines) may need longer delays or chunking adjustment
- Some terminals may not support Shift+Arrow properly (configure via keys scripts)
- Windows PowerShell clipboard may fail if forms assembly not available (fallback to copy/paste)

## Integration Points

1. **Terminal Input**: `cmd/go2dos/term.go:inputParser.parse()`
2. **Selection State**: `cmd/go2dos/selection.go:selectionHandler`
3. **DOS Clipboard**: `dos/int21.go` (WinOldAp implementation)
4. **Screen Snapshot**: `bios/screen.go:Screen`
5. **Code Page**: `cp/cp.go` (for character conversion)

## References

- **DESIGN.md**: §8 (screen snapshots), §9 (clipboard), §10 (code page)
- **FRONTEND.md**: §5.5 (clipboard), §6 (plan F0-F7)
- **TASKS.md**: T06 (current), T05 (completed WinOldAp), T14 (vtui frontend)
- **Ralf Brown's Interrupt List**: INT 2Fh/17xx (WinOldAp)
- **DOSBox-X**: Reference implementation (`dos clipboard api = true`)

## Testing with Real Programs

### VC (Volkov Commander)
VC supports clipboard via WinOldAp. After patching VC with our clipboard client:
1. Select files/text in VC
2. Press Ctrl-Ins (copy to clipboard)
3. Outside VC, press Ctrl-V to paste
4. Inside VC, press Shift-Ins to paste from host

### NC (Norton Commander)
NC should work with clipboard if it implements WinOldAp or compatible API.
Test with actual NC binary to verify.

### Custom DOS Programs
Programs using `INT 2Fh/17xx` for clipboard will work directly.
Programs reading keys from BIOS will see pasted text as keystrokes.

## Commit History

| Commit | Author | Date | Message |
|--------|--------|------|---------|
| 256d960 | Claude Haiku 4.5 | 2026-10-01 | Text Selection Interface (T06 commit 1/8) |
| 2e4d76d | Claude Haiku 4.5 | 2026-10-01 | Paste from Selection (T06 commit 2/8) |
| c9c906c | Claude Haiku 4.5 | 2026-10-01 | Frontend Selection Handler (T06 commit 3/8) |
| 8912850 | Claude Haiku 4.5 | 2026-10-01 | Terminal Integration: Shift+Arrow (T06 commit 4/8) |
| ab00e93 | Claude Haiku 4.5 | 2026-10-01 | Paste by Key: Ctrl-V pasting (T06 commit 5/8) |
| 589af1e | Claude Haiku 4.5 | 2026-10-01 | Tests and fixes (T06 commit 6/8) |
| 17487bf | Claude Haiku 4.5 | 2026-10-01 | End-to-end tests (T06 commit 7/8) |
| TBD | Claude Haiku 4.5 | 2026-10-01 | Documentation (T06 commit 8/8) |

## Sign-off

T06 is complete and ready for review. All features specified in TASKS.md are implemented:

✓ Copying from screen (terminal selection)
✓ Pasting with key presses (Ctrl-V)
✓ Text → keystrokes conversion with portion handling
✓ Multi-line support with newline → Enter conversion
✓ Code page awareness
✓ Tests (unit and e2e)
✓ Documentation

Next task: T14 (vtui frontend with mouse support and visual highlighting)
