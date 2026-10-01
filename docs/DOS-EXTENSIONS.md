# go2dos: extensions of the DOS environment (specification, draft 1.0)

Status: this document is the single entry point to everything that go2dos adds to
a plain DOS/BIOS for programs running in it, and to what it plans to add. Details
that already have their own document are summarised here and linked; the
interfaces that had none (the clipboard server, the long file name functions, the
keyboard modifier flags, the built-in shell, the changes to standard calls) are
specified here. Written for implementers of other emulators and for authors of DOS
programs. The other documents of the repository are in Russian; the interface
specifications (`TEXTWIN.md`, `UTF8NAMES.md`, `HOSTEXEC-API.md`) and this one are in
English.

Every item has a status. **Implemented** means: the code is in `main` and a test
runs it (the tests are named below). **Planned** means: a design exists, there is no
code in `main`; nothing in a planned section may be relied on.

## 1. Overview

| § | Extension | Where a program sees it | Status | Details |
|---|---|---|---|---|
| 2 | AMIS discovery of our providers | `INT 2Dh` | Implemented | this document, §2 |
| 3 | Clipboard server | `INT 2Fh AH=17h` (WinOldAp) | Implemented | §3 |
| 4 | Long file names | `INT 21h AH=71h` | Implemented | §4 |
| 5 | UTF-8 file names, `DOS-UTF8/NAMES` | `INT 2Dh` AL=10h, 11h | Implemented | §5, [UTF8NAMES.md](UTF8NAMES.md) |
| 6 | Lossless names for programs that do not know UTF-8 | classic `INT 21h` and `71xx` | Implemented | §6, [NAMES.md](NAMES.md) (Russian) |
| 7 | Text window size and events, `DOS-HOST/TEXTWIN` | `INT 2Dh` AL=10h-12h | Implemented (machine level) | §7, [TEXTWIN.md](TEXTWIN.md) |
| 8 | Host commands, `DOS-HOST/HOSTEXEC` | `INT 2Dh` AL=10h; `COMMAND.COM` | Implemented, off by default | §8, [HOSTEXEC-API.md](HOSTEXEC-API.md) |
| 9 | Keyboard modifier flags of the key just read | `INT 16h AH=02h/12h` | Implemented | §9 |
| 10 | Changes to standard calls (volume, IOCTL, DPB) | `INT 21h` | Implemented | §10 |
| 11 | Built-in `COMMAND.COM` | `EXEC` of `COMMAND.COM` | Implemented | §11, [HOSTEXEC.md](HOSTEXEC.md) (Russian) |
| 12 | WASI bridge, `DOS-WASI/PREVIEW1` | `INT 2Dh` and a far call | **Planned** | §12, [WASI-BRIDGE.md](WASI-BRIDGE.md) (Russian) |
| 13 | Extension module of Volkov Commander 4.05, `VCEXT.BIN` (VCX1) | VC only, not a DOS API | Implemented | §13, [ASM-GATES.md](ASM-GATES.md) (Russian) |

Not DOS interfaces, but part of the same product and not described here: the pipe
mode (the program's handles 0-2 are the streams of the host, UTF-8 to OEM and back;
[SCREEN.md](SCREEN.md)), the screen size flag (`-size WxH`), the terminal keys
`Ctrl-]` (README).

Conventions: numbers are hexadecimal with the `h` suffix; `AL=10h` means "register
AL holds 10h"; OEM is the DOS code page of the machine (`-cp`, default from the
host locale). Where a call is not implemented the emulator stops with an error, or,
in `-lenient` mode, answers "not supported" (CF=1, AX=0001h) and logs it.

## 2. Discovery: AMIS (`INT 2Dh`)

Our private interfaces are found with the Alternate Multiplex Interrupt
Specification 3.6 (Ralf Brown's Interrupt List, `INT 2D`), so no multiplex number
is reserved. The emulator gives each provider the first free number from `C0h` up
(`C0h` is a choice of go2dos; the specification names no number).

A client checks that the `INT 2Dh` vector is not `0000:0000`, then for each
AH = 00h..FFh calls `INT 2Dh` with AL = 00h. A provider answers AL = FFh,
CX = version (CH major, CL minor) and DX:DI -> its signature: 8 bytes of
manufacturer, 8 bytes of product (both space-padded), then an ASCIIZ description
(at most 63 characters). A client compares the first 16 bytes. Private functions
are AL = 10h..FFh with the same AH; the AMIS functions 01h-06h answer as the
specification says for a program with no private entry point, hotkeys or device
drivers.

| Manufacturer | Product | Version | Function numbers | Section |
|---|---|---|---|---|
| `DOS-UTF8` | `NAMES   ` | 1.0 | 10h set encoding, 11h get encoding | §5 |
| `DOS-HOST` | `TEXTWIN ` | 1.0 | 10h size, 11h watch, 12h screen role | §7 |
| `DOS-HOST` | `HOSTEXEC` | 1.0 | 10h run a command | §8 |
| `DOS-WASI` | `PREVIEW1` | planned | planned: 01h returns the entry point | §12 |

Implementation: `dos/amis.go` (`RegisterAMIS`). The providers do not depend on one
another.

## 3. Clipboard server (WinOldAp, `INT 2Fh AH=17h`)

Status: implemented (`dos/clip.go`; tests `machine/clip_test.go`, and the Volkov
Commander tests `e2e/vc_clip_test.go`). The server speaks the protocol of the
Windows 3.x "old application" support (RBIL, `INT 2F/AX=1700h`), which the DOS
programs GNU Emacs (`w16select.c`), 4DOS and WCLIP already use. No new interface
was invented.

Presence: if the host has no clipboard (`Config.Clipboard == nil`), `INT 2Fh AH=17h`
is not handled and AX comes back unchanged, as without Windows. A client calls
AX=1700h and takes any change of AX as "present".

| AX | Function | In | Out |
|---|---|---|---|
| 1700h | version | | AX = 0A03h (the value is our assumption, "Windows 3.10") |
| 1701h | open | | AX = 1 on success, AX = 0 if already open |
| 1702h | empty | clipboard open | AX = 1 on success, 0 if not open |
| 1703h | set data | DX = format, ES:BX = data, SI:CX = size | AX = 1 on success, 0 on error (not open, wrong format, size over 1 MB) |
| 1704h | size of data | DX = format | DX:AX = size including the final 0; 0 if no data |
| 1705h | get data | DX = format, ES:BX = buffer | AX = 1 on success, 0 if no data |
| 1708h | close | | AX = 1 on success, 0 if not open |
| 1709h | compact | | DX:AX = 1 MB (always enough room) |

Formats: 01h (`CF_TEXT`) and 07h (`CF_OEMTEXT`); both are taken as text in the OEM
code page. Text on the wire is OEM bytes with CR LF line ends and a final 0. The
host side holds Unicode text with LF; the server converts in both directions through
the code page. A reader stops at the first 0 byte (Windows rounds sizes up and does
not shrink them after trailing blanks are cut). Any other function of `AH=17h`
stops the machine ("not supported"). All of this is per machine: there is one
clipboard and one "open" flag.

Host side (not a DOS interface): in the terminal front end the clipboard of the
server is tied to the system clipboard (`-clip-sync auto|tool|osc52|off`, default
`auto`, `cmd/go2dos/sysclip.go`): a write goes to `wl-copy`, `xclip`, `xsel` or
`pbcopy` if one is installed (and `WAYLAND_DISPLAY`/`DISPLAY` is set), else to the
terminal with OSC 52; a read takes the text from `wl-paste`, `xclip -o`, `xsel -b -o`
or `pbpaste`, else the text held last (including text the terminal pasted in
bracketed-paste mode). The tools get one second. Other front ends (vtui, headless)
are not changed.

Typical use (as in the clients above): copy = 1700h, 1701h, 1702h, 1703h, 1708h;
paste = 1700h, 1701h, 1704h (size), allocate a buffer of at least that size, 1705h,
1708h. The text of the clipboard is limited by the client's buffer: a client must
check the size from 1704h before 1705h, because the server writes all of it.

## 4. Long file names (`INT 21h AH=71h`)

Status: implemented (`dos/lfn.go`; tests `dos/lfn_test.go`, and in the Volkov
Commander 4.99.09 tests). The functions follow the Windows 95 LFN API (RBIL
`INT 21/AX=71xxh`). Can be switched off as a whole (`-nolfn`,
`Config.NoLFN`): then every `71xx` call answers AX=7100h, CF=1 ("not supported"), so
that a program sees no sign of long names.

| AX | Function |
|---|---|
| 710Dh | reset drive |
| 7139h / 713Ah / 713Bh | make / remove / change directory |
| 7141h | delete file |
| 7143h | get / set attributes |
| 7147h | get current directory (long name, without drive and the leading backslash) |
| 714Eh / 714Fh / 71A1h | find first / next / close; 71A2h is the matching "extended" find |
| 7156h | rename or move |
| 7160h | true name (CL = 0 full, 1 short, 2 long) |
| 716Ch | extended open / create |
| 71A0h | volume information: flags 4006h = case preserved, Unicode names, LFN functions present |

Long names are the host names. The encoding of the names on this API depends on the
mode of the process: the OEM code page by default, UTF-8 after §5. Short names are
the 8.3 form with a numeric tail (`NAME~1.EXT`) where the long name does not fit.
Names that the code page cannot show are handled by §6.

## 5. UTF-8 file names (`DOS-UTF8/NAMES`)

Status: implemented (`dos/utf8names.go`; tests `machine/utf8names_test.go`). Full text:
[UTF8NAMES.md](UTF8NAMES.md). A program that knows UTF-8 finds the provider (§2) and
calls AL = 10h with BX = 65001 (UTF-8) or 0 (OEM, the default); AL = 11h returns the
current setting in BX. The setting belongs to the process whose PSP is current and
ends with it (`EXEC` does not inherit it). In UTF-8 mode every `71xx` call takes and
returns long names and paths in UTF-8, short names stay ASCII, and the upper-case
tables of `INT 21h AX=6502h/6504h/6520h-6522h` leave bytes 80h-FFh alone, so table
driven case conversion cannot corrupt UTF-8. Everything else (screen, keyboard,
environment, command lines) stays OEM. The extension is small enough for any DOS
that has the LFN API (the document gives the checklist for a client and the notes
for a provider).

## 6. Lossless names for programs that do not know UTF-8

Status: implemented (`dos/names.go`; tests `dos/names_test.go`,
`e2e/vc_names_test.go`). Full text, in Russian: [NAMES.md](NAMES.md). A host name
with a character that the OEM code page lacks is given to a program under an alias
that is unique and leads back to the host name: `STEM~HHHH.EXT` on the classic
calls and `71xx` short names, and a long form with `_` for the missing characters
plus `~HHHH` on the long calls. The registry of aliases is per run and for all
directories; renaming, copying, moving, creating by alias, deleting and entering a
directory act on the real host name, a rename that only changes the extension keeps
the base, and nothing is overwritten. The registry is filled by listing a directory;
an alias saved by a program in another run is unknown until its directory is
listed. A program that selects UTF-8 (§5) does not see aliases for long names.

## 7. Text window size and events (`DOS-HOST/TEXTWIN`)

Status: implemented at machine level (`dos/textwin.go`, `Machine.Resize`; tests
`machine/textwin_test.go`); the go2dos front ends do not report window changes to
the machine yet. Full text: [TEXTWIN.md](TEXTWIN.md). Functions on the AMIS number
of the provider: AL = 10h returns the window size (CX columns, DX rows, BX the most
cells, 32768 in go2dos, with 80-255 columns and 25-255 rows); AL = 11h with BL = 1
asks to be told of size changes, which then update the BDA and the CRTC (the screen is
cleared) and put the word **FF00h** in the keyboard buffer (`INT 16h` returns
AX = FF00h; scan code FFh, the keyboard overrun code no real key sends); AL = 12h
sets the role of the screen (BL = 0 console stream, 1 full-screen interface).

## 8. Host commands (`DOS-HOST/HOSTEXEC`)

Status: implemented, **off by default** because it leaves the sandbox
(`dos/hostexec.go`; `-host-exec`, `Config.HostExec`; tests `machine/hostexec_test.go`,
`e2e/vc_shell_test.go`). Full text: [HOSTEXEC-API.md](HOSTEXEC-API.md). AL = 10h with
DS:SI -> a block of two far pointers (an OEM command line in ASCIIZ; a DOS
directory in ASCIIZ or 0000:0000) runs the command in the shell of the host
(`sh -c`, `cmd /c`), the output goes to the caller's handle 1 (CR LF line ends, OEM),
standard input is empty, the wait is at most 60 s. CF clear: AX = exit code
(255 also means "ended by a signal"); CF set: AX = DOS error (0005h disabled,
0003h directory not found). The provider answers AMIS even when the feature is
off, so a program can tell "disabled" from "absent". The same mechanism serves the
built-in shell (§11).

## 9. Keyboard: modifier flags of the key just read (`INT 16h AH=02h/12h`)

Status: implemented (`bios/bios.go`; tests `machine/kbdmods_test.go`). On a real
PC the Shift, Ctrl and Alt state in the BDA (`0040:0017`, bits 0-3) is that of the
moment, so a program that reads a key and then asks `INT 16h AH=02h` sees what is
held now. An emulator gets keys as events and the key-up arrives long before a
program reads the key, which loses the difference between Ins (52E0h) and Shift-Ins,
or Ctrl-Ins, which have the same word. go2dos therefore keeps the flags of every
key in the type-ahead buffer, and `INT 16h AH=00h/10h` puts the flags of the key it
returns back into the BDA until the next `INT 16h` read or poll. A program that
reads a key and then calls `AH=02h` or `AH=12h` thus gets the modifiers that the key
was pressed with, as for a key held down. Nothing changes for programs that do not
ask. (The Volkov Commander clipboard keys of §13 depend on this.)

## 10. Changes to standard calls

Behaviour that a program may rely on, added to a plain DOS (see `docs/DOUBTS.md`,
"DOS", for the sources):

| Call | Behaviour |
|---|---|
| `INT 21h AX=440Dh CX=0866h` (generic IOCTL, get media id) and `AH=69h AL=0` | level 0, volume serial (FNV-1a of the host root path, stable for a directory), label (11 bytes, `NO NAME    ` if none), file system `FAT16`; other `CL` values stop the machine |
| `AX=4408h/4409h/440Eh/440Fh` | check the drive (error 000Fh for an unmapped one); `4401h` checks `DH` (000Dh) |
| `AH=32h` (DPB) | `AL=FFh` ("invalid or network drive"): a host directory has no DPB |
| `AH=4Eh` with attribute 08h | returns the volume label (`Config.Labels`, default `NO NAME`) in the root, as DOS 3+ does |
| `INT 23h`, `INT 24h`, `INT 1Bh` | Ctrl-C and critical error handling as in DOS (`dos/crit.go`) |
| after `EXEC` (`4B00h`) | the parent gets its own registers back, as `restore_world` does in MS-DOS 4.0 (needed by Volkov Commander 4.99.09) |

## 11. Built-in `COMMAND.COM`

Status: implemented (`dos/shell.go`; tests `e2e/vc_shell_test.go`). When a program runs `COMMAND.COM` (typically through
`COMSPEC`, `COMMAND.COM /C line`) and the host directory has no such file, `EXEC`
creates an ordinary process whose code traps into the emulator, so the PSP chain and
the exit code behave as with a real shell. If a `COMMAND.COM` file is there (for
instance FreeCOM), it runs instead.

Internal commands: `REM ECHO VER EXIT CD/CHDIR MD/MKDIR RD/RMDIR DEL/ERASE
REN/RENAME SET PATH PROMPT TYPE COPY DIR CALL`. The lookup order is: internal
commands, DOS programs (the current directory, then `PATH`), and, with host
commands on (§8), the programs of the host (`PATH` of the host); then "Bad command
or file name". A line that starts with `!` goes to the host at once. The exit code of
`COMMAND.COM /C` is that of the last program; a command that is not found gives 255
(an arbitrary choice).

Planned (the owner's wish, recorded in `HOSTEXEC.md`): the internal commands, and
the programs started from the shell, should use the extensions of this document
(UTF-8 and long names, the clipboard, the window size, command execution), so that
what a program can do on its own it can also do through the shell.

## 12. WASI bridge (`DOS-WASI/PREVIEW1`): planned

Status: **planned, no code in `main`**. The design, the research (R1-R5) and the
function mapping (W1-W4) are in [WASI-BRIDGE.md](WASI-BRIDGE.md) and
[WASI-RESEARCH.md](WASI-RESEARCH.md) (Russian). Nothing below is final.

- **Purpose.** Give a DOS program the whole host through the API of a WebAssembly
  sandbox, WASI preview1 (`wasi_snapshot_preview1`): files with UTF-8 paths, 64-bit
  offsets, directories, clocks, random numbers, arguments and environment. The UTF-8
  names of §5 stay as the quick path for LFN-aware programs (Volkov Commander, Dos
  Navigator); the bridge is for new and ported programs (C, Pascal) that need the
  host, not only names. WASI 0.2 (preview2, the Component Model) is out of scope.
- **Discovery.** AMIS (§2): manufacturer `DOS-WASI`, product `PREVIEW1`; AMIS
  function 01h ("get private entry point") returns DX:BX, and the program makes a far
  call to it.
- **Call.** AX = function number (a numbering of the bridge, not the witx order),
  DS:SI -> a parameter block (the parameters in witx order; i32, pointers and
  descriptors take 4 bytes, i64 takes 8, no alignment). Out: AX = WASI errno (0 on
  success), CF = 1 if AX is not 0; output values are written through pointers, as in
  preview1.
- **Pointers.** 32-bit x86 far pointers (offset in the low word), so the preview1
  structures (`iovec`, `filestat`, `dirent`, `prestat`, `event`, `subscription`) keep
  the wasm32 layout byte for byte.
- **Strings and paths.** UTF-8 (`ptr`, `len`), `/` as the separator, paths relative to
  a preopened descriptor.
- **Preopens and descriptors.** fd 3, 4, ... one per mounted drive, named `C:`, `D:`,
  ...; the sandbox and the path resolution are those of the DOS drives. The descriptor
  table is per process (PSP), separate from the DOS handles, and closed when the
  process ends. fd 0-2 are the console (output converted from UTF-8 to the screen code
  page).
- **Process.** `args_*` and `environ_*` come from the PSP tail and the environment,
  converted from OEM to UTF-8; `proc_exit` acts as `AH=4Ch`; `sched_yield` and waits in
  `poll_oneoff` go through the idle call.
- **Order of work.** Research R1-R5 (any time) and the details W1-W4 are done on
  paper; the implementation is milestone M11, after UTF-8 names (M10), re-using the
  AMIS dispatcher and the drive paths. Milestone M12 puts go2dos itself under a wasm
  runtime and in the browser, where the bridge becomes a direct pass-through of WASI.
- **Not yet decided** (to be settled before the first line of code, in `DOUBTS.md`):
  the numbering of the functions, how the 64-bit values are passed, the behaviour of
  a descriptor shared by two processes, and how much of preview1 is implemented
  first (files and the console before `poll_oneoff`).

## 13. Extension module of Volkov Commander 4.05 (`VCEXT.BIN`, format VCX1)

Status: implemented; this is **not a DOS interface**: it extends one program, and
other programs cannot use it. Specified in `third_party/vc/ext/vcext.asm` and
[ASM-GATES.md](ASM-GATES.md). Volkov Commander 4.05 is one `.COM` of 64 KB and has
no room for new functions, so the build in `bin/4.05-clip` (patch
`third_party/vc/patches/vc-4.05-clip.patch`, built by `tools/build-vc405-pts.sh`)
loads `VCEXT.BIN` from the directory of `VC.COM` at every start of VC (and again after
every program it runs) into a 4 KB block, and calls its entries with a far call.
Without the file VC works as before.

- **Format.** The signature `VCX1`; then entries of 3 bytes each (`JMP NEAR`) at
  offsets 4 and 7. The block: 0000h-07FFh the file, 0800h-0FFFh the text of the
  clipboard (the loader keeps the path of the file at 0C00h only until the module
  runs).
- **Entries.** 4 *Copy*: ES:DI = an ASCIIZ line, copies it to the clipboard (§3), an
  empty line is not copied, CF = 1 if there is no clipboard server. 7 *Fetch*: the
  first line of the clipboard text (at most 2048 bytes in all) is put at `CS:0800h`,
  CX = its length (0 = none), CF = 1 if there is no server.
- **Keys** (input fields and the command line only): Ctrl-Ins copies the line,
  Shift-Ins (or Ctrl-Shift-Ins from a terminal that delivers it; the host turns it into Shift-Ins) pastes the first line, Shift-Del cuts it (Del without Shift deletes a
  character, as before). Terminals that keep these combinations for themselves can
  send them with `Ctrl-]`, then `y`, `p` or `x`.
- **Limits.** `VC.COM` has 8 bytes left of 65280 (gate G3 of `ASM-GATES.md`). The editor
  of VC 4.05 has no block selection; clipboard in the editor is planned, and the editor
  of 4.99.09 does not exist in the build (its source is commented out).

## 14. Versions and compatibility

The AMIS version (§2) of a provider is the version of its interface; a client checks
the major number (CH). A change that a client written for the earlier minor number
cannot survive raises the major number. The WinOldAp server (§3) and the LFN
functions (§4) are not ours to version: they follow the sources named there. The
status words in §1 are the truth about `main`; the documents of the repository that
were written before the code may still say "done in a branch", and the CI and the
code win over them (README).
