; ctrlc.com - checks Ctrl-C / Ctrl-Break handling. Command tail " <m>[P]":
;   m=0  own INT 23h that prints "CC" and returns with IRET (call repeated)
;   m=1  own INT 23h that returns with STC, RETF (the program ends)
;   m=2  no INT 23h of its own (the default ends the program)
;   m=3  own INT 1Bh that prints "B71=" and bit 7 of 0040:0071 in hex (the
;        program prints "R" first, once the handler is installed)
; The program waits for a key with INT 21h/08h, prints "GOT <key>" and exits
; with 0. With P it runs itself as a child (EXEC) and prints the exit type
; and code (4Dh) as "EXIT AX=....".
        org 100h
start:  mov al, [82h]
        cmp byte [83h], 'P'
        je parent
        cmp al, '0'
        je mode0
        cmp al, '1'
        je mode1
        cmp al, '3'
        je mode3
        jmp getkey
mode0: mov ax, 2523h
        mov dx, h0
        int 21h
        jmp getkey
mode1: mov ax, 2523h
        mov dx, h1
        int 21h
        jmp getkey
mode3: mov ax, 251Bh
        mov dx, h3
        int 21h
        mov dx, msgrdy          ; tells the test that INT 1Bh is hooked
        mov ah, 9
        int 21h
getkey:   mov ah, 08h
        int 21h
        mov [key], al
        mov dx, msggot
        mov ah, 9
        int 21h
        mov dl, [key]
        mov ah, 2
        int 21h
        mov dx, crlf
        mov ah, 9
        int 21h
        mov ax, 4C00h
        int 21h

parent: mov bx, 1000h
        mov ah, 4Ah
        int 21h
        mov al, [82h]
        mov [tailbuf + 2], al
        mov [pb + 4], cs
        mov [pb + 8], cs
        mov [pb + 12], cs
        mov ax, 4B00h
        mov dx, selfname
        mov bx, pb
        int 21h
        jc pfail
        mov ah, 4Dh
        int 21h
        mov [r_ax], ax
        mov dx, msgexit
        mov ah, 9
        int 21h
        mov ax, [r_ax]
        call hex16
        mov dx, crlf
        mov ah, 9
        int 21h
        mov ax, 4C00h
        int 21h
pfail:  mov ax, 4C01h
        int 21h

h0:     push ax
        push dx
        mov dx, msgcc
        mov ah, 9
        int 21h
        pop dx
        pop ax
        iret
h1:     stc
        retf
h3:     push ax
        push dx
        push es
        mov dx, msgb71
        mov ah, 9
        int 21h
        mov ax, 40h
        mov es, ax
        mov al, [es:71h]
        call hex8
        mov dx, crlf
        mov ah, 9
        int 21h
        pop es
        pop dx
        pop ax
        iret

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

selfname db 'CTRLC.COM', 0
msggot   db 'GOT $'
msgcc    db 'CC', 13, 10, '$'
msgrdy   db 'R', 13, 10, '$'
msgb71   db 'B71=$'
msgexit  db 'EXIT AX=$'
crlf     db 13, 10, '$'
tailbuf  db 2, ' ', '0', 13
pb       dw 0, tailbuf, 0, 5Ch, 0, 6Ch, 0
key      db 0
r_ax     dw 0
