; hostexec.com - a client of the AMIS provider DOS-HOST/HOSTEXEC (AL=10h).
; Command tail: " <m> <command>" with m = C (current DOS directory) or D (the
; directory SUB). Finds the provider by scanning AMIS, runs the command and
; prints "RC=<AX in hex> CF=<flag>".  Without the provider: "NOT FOUND".
        cpu 186
        org 100h
        cld
        xor bx, bx
scan:   mov ah, bl
        xor al, al
        int 2Dh
        cmp al, 0FFh
        jne next
        mov es, dx
        mov si, expect
        mov cx, 16
        repe cmpsb
        je found
next:   inc bl
        jnz scan
        mov dx, msgnf
        call puts
        mov ax, 4C01h
        int 21h
found:  mov [mux], bl
        push cs                 ; ES was the provider's segment during the scan
        pop es
        ; command = the tail after " <m> ": copy from 84h, NUL-terminated
        xor cx, cx
        mov cl, [80h]
        sub cx, 3               ; the tail minus " m "
        mov si, 84h
        mov di, cmdbuf
        rep movsb
        mov byte [di], 0
        mov ax, cs              ; parameter block: command, directory
        mov [pb + 2], ax
        mov word [pb], cmdbuf
        cmp byte [82h], 'D'
        jne cur
        mov [pb + 6], ax
        mov word [pb + 4], dirname
cur:    mov si, pb
        mov ah, [mux]
        mov al, 10h
        int 2Dh
        mov [r_ax], ax
        mov al, 0
        adc al, 0
        mov [r_cf], al
        mov dx, msgrc
        call puts
        mov ax, [r_ax]
        call hex16
        mov dx, msgcf
        call puts
        mov al, [r_cf]
        call hex8
        mov dx, msgcrlf
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

expect  db 'DOS-HOSTHOSTEXEC'
dirname db 'SUB', 0
msgrc   db 'RC=$'
msgcf   db ' CF=$'
msgnf   db 'NOT FOUND', 13, 10, '$'
msgcrlf db 13, 10, '$'
pb      dw 0, 0, 0, 0
mux     db 0
r_cf    db 0
r_ax    dw 0
cmdbuf  times 128 db 0
