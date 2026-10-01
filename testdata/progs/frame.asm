; frame.com - draws a frame along the edges of the text screen using the size
; the BIOS data area reports: columns at 0040:004A, rows-1 at 0040:0084.
; Direct writes to B800h, so rows beyond B800h:7FFFh reach the 64 KiB window.
        org 100h
        cld
        mov ax, 40h
        mov es, ax
        mov ax, [es:4Ah]
        mov [cols], ax
        xor ax, ax
        mov al, [es:84h]
        inc ax
        mov [rows], ax
        mov ax, 0B800h
        mov es, ax
        xor si, si              ; row
row:    mov ax, si
        mul word [cols]
        shl ax, 1
        mov di, ax
        mov bl, '-'             ; edge rows: '-' with '+' corners
        mov bh, '+'
        cmp si, 0
        je go
        mov dx, [rows]
        dec dx
        cmp si, dx
        je go
        mov bl, ' '             ; other rows: '|' at the ends
        mov bh, '|'
go:     mov ah, 07h
        mov cx, [cols]
        sub cx, 2
        mov al, bh
        stosw
        mov al, bl
        rep stosw
        mov al, bh
        stosw
        inc si
        cmp si, [rows]
        jb row
        mov ax, 4C00h
        int 21h
cols    dw 0
rows    dw 0
