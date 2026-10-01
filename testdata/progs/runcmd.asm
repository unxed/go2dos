; runcmd.com - runs C:\COMMAND.COM (what a program does with COMSPEC) with
; its own command tail, then prints "EXIT AX=xxxx" (INT 21h/4Dh: exit type
; and code), or "EXEC FAILED AX=xxxx".
        org 100h
        mov bx, 1000h           ; shrink to 64 KiB for the child
        mov ah, 4Ah
        int 21h
        mov si, 80h             ; copy length, text and CR of the tail
        mov di, tailbuf
        xor cx, cx
        mov cl, [80h]
        add cx, 2
        rep movsb
        mov [pb + 4], cs
        mov [pb + 8], cs
        mov [pb + 12], cs
        mov ax, 4B00h
        mov dx, comspec
        mov bx, pb
        int 21h
        jc failed
        mov ah, 4Dh
        int 21h
        mov [r_ax], ax
        mov dx, msgexit
        call puts
        jmp done
failed: mov [r_ax], ax
        mov dx, msgfail
        call puts
done:   mov ax, [r_ax]
        call hex16
        mov dx, crlf
        call puts
        mov ax, 4C00h
        int 21h

puts:   mov ah, 9
        int 21h
        ret
hex16:  push ax
        mov al, ah
        call hex8
        pop ax
hex8:   push ax
        shr al, 4
        call nib
        pop ax
        and al, 0Fh
nib:    add al, '0'
        cmp al, '9'
        jbe .p
        add al, 7
.p:     mov dl, al
        mov ah, 2
        int 21h
        ret

comspec db 'C:\COMMAND.COM', 0
msgexit db 'EXIT AX=$'
msgfail db 'EXEC FAILED AX=$'
crlf    db 13, 10, '$'
pb      dw 0, tailbuf, 0, 5Ch, 0, 6Ch, 0
r_ax    dw 0
tailbuf times 130 db 0
