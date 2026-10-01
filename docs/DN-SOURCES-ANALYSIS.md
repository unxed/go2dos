# DN 1.51 Source Code Analysis

**Version**: 1.51  
**Release Date**: April 19, 1999  
**Source**: RIT Research Labs (dn151src.zip)  
**Target Platform**: MS-DOS, Turbo Pascal 7.0 + TASM/TLINK

---

## Executive Summary

DN 1.51 is a classic DOS file manager written in Turbo Pascal 7.0 that provides a Norton Commander-like interface. The analysis of its source code reveals an architecture built on standard DOS INT 21h functions with minimal support for modern DOS extensions. This report details the INT 21h functions used, clipboard handling, and file name constraints that would be critical for implementing DN support in go2dos.

---

## INT 21h Functions Analysis

### Core File I/O Functions

| Function | AH Code | Purpose | Usage in DN |
|----------|---------|---------|------------|
| Open File | 3Dh | Open existing file with specified access mode | TDosStream.Init (OBJECTS.PAS) |
| Close File | 3Eh | Close file handle | TDosStream.Done (OBJECTS.PAS:960) |
| Read File | 3Fh | Read from file | TDosStream.Read (OBJECTS.PAS:1029), TBufStream (1222) |
| Write File | 40h | Write to file | TDosStream.Write (OBJECTS.PAS:1091), TBufStream (1128, 1129) |
| Seek File | 42h | Reposition file pointer with multiple methods:<br>- AX=4200h: Seek from start<br>- AX=4201h: Seek from current position<br>- AX=4202h: Seek from end | TDosStream.Seek (1059), GetPos (977), GetSize (995, 1001, 1008) |
| Create File | 3Ch | Create new file (must exist: stCreate = $3C40) | TDosStream.Init |
| Delete File | 41h | Delete file | Referenced in FuncClassTab (SYSINT.ASM:267) |
| Get File Attributes | 43h | Get/Set file attributes | Referenced in FuncClassTab (SYSINT.ASM:269) |
| Rename File | 56h | Rename/move file | Referenced in FuncClassTab (SYSINT.ASM:288) |
| Get/Set File Date/Time | 57h | Get or set file timestamp | Referenced in FuncClassTab (SYSINT.ASM:289) |

### Directory & Path Functions

| Function | AH Code | Purpose | Usage in DN |
|----------|---------|---------|------------|
| Find First | 4Eh | Find first file matching mask | Turbo Pascal DOS unit; used throughout (FILEFIND.PAS, FILECOPY.PAS, etc.) |
| Find Next | 4Fh | Find next file in search | Turbo Pascal DOS unit; used throughout |
| Create Directory | 39h | Create new directory | Referenced in FuncClassTab (SYSINT.ASM:259) |
| Remove Directory | 3Ah | Delete empty directory | Referenced in FuncClassTab (SYSINT.ASM:260) |
| Change Directory | 3Bh | Change current directory | Referenced in FuncClassTab (SYSINT.ASM:261) |
| Get Current Directory | 47h | Get full path of current directory | Referenced in FuncClassTab (SYSINT.ASM:273) |
| Get Disk Free Space | 36h | Get bytes free, total, etc. | Referenced in FuncClassTab (SYSINT.ASM:256) |

### System & Process Functions

| Function | AH Code | Purpose | Usage in DN |
|----------|---------|---------|------------|
| Get DOS Version | 30h | Query DOS version (major.minor) | DN.PAS:586 |
| Get Current Drive | 19h | Return current drive number | SYSINT.ASM:765 |
| Load/Execute Program | 4Bh | Run external program | Referenced in FuncClassTab (SYSINT.ASM:277) |
| Get PSP Address | 51h | Get Program Segment Prefix | OBJECTS.PAS:1377 |
| Get Device Info | 44h | Get device characteristics | SYSINT.ASM:745 |
| Duplicate Handle | 45h | Duplicate file handle | OBJECTS.PAS:1462 |
| Redirect Device | 46h | Force I/O redirection | OBJECTS.PAS:1410 |
| Get/Set Ctrl-Break | 33h | Ctrl-Break status | SYSINT.ASM:304-309 |

### Interrupt Handler Control

| Function | AH Code | Purpose | Usage in DN |
|----------|---------|---------|------------|
| Get Interrupt Vector | 35h | Read IVT entry (DOS 2.0) | SYSINT.ASM:397-398 (INT 21h/35h for INT 67h, EMS) |
| Set Interrupt Vector | 25h | Write IVT entry (DOS 2.0) | SYSINT.ASM: preserved but managed via DPMI |

### INT 67h: EMS (Expanded Memory) Functions

DN includes EMS memory support (TEmsStream, OBJECTS.PAS:1350+):

| Function | AH Code | Purpose | EMS Usage |
|----------|---------|---------|-----------|
| Get EMS Version | 46h | Query EMS version | OBJECTS.PAS:1412 |
| Get EMM Device Address | 41h | Get base segment for EMS | OBJECTS.PAS:1409 |
| Allocate Pages | 43h | Allocate expanded memory | OBJECTS.PAS:1429 |
| Map Page | 44h | Map EMS page to window | OBJECTS.PAS:1344 |
| Deallocate Handles | 45h | Free EMS memory | OBJECTS.PAS:1462 |
| Get Unallocated Pages | 42h | Query available EMS | OBJECTS.PAS:1429 |
| Get Handle Pages | 51h | Query pages for handle | OBJECTS.PAS:1377 |

---

## Clipboard Support

### Current Implementation

DN 1.51 includes a module called **WinClp** (visible in DN.PAS overlays at line 216), which suggests Windows clipboard support was integrated. However:

1. **No explicit INT 21h clipboard operations**: The standard DOS clipboard interface (INT 21h functions 17h/Copy, 18h/Paste) is not used in the examined code.

2. **Windows clipboard integration**: The presence of WinClp overlay indicates DN could access Windows clipboard when running under Windows (VM), but this is via Windows API, not DOS INT 21h.

3. **Limitations**:
   - Clipboard operations in pure DOS mode likely require custom memory-resident handlers or direct video/keyboard manipulation
   - No cross-application clipboard sharing in native DOS (except through files)
   - Windows 3.x/9x clipboard access requires linking to Windows DLLs

### Implications for go2dos

- Clipboard operations would need to be implemented via the host environment (Windows/Linux terminal)
- Pure DOS mode clipboard would require file-based workarounds (temporary files)
- Consider implementing a small TSR (terminate and stay resident) for clipboard support, or relay through host system calls

---

## File Name Support

### 8.3 Naming Constraints

DN 1.51 **exclusively uses DOS 8.3 file names** (maximum 8 characters + 3-character extension):

1. **Evidence**:
   - TFindFile record (FILEFIND.PAS:111) uses fixed array: `Name: Array [1..12] of Char` (8.3 format)
   - No LFN (Long File Name) API calls (INT 21h/AX=714Eh, 714Fh, etc.)
   - FindFirst/FindNext (INT 21h/4Eh/4Fh) returns only 8.3 names via DOS unit

2. **File name searches**: 
   - Uses masks like `'*.pas'`, `'*.*'`, `'*.TDR'` (FILEFIND.PAS, FILECOPY.PAS)
   - No VFAT or LFN aliases (like `PROGRA~1` to `Program Files`)

3. **Attribute handling**:
   - Uses standard DOS attributes: Archive, ReadOnly, Hidden, SysFile
   - No extended attributes or alternative data streams

### Impact on go2dos

- **Full compatibility** with DOS/FAT file systems (8.3 names only)
- **No LFN support needed** for basic DN functionality
- **Compatibility issue**: Modern FAT/VFAT systems (Windows 95+, USB drives) expose long file names, which DN cannot display
  - Truncates long names to 8.3 DOS aliases (e.g., `LONGFILE.TXT` → `LONGFI~1.TXT`)
  - Cannot rename files to long names
  - Cannot display original long names in file listing

---

## INT 21h Function Classification

The SYSINT.ASM file contains a function classification table (FuncClassTab at line 254) that categorizes DOS functions by parameter type:

```asm
cNothing  = 0  ; No check needed (AX)
cName     = 2  ; Check name at DS:DX
cHandle   = 4  ; Check handle in BX
cDrive    = 6  ; Check drive in DL
```

This enables automatic validation of file names, drive letters, and file handles to prevent operations on unsupported drives (floppy swap protection).

---

## DPMI (DOS Protected Mode Interface) Support

DN 1.51 includes **dual-mode support**:

1. **Real Mode** (Real DOS): Direct INT 21h calls via saved vectors
2. **Protected Mode** (Windows 3.x/Win95 WinOS2 box): INT 21h calls via DPMI translation

DPMI functions used:
- `dpmiAllocDesc` (0000h): Allocate memory descriptor
- `dpmiFreeDesc` (0001h): Free descriptor
- `dpmiSetSegBase` (0007h): Set descriptor base address
- `dpmiGetRealInt` (0200h): Get real-mode interrupt vector
- `dpmiSetRealInt` (0201h): Set real-mode interrupt vector
- `dpmiGetProtInt` (0204h): Get protected-mode interrupt vector
- `dpmiSetProtInt` (0205h): Set protected-mode interrupt vector
- `dpmiAllocRMCB` (0303h): Allocate real-mode callback
- `dpmiFreeRMCB` (0304h): Free real-mode callback

This dual-mode support allows DN to run correctly under:
- Pure DOS (6.0, 6.2, 6.22)
- Windows 3.x DOS box
- Windows 95/98 DOS box
- OS/2 Warp WinOS2

---

## EMS & XMS Memory Support

DN includes support for extended memory beyond the 640 KB conventional memory limit:

### EMS (Expanded Memory)
- Accessible via INT 67h (EMS manager)
- TEmsStream class implements buffered access to EMS pages
- Page size: typically 16 KB per page
- Version detection (EMS 3.x vs 4.x)

### XMS (Extended Memory)
- Not directly visible in examined code
- May be accessed indirectly through overlays (ExtraMemory.PAS)

---

## Error Handling

DN intercepts multiple critical interrupts to provide graceful error handling:

| Interrupt | Function | Purpose |
|-----------|----------|---------|
| INT 09h | Keyboard | Custom key conversion (SYSINT.ASM:585) |
| INT 1Bh | Ctrl-Break | Custom ctrl-break handler |
| INT 21h | DOS | Function validation, drive swap protection (SYSINT.ASM:698) |
| INT 23h | Ctrl-C | Custom ctrl-c handler |
| INT 24h | Critical Error | Disk error handler with drive swap detection (SYSINT.ASM:838) |

The INT 24h handler is particularly sophisticated:
- Detects printer errors (code 9)
- Distinguishes block vs. character devices
- Enables automatic disk swap detection (floppy A: → B:)
- Returns appropriate error codes to DOS

---

## Build System

DN is built using:
- **Compiler**: Borland Pascal 7.0
- **Assembler**: TASM (Turbo Assembler)
- **Linker**: TLINK (Turbo Linker)
- **Build Script**: BUILDDN.BAT

The overlay system (`{$O}` directives) manages the 640 KB real-mode DOS memory constraint by loading code modules on-demand.

---

## Summary of Key Findings

### What DN 1.51 Uses
1. **Standard DOS INT 21h functions** (3Ch-57h range)
2. **8.3 file naming only** - No LFN support
3. **EMS memory** (INT 67h) for buffering beyond 640 KB
4. **DPMI support** for protected-mode environments
5. **Interrupt interception** for graceful error handling
6. **Windows clipboard** integration (WinClp overlay)

### What DN 1.51 Does NOT Use
1. ❌ Long File Names (LFN / VFAT, INT 21h/714Eh+)
2. ❌ DOS clipboard (INT 21h/17h, 18h)
3. ❌ Direct XMS memory API
4. ❌ Device drivers or block device control (INT 25h, 26h)
5. ❌ Network redirector (INT 2Fh/11h)
6. ❌ Novell/Unix file permissions (beyond basic DOS attributes)

---

## Requirements for go2dos DN Support

### Phase 1: Core Compatibility (Minimum)
- [ ] Implement INT 21h functions 30h, 19h, 33h, 36h, 39h-3Fh, 41h-47h, 51h, 54h, 56h-57h
- [ ] Support 8.3 file names with DOS attributes (Archive, ReadOnly, Hidden, System)
- [ ] Implement file handle management (open/close/read/write/seek)
- [ ] Handle drive detection and current directory tracking
- [ ] Interrupt vector management for error handlers

### Phase 2: Extended Compatibility (High Priority)
- [ ] EMS memory support (INT 67h) via emulated expanded memory
- [ ] DPMI support for protected-mode hosts
- [ ] Keyboard interrupt (INT 09h) handling with custom key mappings
- [ ] Critical error handler (INT 24h) with drive swap detection
- [ ] Ctrl-Break/Ctrl-C handling (INT 1Bh, INT 23h)

### Phase 3: Modern Features (Nice to Have)
- [ ] Long File Name (LFN) support via VFAT extensions
  - Requires INT 21h/AX=714Eh (Get LFN) and 714Fh (Create LFN)
  - Maintain backward compatibility with 8.3 DOS names
- [ ] Clipboard integration via host system
  - File-based interim solution for pure DOS mode
  - Windows/Linux clipboard bridge for VM environments
- [ ] XMS memory access
- [ ] Network/mounted volume support

### Phase 4: Compatibility Testing
- [ ] Test with DN 1.51 itself (regression testing)
- [ ] Verify with other DOS applications (Norton Commander, VC, etc.)
- [ ] Stress test file operations (large files, deep directory trees)
- [ ] Cross-platform testing (DOS 6.x, Windows 3.x box, Windows 95/98 box)

---

## References

1. **DN 1.51 Source**: `/tmp/claude-1001/-home-ivan/4de3ddac-e7e9-410b-a2a1-66eb0d948ee1/scratchpad/dn151/`
2. **Key Files**:
   - `SYSINT.ASM` - Interrupt handler infrastructure
   - `OBJECTS.PAS` - Stream I/O with INT 21h wrappers
   - `FILEFIND.PAS` - File finding and directory traversal
   - `FILECOPY.PAS` - File copy/move operations
   - `DN.PAS` - Main program structure

3. **Related INT 21h Documentation**:
   - DOS Programmer's Reference (Ralph Brown Interrupt List)
   - Borland Pascal 7.0 Runtime Library documentation
   - DPMI 1.0 Specification (Intel/Microsoft)

---

**Analysis Date**: 2024-10-01  
**Analyst**: Claude Haiku  
**Status**: Complete
