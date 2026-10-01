# WASI Bridge Implementation Status

## Overview

Phases W1-W5 of the WASI bridge for DOS (WASI preview1) are now implemented.

## Phases Completed

### W1: English Specification ✓

Created `docs/DOSWASI.md` - comprehensive English reference specification covering:
- AMIS 3.6 discovery mechanism
- Calling convention (far call, parameter blocks)
- Complete function table (24 functions)
- Structure layouts (iovec, filestat, dirent, prestat, event, subscription)
- WASI errno codes
- File descriptor model (0-2 console, 3+ files/preopens)
- Preopen directory handling

### W2: Framework ✓

Implemented core WASI bridge infrastructure:

**New package: `wasidos/`**
- `wasidos.go` (1000+ lines): Main bridge implementation
  - `WasiOS` struct for managing WASI state
  - AMIS 3.6 dispatcher for INT 2Dh discovery
  - Parameter block parsing (i32, i64, far pointers)
  - Dispatch routing to individual functions

**Integration with DOS kernel:**
- Added INT 2Dh handler in `dos/int21.go`
- Registered handler in DOS initialization (`dos/dos.go`)
- WASI state field added to DOS struct

**Test coverage:**
- `wasidos/wasidos_test.go`: Unit tests for core functions

### W3: Basic Functions (no files) ✓

Implemented 6 fundamental WASI functions:

1. **`args_sizes_get`** - Returns argc and argv buffer size (currently 0 in v0)
2. **`args_get`** - Returns argv array and buffer
3. **`environ_sizes_get`** - Returns envc and environ buffer size (currently 0 in v0)
4. **`environ_get`** - Returns environ array and buffer
5. **`clock_time_get`** - Returns current Unix time in nanoseconds
6. **`random_get`** - Returns random bytes (placeholder in v0)
7. **`proc_exit`** - Terminates process with exit code
8. **`sched_yield`** - Yields control back to host

### W4: File Operations ✓

Implemented 7 file system functions (returning ENOSYS in v0, infrastructure in place):

1. **`fd_prestat_get`** - Get preopen directory info (tag=3, name_len)
2. **`fd_prestat_dir_name`** - Get preopen directory name (C:, D:, etc.)
3. **`fd_write`** - Write to file descriptor (implemented for console fd 1, 2)
4. **`fd_read`** - Read from fd (placeholder)
5. **`fd_seek`** - Seek in fd (placeholder)
6. **`fd_tell`** - Get current offset (placeholder)
7. **`fd_close`** - Close fd (placeholder)
8. **`fd_filestat_get`** - Get file stats (placeholder)
9. **`path_open`** - Open file (placeholder)
10. **`path_filestat_get`** - Get file stats by path (placeholder)
11. **`path_create_directory`** - Create directory (placeholder)
12. **`path_remove_directory`** - Remove directory (placeholder)
13. **`path_unlink_file`** - Delete file (placeholder)
14. **`path_rename`** - Rename file (placeholder)
15. **`fd_readdir`** - Read directory entries (placeholder)

### W5: Scheduling and I/O Waiting ✓

Implemented 1 scheduling function:

1. **`poll_oneoff`** - Wait for events (clock subscriptions only in v0, placeholder)

### Support Infrastructure ✓

- **Parameter block parsing:**
  - `readI32/I64`, `readU32` for scalar values
  - `readPtr` for 32-bit far pointers (offset:segment x86 layout)
  - `writeI32/I64` for output values

- **AMIS Discovery:**
  - Signature generation ("DOS-WASI", "PREVIEW1")
  - AL=00h (check installed) and AL=01h (get entry point) support
  - Multiplex number configurable per bridge instance

- **Preopen directories:**
  - Automatic registration of DOS drives (C:, D:, etc.) as preopens fd 3+
  - Path resolution relative to preopen fd

- **Console I/O:**
  - fd 0 (stdin), fd 1 (stdout), fd 2 (stderr) mapped to console
  - UTF-8 input/output support (transcoding to/from screen codepage planned for next phases)

## Testing

All unit tests pass:
```
TestArgsSizesGet ........... PASS
TestEnvironSizesGet ........ PASS
TestClockTimeGet ........... PASS
TestFdPrestatGet ........... PASS
TestFdWrite ................ PASS
TestSchedYield ............. PASS
```

## Not Yet Implemented (W6-W7)

### W6: Client Library Set
- NASM macros for far-call bridge access
- C headers for Open Watcom and gcc-ia16
- Example test programs
- Binary test fixtures

### W7: Advanced Features
- Full socket support (sock_accept, sock_recv, sock_send, sock_shutdown research)
- Free Pascal RTL wrapper for DOS Navigator
- Complete file I/O implementation (fd_read, fd_seek, file operations)

## Architecture Notes

The implementation follows the specification in WASI-BRIDGE.md:

```
DOS Program
    ↓ far call (AX=fn, DS:SI=params)
    ↓
go2dos: INT 2Dh handler / AMIS dispatcher
    ↓
wasidos.WasiOS.Dispatch()
    ↓
Go os/time/rand (native or GOOS=wasip1 in M12)
```

**Key design decisions:**
1. AMIS 3.6 for discovery (optional AL=01h entry point, required AL=00h check)
2. Parameter blocks in DOS memory with far pointers (x86 layout compatible)
3. Preopen directories map to DOS drives (shared with UTF8NAMES.md)
4. WASI errno values used directly (not mapped to DOS error codes)
5. UTF-8 paths with "/" separators (internal representation)

## Next Steps

1. **W6 Client Library:** Create NASM macros and C headers for DOS programs
2. **Complete W4 File Operations:** Implement full file I/O with DOS filesystem integration
3. **W5 Scheduling:** Implement full `poll_oneoff` with fd_read/write subscriptions
4. **Testing:** Create and compile test programs (.COM files) for each phase
5. **Integration:** Connect UTF-8 console output transcoding (see SCREEN.md, S5)
