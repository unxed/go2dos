; longline.com - prints a 160-character line that wraps to two screen rows.
; Tests the wrapped line handling in Screen.Text().
        org 100h
        mov dx, msg
        mov ah, 09h
        int 21h
        mov ax, 4C00h
        int 21h
msg     db 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA$'
