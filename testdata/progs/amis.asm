; amis.com - finds an AMIS provider by its signature the way RBIL says:
; scan AH=00h..FFh with AL=00h, compare the first 16 bytes of the signature
; at DX:DI. Prints the multiplex number and version (AL=00h), the private
; function AL=10h (AX), the hook list of AL=04h (status and first interrupt
; number) and the uninstall status of AL=02h; "NOT FOUND" if no provider.
        org 100h
        cld
        xor bx, bx
scan:   mov ah, bl
        xor al, al
        int 2Dh
        cmp al, 0FFh
        jne next
        mov [ver], cx
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
        mov dx, msgmux
        call puts
        mov al, [mux]
        call hex8
        mov dx, msgver
        call puts
        mov ax, [ver]
        call hex16
        call crlf
        mov ah, [mux]           ; private function
        mov al, 10h
        int 2Dh
        mov [r_ax], ax
        mov dx, msgpriv
        call puts
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ah, [mux]           ; chained interrupts
        mov al, 4
        int 2Dh
        mov [r_ax], ax
        mov es, dx
        mov al, [es:bx]
        mov [first], al
        mov dx, msghook
        call puts
        mov al, [r_ax]
        call hex8
        mov dl, ' '
        mov ah, 2
        int 21h
        mov al, [first]
        call hex8
        call crlf
        mov ah, [mux]           ; uninstall
        mov al, 2
        int 2Dh
        mov [r_ax], ax
        mov dx, msgun
        call puts
        mov al, [r_ax]
        call hex8
        call crlf
        mov ax, 4C00h
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

expect  db 'go2dos  TESTPROV'
msgmux  db 'MUX=$'
msgver  db ' VER=$'
msgpriv db 'PRIV=$'
msghook db 'HOOKS=$'
msgun   db 'UNINST=$'
msgnf   db 'NOT FOUND', 13, 10, '$'
msgcrlf db 13, 10, '$'
mux     db 0
first   db 0
ver     dw 0
r_ax    dw 0
