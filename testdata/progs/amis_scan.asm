; amis_scan.com - scans INT 2Dh for AMIS providers and prints what it finds.
; Returns with exit code = number of providers found.
        org 100h

        xor ax, ax              ; AX = 0, used for loop counter
        xor bx, bx              ; provider count
        jmp scan_start

scan_loop:
        inc al                  ; next multiplex number

scan_start:
        cmp al, 0               ; have we tried all 256?
        jne scan_check          ; if not, continue
        mov al, bl              ; exit code = number of providers found
        mov ah, 4Ch
        int 21h

scan_check:
        push ax                 ; save multiplex number

        ; Call INT 2Dh with AL=00h (installation check) for AH=multiplex
        mov ah, al              ; AH = multiplex number
        mov al, 00h             ; AL = 00h (check)
        int 2Dh

        pop cx                  ; restore multiplex number to CX for later

        ; Check if AL = FFh (provider found)
        cmp al, 0FFh
        jne scan_loop           ; if not found, continue scanning

        ; Provider found!
        inc bx                  ; increment provider count

        ; Print a message: "Provider at XX: xxxxxxxx/yyyyyyyy"
        ; Print "Prov: "
        mov dx, msg_prov
        mov ah, 09h
        int 21h

        ; Print multiplex number in hex (CL)
        mov al, cl
        call print_hex

        ; Print ": "
        mov dx, msg_colon
        mov ah, 09h
        int 21h

        ; Print manufacturer (8 bytes from DI:DX)
        ; For now, just a placeholder message
        mov dx, msg_newline
        mov ah, 09h
        int 21h

        jmp scan_loop

print_hex:
        ; Print AL as 2 hex digits
        mov ah, al
        shr al, 4
        call print_hex_digit
        mov al, ah
        and al, 0Fh
        call print_hex_digit
        ret

print_hex_digit:
        ; Print AL (0-F) as hex digit
        cmp al, 9
        jle digit_0_9
        add al, 'A' - 10
        jmp digit_out
digit_0_9:
        add al, '0'
digit_out:
        mov dl, al
        mov ah, 02h
        int 21h
        ret

msg_prov        db 'Found provider at $'
msg_colon       db ': ', 0Dh, 0Ah, '$'
msg_newline     db 0Dh, 0Ah, '$'
