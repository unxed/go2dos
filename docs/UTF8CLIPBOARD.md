# UTF-8 text of the clipboard for DOS programs (draft 1.0)

Status: implemented in go2dos (`dos/utf8clip.go`; tests `machine/utf8clip_test.go`, the client
`testdata/progs/utf8clip.asm`). It is the twin of [UTF8NAMES.md](UTF8NAMES.md) and has the same shape:
a per-process opt-in found through AMIS. The goal is an extension small enough for any DOS
environment that has a clipboard server (the WinOldAp `INT 2Fh AH=17h` of Windows 3.x/9x and of the
emulators that copy it: go2dos, DOSBox-X) and for the programs that use it, without rewriting either.
Version 1.0 is also implemented in DOSBox-X (https://github.com/joncampbell123/dosbox-x/pull/6632).
The section "Version 1.1 (proposal)" at the end is a proposal: no provider implements it yet. The
project as a whole (who implements what, open questions): [UTF8-DOS.md](UTF8-DOS.md) (in Russian).

## Problem

The WinOldAp clipboard formats `CF_TEXT` (01h) and `CF_OEMTEXT` (07h) carry text in the OEM code
page. A host that holds Unicode text has to convert it, and every character that the code page lacks
is lost on the way: Greek copied from a browser arrives in a CP866 program as question marks, and the
other way round. A program that is UTF-8 inside (an editor, a file manager) must convert at the
border and cannot do better than the page.

## Design

Per-process opt-in, as for the file names: a program that knows UTF-8 turns it on for itself; other
processes, including the children it starts, are not affected. The provider is found through AMIS
(`INT 2Dh`), exactly as in UTF8NAMES.md.

### Discovery

For each multiplex number AH = 00h..FFh call `INT 2Dh` with AL = 00h. A provider answers AL = FFh,
CX = version (CH major, CL minor) and DX:DI -> its signature: manufacturer `DOS-UTF8`, product
`CLIPBRD ` (8 + 8 bytes, space-padded), then an ASCIIZ description. Version 1.0 is described here.
The provider exists only where the clipboard server exists: a client that finds no WinOldAp (1700h
leaves AX unchanged) will not find this provider either.

### Functions (AH = the multiplex number found)

**AL = 10h: set the encoding of the clipboard text for the calling process**

- In: BX = 65001 (UTF-8) or 0 (the OEM code page, the default).
- Out: AL = FFh on success, BX = the previous setting. AL = 00h if the value is not supported (the
  setting is unchanged).

**AL = 11h: get the encoding.** Out: AL = FFh, BX = the current setting (65001 or 0).

The calling process is the one whose PSP is current. The setting ends with that process and is not
inherited by `EXEC`.

### Semantics in UTF-8 mode

The WinOldAp functions are the same; only the text on the wire changes, for the formats 01h
(`CF_TEXT`) and 07h (`CF_OEMTEXT`) of the calling process:

1. 1704h (size) returns the size in bytes of the UTF-8 text, including the final 0.
2. 1705h (data) gives the text as UTF-8 with CR LF line ends and a final 0. A reader stops at the first
   0 byte, as before.
3. 1703h (set data) takes UTF-8. A byte sequence that is not valid UTF-8 is replaced by U+FFFD
   (the provider does not fail the call).
4. The limit of the text (1 MB, 1709h) is in bytes; a text that is cut is cut at a character boundary.
5. No new format number is used: `CF_UNICODETEXT` of Windows is UTF-16 and is not meant here. The
   mode is a property of the process, not of the format, so one process never sees two encodings of the
   same format.

Everything else (the screen, the keyboard, the environment) stays in the OEM code page.

## Client checklist

1. At start-up: check WinOldAp (1700h), find the provider and call AL = 10h with BX = 65001. If there is
   no provider, carry on in the OEM code page as before (convert at the border).
2. Copy: 1701h, 1702h, 1703h with the UTF-8 text (CR LF or LF: the server normalizes), 1708h.
3. Paste: 1701h, 1704h (size), allocate the buffer, 1705h, 1708h, convert CR LF as needed.

## Provider notes

- A host that holds Unicode text converts nothing in UTF-8 mode (the text is only given a CR LF).
- A provider in an environment that keeps the clipboard text itself (DOSBox-X: the host clipboard in
  UTF-8) has the same duty as for the names: the setting belongs to the process whose PSP is current.
- A provider must implement the AMIS installation check and the AMIS functions it is required to
  support (the same dispatcher as for the other providers).

## Version 1.1 (proposal): the clipboard without WinOldAp

Status: proposal, not implemented. Nothing in this section may be relied on until a provider
implements it.

### Problem

An answer to `INT 2Fh AX=1700h` tells a program more than "there is a clipboard": it says "you run
in a DOS box of Windows 3.x/9x". Some programs change behaviour on it. Norton Desktop 1.0 and the
tools of Norton Commander turn off their own character set and the graphic mouse cursor
(https://github.com/joncampbell123/dosbox-x/issues/5025). DOSBox-X keeps a list of program names
that get no answer to 1700h, and its maintainer asked in that issue whether the clipboard could be
offered "in a way that doesn't make DOS programs think they're running under Windows". A clipboard
driver for real DOS (a TSR: for example, the shared clipboard of VirtualBox) would cause the same side
effect in every program, including the ones that never use the clipboard. That is the main
objection to such a driver.

### Design

The provider `DOS-UTF8/CLIPBRD` of version 1.1 (CX = 0101h) also accepts the clipboard calls
themselves, so that `INT 2Fh` is no longer the only way to reach the clipboard.

**AL = 20h + n, n = 00h..0Fh: clipboard call.** The same as `INT 2Fh AX = 1700h + n`: the same input
registers (DX = format, ES:BX = buffer, SI:CX = size), the same results (AX, DX:AX). Both ways lead to
the same state (one clipboard, one "open" flag) and use the same encoding (AL = 10h), so a call
through `INT 2Dh` and a call through `INT 2Fh` are the same call. Version 1.1 defines 20h (version, AX
as 1700h returns it), 21h (open), 22h (empty), 23h (set data), 24h (size), 25h (get data),
28h (close), 29h (compact). A number of the range that the server does not implement returns
AX = 0 and changes nothing.

**`INT 2Fh AH=17h` becomes optional.** A server of version 1.1 may leave `INT 2Fh AH=17h` unanswered
(a quiet server). Programs that look for Windows then find nothing, and programs that know this
provider still have the clipboard. Recommended defaults:

- emulators that already answer 1700h (go2dos, DOSBox-X) keep answering it: clients of
  version 1.0, GNU Emacs and 4DOS still work;
- a new driver for real DOS (a TSR) is quiet by default and answers `INT 2Fh` only when this is
  switched on.

The rule of 1.0 "the provider exists only where the clipboard server exists" becomes "the provider
exists only where a clipboard exists". A client of 1.0 checks only the major version (CH = 1), so it
still finds the provider; against a quiet server it finds no WinOldAp and goes on without a
clipboard, as it would without Windows.

### Client checklist (1.1)

1. Find `DOS-UTF8/CLIPBRD`. If it is there and CX >= 0101h, call AL = 10h with BX = 65001 and make
   every clipboard call through `INT 2Dh` (AH = the multiplex number, AL = 20h + n). Do not call
   1700h.
2. If it is there with version 1.0: call AL = 10h, then use `INT 2Fh AH=17h` as in the 1.0 checklist.
3. If it is not there: `INT 2Fh AX=1700h`; any change of AX means WinOldAp, with text in the OEM
   code page.

For an existing WinOldAp client the change is in one place, the routine that does
`mov ax,17xxh` / `int 2Fh`: it becomes `mov ah,[mux]` / `mov al,20h+xx` / `int 2Dh`.

Protected mode clients (DPMI: DJGPP, HX): ES:BX is a real mode address, as for WinOldAp. Call
`INT 2Dh` through DPMI function 0300h (simulate a real mode interrupt) with the buffer in
conventional memory.

### Left out on purpose

- A new format number for UTF-8 or UTF-16 (`CF_UNICODETEXT`): see point 5 of the 1.0 semantics.
- A device (`CLIP$` of DOSBox-X): ordinary file I/O, but no size and no way to say the encoding.
- A new number on `INT 2Fh`: collisions are the problem AMIS exists to solve.
- A change counter (as `GetClipboardSequenceNumber` of Win32): left for a later minor version,
  when there is a client that needs it.
