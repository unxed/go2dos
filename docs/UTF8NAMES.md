# UTF-8 file names for DOS programs (draft 0.1)

Status: draft, implemented nowhere yet. Comments welcome. The goal is an
extension small enough to be added to any DOS that has the long file name
(LFN) API — emulators with host file systems (go2dos, DOSBox-X), DOSLFN on
FreeDOS or MS-DOS — and to LFN-aware programs, without rewriting either.

## Problem

DOS file names, including LFN names (`INT 21h AX=71xxh`), are byte strings in
the active OEM code page. Names with characters outside that page cannot be
shown or used: a Japanese name on a CP866 system, a Greek one on CP437.

A "UTF-8 code page" selected globally (suggested on the BTTR forum for DOSLFN)
breaks every program that does not expect it, and longer UTF-8 names overflow
fixed-size buffers.

## Design

UTF-8 naming is a per-process opt-in. A program that knows how to handle
UTF-8 turns it on for itself; nothing changes for any other process,
including the children it starts.

The provider is found through AMIS, the Alternate Multiplex Interrupt
Specification (`INT 2Dh`, Ralf Brown's Interrupt List), which exists exactly
to add APIs without number collisions.

### Discovery

For each multiplex number AH = 00h..FFh, call `INT 2Dh` with AL = 00h
(installation check). A provider answers AL = FFh, CX = version (CH major,
CL minor) and DX:DI pointing to its signature:

| Bytes | Value |
|---|---|
| 0-7   | manufacturer `DOS-UTF8` |
| 8-15  | product `NAMES   ` |
| 16-   | ASCIIZ description |

Before scanning, a client should check that the `INT 2Dh` vector is not
0000:0000. Version 1.0 is described here.

### Functions (AH = the multiplex number found)

**AL = 10h — set file name encoding for the calling process**

- In: BX = 65001 (UTF-8) or 0 (the OEM code page, the default).
- Out: AL = FFh on success, BX = previous setting. AL = 00h if the value is
  not supported (the setting is unchanged).

**AL = 11h — get file name encoding**

- Out: AL = FFh, BX = current setting (65001 or 0).

The calling process is the one whose PSP is current (`INT 21h AH=62h`) at the
time of the call. The setting ends with that process and is not inherited by
`EXEC`.

### Semantics in UTF-8 mode

1. Every `INT 21h` function that takes or returns a long name or path
   (`71xxh`) uses UTF-8 for it. Invalid UTF-8 in input is an error
   (AX = 0002h or 0003h, as for a name that does not exist).
2. Short (8.3) names are ASCII-only. A file whose short name would need
   other characters gets a numeric-tail alias (`NAME~1.EXT`). Classic
   functions (non-`71xx`) therefore see ASCII names only.
3. A long name whose UTF-8 form does not fit the LFN buffers (255 bytes for a
   name, 260 for a path) is reported by its short name in long-name fields.
4. Upper-case tables returned to the process by `INT 21h AX=6502h/6504h` map
   80h-FFh to themselves, and `AX=6520h-6522h` leave those bytes unchanged,
   so table-driven case conversion cannot corrupt UTF-8.
5. Everything else stays in the OEM code page: the screen, the keyboard, the
   environment, command lines.

## Client checklist

An LFN-aware program needs:

1. At start-up: find the provider and call AL = 10h with BX = 65001. If there
   is none, carry on in the OEM code page as before.
2. When displaying a name: convert UTF-8 to the screen code page (unmappable
   characters as `?`) and measure widths in characters, not bytes.
3. When passing a name to another program (command line, `EXEC`), pass the
   short name (`INT 21h AX=7160h CL=01h`), because that program is not in
   UTF-8 mode.

Everything else — opening, copying, renaming — passes names through as byte
strings and needs no change.

## Provider notes

- Emulators with host file systems already hold names in Unicode; UTF-8 mode
  skips the conversion to the OEM code page.
- DOSLFN stores names as UTF-16 on FAT; UTF-8 mode converts UTF-16 to UTF-8
  instead of to the OEM code page.
- A provider must implement the AMIS installation check and the AMIS
  functions it is required to support; see the AMIS specification.

## Open points

- Verify the AMIS details above against the AMIS 3.6 text before freezing
  version 1.0.
- Whether to also report UTF-8 mode through `INT 21h AX=71A0h` (volume
  information flags).
