; wasi_test.com - test WASI bridge discovery and basic function calls.
        org 100h

        ; Test 1: Try to discover WASI via AMIS (INT 2Dh)
        mov ah, 0x2D        ; AMIS multiplex for WASI (placeholder)
        mov al, 0x00        ; Check if installed
        mov es, cs
        mov di, sig_buf
        int 2Dh

        ; Check if found (AL should be 0xFF if found, 0x00 if not)
        cmp al, 0xFF
        jne .not_found

        ; Print success message
        mov dx, found_msg
        mov ah, 0x09
        int 21h
        jmp .exit

.not_found:
        ; Print not found message
        mov dx, not_found_msg
        mov ah, 0x09
        int 21h

.exit:
        ; Exit with code 0
        mov ax, 0x4C00
        int 21h

found_msg:      db "WASI bridge found", 13, 10, "$"
not_found_msg:  db "WASI bridge not found", 13, 10, "$"

sig_buf:        times 32 db 0   ; 32-byte buffer for AMIS signature
