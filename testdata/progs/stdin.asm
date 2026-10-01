; stdin.com - reads from stdin and writes to stdout
; demonstrates pipe mode with stdin/stdout
	org 100h

	; Read up to 100 bytes from stdin (handle 0)
	mov ah, 3Fh       ; INT 21h read from file
	mov bx, 0         ; handle 0 = stdin
	mov cx, 100       ; read up to 100 bytes
	mov dx, buffer    ; buffer at offset 'buffer'
	int 21h
	; ax now contains number of bytes read

	; Write those bytes to stdout (handle 1)
	mov cx, ax        ; cx = number of bytes read
	mov ah, 40h       ; INT 21h write to file
	mov bx, 1         ; handle 1 = stdout
	mov dx, buffer    ; buffer at offset 'buffer'
	int 21h

	; Exit with code 0
	mov ax, 4C00h
	int 21h

buffer:	db 100 dup(0)
