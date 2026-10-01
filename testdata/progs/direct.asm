; direct.com - prints a line, writes one cell directly to video memory,
; prints another line, exits. Tests the console display mode.
        org 100h
        mov dx, before
        mov ah, 09h
        int 21h
        push es
        mov ax, 0B800h
        mov es, ax
        mov word [es:0], 1E58h  ; 'X', yellow on blue
        pop es
        mov dx, after
        mov ah, 09h
        int 21h
        mov ax, 4C00h
        int 21h
before  db 'before', 13, 10, '$'
after   db 'after', 13, 10, '$'
