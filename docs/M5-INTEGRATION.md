# M5 Integration: Unified TUI Framework

## Overview

The m5 system integrates three core components into a unified terminal user interface (TUI) framework:

- **T-04: Keyboard Event Decoder** (`keys/decoder.go`) - Decodes raw terminal input and ANSI escape sequences
- **T-05: UI Framework** (`ui/` package) - Provides basic UI components (Buffer, Panel, List)
- **T-06: Filesystem Abstraction** (`fs/fs.go`) - Clean filesystem operations layer

## Architecture

```
┌─ cmd/m5/main.go ────────── Application entry point
│
├─ m5/app.go ──────────────── Main App struct, terminal init, event loop
│
├─ T-04: keys/decoder.go ──── KeyEvent, KeyType, Decoder
│  └─ Parses: CSI sequences (arrows, etc.), control chars, escape codes
│
├─ T-05: ui/ ─────────────── UI Components
│  ├─ component.go ─────────── Base Drawable interface
│  ├─ buffer.go ───────────── Screen buffer with rendering
│  └─ panel.go ────────────── Layout containers
│
└─ T-06: fs/fs.go ────────── Filesystem wrapper
   └─ Provides: ListDir, IsDir, GetParentDir, etc.
```

## Components

### T-04: Keyboard Decoder
- Converts raw input bytes to typed KeyEvent
- Supports arrow keys, Enter, Escape, Ctrl+C/V/X
- Handles incomplete sequences (buffering)
- Location: `keys/decoder.go`

### T-05: UI Framework
- `Buffer`: 2D grid of cells with color attributes
- `Component`: Base type with bounds and drawing
- `Drawable`: Interface for all renderable components
- Supports ANSI escape codes for colors and positioning
- Location: `ui/component.go`, `ui/buffer.go`, `ui/panel.go`

### T-06: Filesystem Abstraction
- `FS`: Root-bounded filesystem wrapper (prevents escape)
- `Entry`: File/directory metadata (name, type, size, mtime)
- `EntryType`: File, Directory, Symlink
- Methods: ListDir, IsDir, GetParentDir, etc.
- Location: `fs/fs.go`

## Test Coverage

- `keys/decoder_test.go`: Keyboard event parsing
- `ui/buffer_test.go`: Terminal rendering
- `fs/fs_test.go`: Filesystem operations
- `m5/event_handler_test.go`: Event routing
- `cmd/m5/main_test.go`: Integration scaffold

## Building and Running

```bash
# Build
go build -o ./bin/m5 ./cmd/m5

# Run (basic proof-of-concept, displays static text)
./bin/m5

# Test individual components
go test ./keys -v
go test ./ui -v
go test ./fs -v
go test ./m5 -v
```

## Next Steps

1. **Complete m5/file_browser.go**: List navigation, enter/parent directory
2. **Wire keyboard decoder**: Connect KeyEvent → app behavior
3. **Render file list**: Use Buffer + Panel to display entries
4. **Handle terminal resize**: SIGWINCH signal
5. **Keyboard input loop**: Read from stdin in goroutine
6. **Full integration test**: E2E navigation scenario

## Design Notes

- **Terminal Abstraction**: Buffer uses ANSI codes, works with any ANSI terminal
- **Component Layout**: Recursive SetBounds/Draw for composition
- **Filesystem Safety**: resolvePath prevents directory traversal
- **Event Loop**: Signal-driven with keyboard input in background goroutine

## Files Created

- `keys/decoder.go` (147 lines) - T-04 implementation
- `ui/component.go` (33 lines) - Base types
- `ui/buffer.go` (132 lines) - Screen buffer
- `m5/app.go` (107 lines) - Main app structure
- `cmd/m5/main.go` (36 lines) - Entry point
- `fs/fs.go` (141 lines) - T-06 implementation

Total: ~600 lines of core integration code
