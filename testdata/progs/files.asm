; files.com - creates TEST.TXT, writes to it, finds it with FindFirst,
; reads it back, prints the name and content, deletes it, exits with 0.
; Any failure exits with code 1.
        org 100h
        mov ah, 3Ch             ; create
        xor cx, cx
        mov dx, fname
        int 21h
        jc fail
        mov bx, ax
        mov ah, 40h             ; write
        mov cx, datalen
        mov dx, data
        int 21h
        jc fail
        mov ah, 3Eh             ; close
        int 21h
        mov ah, 4Eh             ; find first
        xor cx, cx
        mov dx, fname
        int 21h
        jc fail
        mov si, 80h + 1Eh       ; name in the default DTA
print:  lodsb
        or al, al
        jz named
        mov dl, al
        mov ah, 02h
        int 21h
        jmp print
named:  mov ax, 3D00h           ; open for reading
        mov dx, fname
        int 21h
        jc fail
        mov bx, ax
        mov ah, 3Fh
        mov cx, 64
        mov dx, buf
        int 21h
        jc fail
        mov si, buf
        add si, ax
        mov byte [si], '$'
        mov ah, 3Eh
        int 21h
        mov ah, 09h
        mov dx, colon
        int 21h
        mov ah, 09h
        mov dx, buf
        int 21h
        mov ah, 41h             ; delete
        mov dx, fname
        int 21h
        jc fail
        mov ax, 4C00h
        int 21h
fail:   mov ax, 4C01h
        int 21h
fname   db 'TEST.TXT', 0
data    db 'file I/O works'
datalen equ $ - data
colon   db ': $'
buf     times 65 db 0
