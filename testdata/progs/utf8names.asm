; utf8names.com - UTF-8 file names (docs/UTF8NAMES.md). Finds the provider
; "DOS-UTF8" "NAMES   " through AMIS, turns UTF-8 on (AL=10h BX=65001) and
; finds the first file with INT 21h AX=714Eh; prints the long name as hex bytes
; and the short name as text. Then it turns UTF-8 off and prints the long name
; again, and finally it searches for a pattern with an invalid UTF-8 byte.
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
        mov ah, [mux]           ; UTF-8 on
        mov al, 10h
        mov bx, 65001
        int 2Dh
        mov [r_ax], ax
        mov [r_bx], bx
        mov dx, msgset
        call puts
        mov al, [r_ax]
        call hex8
        mov dx, msgprev
        call puts
        mov ax, [r_bx]
        call hex16
        call crlf
        call findfirst
        mov dx, msgname
        call puts
        call hexname
        call crlf
        mov dx, msgshort
        call puts
        mov si, findbuf + 130h  ; the short name, ASCIIZ
.sh:    lodsb
        or al, al
        jz .shd
        mov dl, al
        mov ah, 2
        int 21h
        jmp .sh
.shd:   call crlf
        mov ah, [mux]           ; current mode
        mov al, 11h
        int 2Dh
        mov [r_bx], bx
        mov dx, msgmode
        call puts
        mov ax, [r_bx]
        call hex16
        call crlf
        mov ah, [mux]           ; UTF-8 off
        mov al, 10h
        xor bx, bx
        int 2Dh
        call findfirst
        mov dx, msgname0
        call puts
        call hexname
        call crlf
        mov ah, [mux]           ; UTF-8 on again, bad pattern
        mov al, 10h
        mov bx, 65001
        int 2Dh
        mov ax, 714Eh
        xor cx, cx
        mov dx, badpat
        mov si, 1
        mov di, findbuf
        int 21h
        mov [r_ax], ax
        mov al, 0
        adc al, 0
        mov [r_cf], al
        mov dx, msgbad
        call puts
        mov al, [r_cf]
        call hex8
        mov ax, [r_ax]
        call hex16
        call crlf
        mov ax, 4C00h
        int 21h

findfirst:
        mov ax, 714Eh
        xor cx, cx
        mov dx, pattern
        mov si, 1
        mov di, findbuf
        int 21h
        jnc .ok
        mov ax, 4C02h
        int 21h
.ok:    ret

hexname:
        mov si, findbuf + 2Ch   ; the long name, ASCIIZ
.n:     lodsb
        or al, al
        jz .d
        call hex8
        jmp .n
.d:     ret

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

expect  db 'DOS-UTF8NAMES   '
pattern db '*.TXT', 0
badpat  db 0FFh, '.TXT', 0
msgset  db 'SET=$'
msgprev db ' PREV=$'
msgname db 'NAME=$'
msgshort db 'SHORT=$'
msgmode db 'MODE=$'
msgname0 db 'NAME0=$'
msgbad  db 'BAD=$'
msgnf   db 'NOT FOUND', 13, 10, '$'
msgcrlf db 13, 10, '$'
mux     db 0
r_cf    db 0
r_ax    dw 0
r_bx    dw 0
findbuf times 320 db 0
