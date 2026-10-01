; Test WinOldAp clipboard (INT 2Fh AX=17xxh)
; Write text to clipboard, read it back, verify it matches.

    bits 16
    cpu 8086

    org 0x100

    ; Check if WinOldAp is installed (INT 2Fh AX=1700h)
    mov ax, 0x1700
    int 0x2F
    cmp ax, 0x1703          ; Should return 0x1703 if installed
    jne not_installed

    ; Open clipboard (INT 2Fh AX=1701h)
    mov ax, 0x1701
    int 0x2F
    cmp ax, 1               ; Should return 1 (success)
    jne open_failed

    ; Set text "Hello" in clipboard (INT 2Fh AX=1703h)
    ; ES:BX = buffer, CX = format (1=CF_TEXT), SI = size
    mov ax, 0x1703
    mov bx, text_hello
    mov cx, 1               ; CF_TEXT
    mov si, 5               ; size of "Hello"
    push ds
    pop es
    int 0x2F
    cmp ax, 1               ; Should return 1 (success)
    jne set_failed

    ; Get text back from clipboard (INT 2Fh AX=1705h)
    ; ES:BX = buffer, CX = format (1=CF_TEXT), SI = buffer size
    mov ax, 0x1705
    mov bx, text_buf
    mov cx, 1               ; CF_TEXT
    mov si, 64              ; buffer size
    int 0x2F
    ; AX now contains the size returned

    ; Compare the returned text with the original
    mov cx, ax              ; size returned
    xor si, si
    xor di, di
check_loop:
    cmp si, cx
    jge check_ok
    mov al, [text_hello + si]
    mov bl, [text_buf + di]
    cmp al, bl
    jne check_failed
    inc si
    inc di
    jmp check_loop

check_ok:
    ; Print success message
    mov ax, 0x0E4B          ; 'K'
    int 0x10
    mov ax, 0x0E4F          ; 'O'
    int 0x10
    int 0x20                ; Exit to DOS

check_failed:
    ; Print 'F' for failure
    mov ax, 0x0E46          ; 'F'
    int 0x10
    int 0x20

set_failed:
    mov ax, 0x0E53          ; 'S'
    int 0x10
    int 0x20

open_failed:
    mov ax, 0x0E4F          ; 'O'
    int 0x10
    int 0x20

not_installed:
    mov ax, 0x0E4E          ; 'N'
    int 0x10
    int 0x20

text_hello: db "Hello"
text_buf:   db 64 dup(0)
