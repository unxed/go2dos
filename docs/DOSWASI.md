# DOS-WASI Bridge: Specification

Status: W1 specification. This document is the English reference for other implementations.

## Overview

The DOS-WASI bridge provides access to the host through WebAssembly System Interface (WASI preview1) to DOS programs. A DOS program calls the bridge via a far call with function number in AX and parameters in a block at DS:SI.

## Discovery: AMIS 3.6

The bridge announces itself via AMIS (American Menus Information Standard, INT 2Dh):
- Multiplex number: vendor-defined (scan 00h-FFh)
- Call: INT 2Dh with AL=00h/FFh checks (as per AMIS 3.6)
- Signature:
  - Bytes 0-7: Vendor string `DOS-WASI ` (8 bytes, space-padded)
  - Bytes 8-15: Product string `PREVIEW1` (8 bytes, space-padded)
  - Bytes 16+: Optional ASCIZ description
- Entry point function AL=01h (optional): Returns DX:BX far pointer to bridge entry point

## Calling Convention

**Discovery:**
```
INT 2Dh: AH = multiplex number, AL = 00h/01h/02h per AMIS 3.6
```

**Direct call to bridge entry point:**
```
FAR CALL at [DX:BX] (found via AMIS AL=01h, or via INT 2Dh if AL=01h not supported)
```

**Parameters:**
- AX: Function number (see Function Table)
- DS:SI: Pointer to parameter block (format per function)
- Return: AX = WASI errno (0 = success); CF set if AX ≠ 0

**Parameter block layout:**
- i32 (function codes, fd): 4 bytes, little-endian
- i64 (offsets, sizes): 8 bytes, little-endian
- u32 (combined flags): 4 bytes, little-endian
- Far pointers (ptr, len): 4 bytes each (offset:segment), compatible with LDS/LES
- Structures: wasm32 layout byte-for-byte (see Structure Layouts)

## Function Table (v0 subset, 22 functions)

| # | Function | Type | Input | Output |
|---|----------|------|-------|--------|
| 0 | `args_sizes_get` | A | — | argc:i32, argv_buf_size:i32 |
| 1 | `args_get` | A | — | argv_array:ptr[argc*4], argv_buf:ptr |
| 2 | `environ_sizes_get` | A | — | envc:i32, env_buf_size:i32 |
| 3 | `environ_get` | A | — | env_array:ptr[envc*4], env_buf:ptr |
| 4 | `clock_time_get` | T | clock_id:u32, precision:u64 | time:ptr i64 |
| 5 | `random_get` | T | buf_len:u32 | buf:ptr, length |
| 6 | `proc_exit` | C | exit_code:i32 | — |
| 7 | `sched_yield` | T | — | — |
| 8 | `fd_prestat_get` | F | fd:i32 | buf:ptr[8] prestat |
| 9 | `fd_prestat_dir_name` | F | fd:i32, path_len:u32 | path:ptr |
| 10 | `fd_read` | F | fd:i32, iovs_count:u32 | iovs:ptr[iovs_count*8], nread:ptr i32 |
| 11 | `fd_write` | F | fd:i32, iovs_count:u32 | iovs:ptr[iovs_count*8], nwritten:ptr i32 |
| 12 | `fd_seek` | F | fd:i32, offset:i64, whence:u32 | new_offset:ptr i64 |
| 13 | `fd_tell` | F | fd:i32 | offset:ptr i64 |
| 14 | `fd_close` | F | fd:i32 | — |
| 15 | `fd_filestat_get` | F | fd:i32 | buf:ptr[64] filestat |
| 16 | `path_open` | F | dirfd:i32, dirflags:u32, path:ptr+len, oflags:u16, rights_base:i64, rights_inheriting:i64, fdflags:u16 | fd:ptr i32 |
| 17 | `path_filestat_get` | F | dirfd:i32, flags:u32, path:ptr+len | buf:ptr[64] filestat |
| 18 | `path_create_directory` | F | dirfd:i32, path:ptr+len | — |
| 19 | `path_remove_directory` | F | dirfd:i32, path:ptr+len | — |
| 20 | `path_unlink_file` | F | dirfd:i32, path:ptr+len | — |
| 21 | `path_rename` | F | old_fd:i32, old_path:ptr+len, new_fd:i32, new_path:ptr+len | — |
| 22 | `fd_readdir` | F | fd:i32, buf_len:u32 | buf:ptr, bufused:ptr i32 |
| 23 | `poll_oneoff` | T | in_subscriptions:ptr[nsubscriptions*48], out_events:ptr[nsubscriptions*32], nsubscriptions:u32 | nevents:ptr i32 |

Legend: A=args, T=time/scheduling, C=control, F=files

## Structure Layouts (wasm32, byte-for-byte)

**iovec (8 bytes, align 4):**
```
Offset  Size  Name
0       4     buf (far ptr)
4       4     buf_len (i32)
```

**filestat (64 bytes, align 8):**
```
Offset  Size  Name
0       8     dev (i64)
8       8     ino (i64)
16      2     filetype (u8)
18      6     (padding)
24      8     nlink (i64)
32      8     size (i64)
40      8     atim (i64, timestamp ns)
48      8     mtim (i64, timestamp ns)
56      8     ctim (i64, timestamp ns)
```

**dirent (24 bytes, align 8):**
```
Offset  Size  Name
0       8     d_next (i64, cookie for next read)
8       8     d_ino (i64, inode)
16      4     d_namlen (u32, name length)
20      1     d_type (u8, filetype)
21      3     (padding)
```

**prestat (8 bytes, align 4):**
```
Offset  Size  Name
0       1     tag (u8; 3=dir)
1       3     (padding)
4       4     pr_name_len (u32, for tag 3)
```

**event (32 bytes, align 8):**
```
Offset  Size  Name
0       8     userdata (i64)
8       2     error (u16, errno)
10      1     type (u8, 0=fd_read, 1=fd_write)
11      1     (padding)
12      4     fd_readwrite.nbytes (u32)
16      2     fd_readwrite.flags (u16)
18      14    (padding/reserved)
```

**subscription (48 bytes, align 8):**
```
Offset  Size  Name
0       8     userdata (i64)
8       1     u.tag (u8; 0=clock, 1=fd_read, 2=fd_write)
9       3     (padding)
12      36    u (union)
    For tag 0 (clock):
    12      4     clock_id (u32)
    16      8     timeout (i64, nanoseconds)
    24      8     precision (i64, nanoseconds)
    32      2     flags (u16)
    34      2     (padding)
```

## WASI errno Codes (excerpt)

| Code | Name | Meaning |
|------|------|---------|
| 0 | ESUCCESS | Success |
| 1 | E2BIG | Argument list too long |
| 2 | EACCES | Permission denied |
| ... | ... | ... |
| 2 | EBADF | Bad file descriptor |
| 4 | EBUSY | Device or resource busy |
| 9 | EEXIST | File exists |
| 13 | EACCES | Permission denied |
| 21 | EISDIR | Is a directory |
| 28 | ENOSPC | No space left on device |
| 38 | ENOSYS | Function not implemented |

(See WASI typenames.witx for full list of 77 errno codes)

## File Descriptors

- **fd 0**: Standard input (keyboard input, UTF-8)
- **fd 1**: Standard output (display, UTF-8 → screen codepage conversion)
- **fd 2**: Standard error (display, UTF-8 → screen codepage conversion)
- **fd 3+**: Opened files and preopen directories

## Preopen Directories

Each mounted DOS drive (C:, D:, …) is accessible as a preopen directory fd:
- fd 3: C: drive root
- fd 4: D: drive root
- etc.

Directory names (from `fd_prestat_dir_name`): "C:", "D:", etc.

Paths: UTF-8, "/" separator, relative to preopen fd.

## Implementation Notes

1. All paths are UTF-8 with "/" separators, converted to DOS paths before filesystem access.
2. File access respects the DOS sandbox and drive aliases (common path resolution with package `dos`).
3. Commands `proc_exit`, `sched_yield` work through the DOS environment (yield returns control, exit triggers process termination).
4. `poll_oneoff` in v0 supports only clock subscriptions; fd_read/write subscriptions return ENOSYS.
5. Functions not in v0 subset return ENOSYS (function not implemented).
