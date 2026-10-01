; crit24.com - checks the INT 24h critical error handler.
; Command tail " <r><m>[P]": r = the handler's answer (0 ignore, 1 retry,
; 2 abort, 3 fail, 9 no handler of its own); m = O open D:\X.TXT, C create D:\NEW.TXT, F free space
; of drive D (36h). With P the program runs itself as a child (EXEC), then
; prints the child's exit type and code (4Dh).
; The handler prints what it was called with: "H AH=.. AL=.. DI=.... AX=....
; BP=.... SI=.... N=.." (AX is the original AX of the INT 21h call, taken
; from the stack as RBIL describes). It answers r, except that Retry becomes
; Fail on the second call.
        org 100h
start:  mov al, [82h]
        sub al, '0'
        mov [resp], al
        cmp byte [83h], 'P'
        je parent
        cmp byte [resp], 9      ; r=9: no handler of its own (the kernel's)
        je noinst
        mov ax, 2524h
        mov dx, handler
        int 21h
noinst: cmp byte [83h], 'C'
        je create
        cmp byte [83h], 'F'
        je free
        mov ax, 3D00h           ; open for reading
        mov dx, fname
        int 21h
        jmp report
create: mov ax, 3C00h
        xor cx, cx
        mov dx, newname
        int 21h
        jmp report
free:   mov ax, 3600h
        mov dl, 4               ; drive D
        int 21h
        mov [r_ax], ax
        mov dx, msgfree
        call puts
        mov ax, [r_ax]
        call hex16
        jmp crlf_exit
report: mov [r_ax], ax
        mov al, 0
        adc al, 0               ; CF
        mov [r_cf], al
        mov dx, msgopen
        call puts
        mov al, [r_cf]
        call hex8
        mov dx, msgax
        call puts
        mov ax, [r_ax]
        call hex16
crlf_exit:
        mov dx, crlf
        call puts
        mov ax, 4C00h
        int 21h

parent: mov bx, 1000h           ; shrink to 64 KiB for the child
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
        call puts
        mov ax, [r_ax]
        call hex16
        mov dx, crlf
        call puts
        mov ax, 4C00h
        int 21h
pfail:  mov ax, 4C01h
        int 21h

handler:
        mov [h_ah], ah
        mov [h_al], al
        mov [h_di], di
        mov [h_bp], bp
        mov [h_si], si
        push bp
        mov bp, sp
        mov ax, [bp + 8]        ; bp, ip, cs, flags, then AX of the INT 21h call
        mov [h_ax], ax
        pop bp
        inc byte [calls]
        mov dx, msgh
        call puts
        mov al, [h_ah]
        call hex8
        mov dx, msgal
        call puts
        mov al, [h_al]
        call hex8
        mov dx, msgdi
        call puts
        mov ax, [h_di]
        call hex16
        mov dx, msgax
        call puts
        mov ax, [h_ax]
        call hex16
        mov dx, msgbp
        call puts
        mov ax, [h_bp]
        call hex16
        mov dx, msgsi
        call puts
        mov ax, [h_si]
        call hex16
        mov dx, msgn
        call puts
        mov al, [calls]
        call hex8
        mov dx, crlf
        call puts
        mov al, [resp]
        cmp al, 1
        jne .ret
        cmp byte [calls], 2
        jb .ret
        mov al, 3
.ret:   iret

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

fname    db 'D:\X.TXT', 0
newname  db 'D:\NEW.TXT', 0
selfname db 'CRIT24.COM', 0
msgh     db 'H AH=$'
msgal    db ' AL=$'
msgdi    db ' DI=$'
msgax    db ' AX=$'
msgbp    db ' BP=$'
msgsi    db ' SI=$'
msgn     db ' N=$'
msgopen  db 'OPEN CF=$'
msgfree  db 'FREE AX=$'
msgexit  db 'EXIT AX=$'
crlf     db 13, 10, '$'
tailbuf  db 2, ' ', '0', 13
pb       dw 0, tailbuf, 0, 5Ch, 0, 6Ch, 0
resp     db 0
calls    db 0
r_cf     db 0
h_ah     db 0
h_al     db 0
h_di     dw 0
h_bp     dw 0
h_si     dw 0
h_ax     dw 0
r_ax     dw 0
