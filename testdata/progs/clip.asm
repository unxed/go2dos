; clip.com - a WinOldAp client (INT 2Fh AX=17xxh) in the order Emacs uses.
; Prints "VER=...." (1700h), "SIZE=........" (1704h, OEM text) and the text
; read with 1705h, then "BM=........" (size of format 2, the bitmap), and
; replaces the clipboard text with "dos <e acute>" CR LF "second" (1701h,
; 1702h, 1703h, 1708h), printing "SET=...." (the status of 1703h).
; Without WinOldAp (1700h unchanged) it prints "NO CLIP".
        cpu 186
        org 100h
        mov ax, 1700h
        int 2Fh
        cmp ax, 1700h
        jne havecl
        jmp noclip
havecl:
        mov [r_ax], ax
        mov dx, msgver
        call puts
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ax, 1701h           ; open
        int 2Fh
        or ax, ax
        jnz opened
        jmp fail
opened:
        mov ax, 1704h           ; size of the OEM text
        mov dx, 7
        int 2Fh
        mov [r_ax], ax
        mov [r_dx], dx
        mov dx, msgsize
        call puts
        mov ax, [r_dx]
        call hex16
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ax, 1705h           ; the text
        mov dx, 7
        mov bx, buf
        int 2Fh
        or ax, ax
        jnz gotit
        jmp fail
gotit:
        mov ax, 1708h           ; close
        int 2Fh
        mov si, buf             ; print up to the first NUL
pr:     lodsb
        or al, al
        jz prdone
        mov dl, al
        mov ah, 2
        int 21h
        jmp pr
prdone: call crlf
        mov ax, 1704h           ; the bitmap format: no data
        mov dx, 2
        int 2Fh
        mov [r_ax], ax
        mov [r_dx], dx
        mov dx, msgbm
        call puts
        mov ax, [r_dx]
        call hex16
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ax, 1701h
        int 2Fh
        mov ax, 1702h           ; empty
        int 2Fh
        mov ax, 1703h           ; set: DX format, ES:BX data, SI:CX size
        mov dx, 7
        mov bx, newtext
        xor si, si
        mov cx, newlen
        int 2Fh
        mov [r_ax], ax
        mov ax, 1708h
        int 2Fh
        mov dx, msgset
        call puts
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ax, 4C00h
        int 21h
noclip: mov dx, msgno
        call puts
        mov ax, 4C00h
        int 21h
fail:   mov ax, 4C01h
        int 21h

puts:   mov ah, 9
        int 21h
        ret
crlf:   mov dx, msgcrlf
        jmp puts
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

msgver  db 'VER=$'
msgsize db 'SIZE=$'
msgbm   db 'BM=$'
msgset  db 'SET=$'
msgno   db 'NO CLIP', 13, 10, '$'
msgcrlf db 13, 10, '$'
newtext db 'dos ', 82h, 13, 10, 'second'
newlen  equ $ - newtext
r_ax    dw 0
r_dx    dw 0
buf     times 256 db 0
