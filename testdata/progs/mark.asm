; mark.com - writes its command tail (PSP:80h) to RAN.TXT in the current
; directory and exits with code 3. Shows that a command line really ran it.
        org 100h
        mov ah, 3Ch             ; create
        xor cx, cx
        mov dx, fname
        int 21h
        jc fail
        mov bx, ax
        xor cx, cx
        mov cl, [80h]
        mov dx, 81h
        mov ah, 40h             ; write the tail
        int 21h
        mov ah, 3Eh             ; close
        int 21h
        mov ax, 4C03h
        int 21h
fail:   mov ax, 4C01h
        int 21h
fname   db 'RAN.TXT', 0
