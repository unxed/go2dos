; hello.com - prints a line through INT 21h/09h and exits with code 7.
        org 100h
        mov dx, msg
        mov ah, 09h
        int 21h
        mov ax, 4C07h
        int 21h
msg     db 'Hello from go2dos', 13, 10, '$'
