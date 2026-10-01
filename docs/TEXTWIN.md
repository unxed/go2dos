# DOS-HOST/TEXTWIN: text window size, size events, screen role (draft 1.0)

Status: implemented in go2dos (`dos/textwin.go`, `Machine.Resize`). The design
is in `SCREEN.md` (in Russian); this is the interface for other
implementations and for DOS programs.

An emulator draws the text screen in a window of the host, and the window may
be any size and may change while the program runs. The program learns the size
from the places it already reads (BDA `0040:004A`, `0040:0084`, `INT 10h/0Fh`,
the CRTC), and this API adds what those cannot tell: the most cells the
emulator supports, a notification of a change, and the role of the screen. The
provider is found through AMIS (`INT 2Dh`), so there is no fixed multiplex
number.

## Discovery

For each multiplex number AH = 00h..FFh call `INT 2Dh` with AL = 00h. A
provider answers AL = FFh, CX = version (CH major, CL minor) and DX:DI pointing
to its signature:

| Bytes | Value |
|---|---|
| 0-7   | manufacturer `DOS-HOST` |
| 8-15  | product `TEXTWIN` |
| 16-   | ASCIIZ description |

## Functions (AH = the multiplex number found)

**AL = 10h: window size.** Out: AL = FFh, CX = columns, DX = rows, BX = the
most cells the emulator can show (go2dos: 32768; columns 80-255, rows 25-255).

**AL = 11h: watch the window size.** In: BL = 1 to be told of changes, 0 to
stop. Out: AL = FFh, BL = the previous setting. AL = 00h if BL is neither (the
setting is unchanged). While it is on, a change of the window size makes the
emulator update the BDA and the CRTC (as a mode set: the screen is cleared) and
put the word **FF00h** in the keyboard buffer, so that `INT 16h` returns
AX = FF00h (scan code FFh, ASCII 00h). The scan code FFh is the keyboard
overrun code that no real key sends, so a program that does not ask for events
never sees it. The program handles the event with the code that redraws after
a change of the number of rows (25/43/50): read the size again, redraw.
Without AL = 11h a change updates the BDA and CRTC and nothing else.

**AL = 12h: role of the screen.** In: BL = 0 console (the output is a stream:
the host shows it in its normal buffer), BL = 1 full-screen interface. Out:
AL = FFh, BL = the previous role; AL = 00h for another BL. The role tells an
emulator that has both displays which one to use (go2dos `-display console`:
the grid on the alternate screen for role 1, the stream for role 0); other
emulators may ignore it.

The AMIS functions 01h-06h answer as the specification says for a program with
no private entry point, hotkeys or device drivers. Values in AL = 10h and 11h
are per machine, not per process.

## Notes

- The value FF00h is a choice of go2dos (the research step S0 of `SCREEN.md`
  has not settled one); it is part of this version of the API.
- The host side (a front end that reports window changes with
  `Machine.Resize`) is separate from this API; go2dos front ends do not call it
  yet, the machine-level API is tested (`machine/textwin_test.go`).
