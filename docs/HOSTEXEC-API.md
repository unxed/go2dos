# DOS-HOST/HOSTEXEC: running a host command from a DOS program (draft 1.0)

Status: implemented in go2dos (`dos/hostexec.go`). The design is in
`HOSTEXEC.md` (in Russian); this is the interface for other implementations
and for DOS programs.

An emulator with a host operating system can let a DOS program run a command
of the host. Because this leaves the sandbox, the emulator ships it **off by
default** (go2dos: `-host-exec`). The provider is found through AMIS, the
Alternate Multiplex Interrupt Specification (`INT 2Dh`, Ralf Brown's Interrupt
List), so there is no fixed multiplex number.

## Discovery

For each multiplex number AH = 00h..FFh call `INT 2Dh` with AL = 00h. A
provider answers AL = FFh, CX = version (CH major, CL minor) and DX:DI pointing
to its signature:

| Bytes | Value |
|---|---|
| 0-7   | manufacturer `DOS-HOST` |
| 8-15  | product `HOSTEXEC` |
| 16-   | ASCIIZ description |

The provider is present even when running host commands is disabled; the call
below then fails (see Errors), so a program can tell "disabled" from "no
provider".

## AL = 10h: run a command

In:

- AH = the multiplex number found, AL = 10h;
- DS:SI -> parameter block:

| Offset | Size | Meaning |
|---|---|---|
| 00h | DWORD | far pointer to the command line, ASCIIZ, in the OEM code page |
| 04h | DWORD | far pointer to a DOS directory (ASCIIZ) in which to run it, or 0000:0000 for the current directory |

Out:

- CF clear: AX = exit code of the command (0-255; 255 also stands for "ended
  by a signal"). The output of the command (standard output and error together)
  was written to the caller's standard output (handle 1) in the OEM code page
  with CR LF line ends; the command reads an empty standard input.
- CF set: AX = DOS error code (see below).

The line is given to the shell of the host (`sh -c`, `cmd /c` on Windows), so
pipes, redirection and wildcards work as at a terminal. The working directory
is the host directory that the given (or current) DOS directory maps to. The
command runs to its end (the emulator waits, with a limit of 60 seconds in
go2dos).

## Errors (CF set)

| AX | Meaning |
|---|---|
| 0005h | access denied: running host commands is disabled |
| 0003h | path not found: the directory does not exist |
| 0002h | file not found: the command could not be started |

The other AMIS functions (01h-06h) answer as the specification says for a
program that has no private entry point, hotkeys or device drivers.
