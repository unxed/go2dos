; VCEXT.BIN - extension module for Volkov Commander 4.05 (format VCX1, T15c).
;
; VC.COM loads this file at start (see vc-4.05-clip.patch, ExtLoad) into a 4 KB
; block and calls the entries with CALL FAR. Without the file VC works as before.
;
; Layout of the block (segment = the module's CS):
;   0000h..07FFh  this file (header + code, at most 2048 bytes)
;   0800h..0BFFh  Buf: the text of the clipboard (first line is read by VC.COM)
;   0C00h..0FFFh  scratch of the loader (the path of the file), free afterwards
; Header: 'VCX1', then entries: 3 bytes each (JMP NEAR) at 4, 7, ...
;
; Entries (G5: what is not listed is preserved; flags are destroyed unless said):
;   4  Copy   in:  ES:DI - ASCIIZ line.  An empty line is not copied.
;             out: CF=1 - there is no clipboard server.
;   7  Fetch  out: CX - length of the first line of the clipboard text (0 - none),
;                  the text is at CS:Buf.  CF=1 - there is no clipboard server.
;                  Destroys: AX, BX, CX.
; Clipboard: INT 2Fh AX=17xxh (WinOldAp), format 7 (CF_OEMTEXT).

	.8086
	.MODEL	TINY
	.CODE
	ORG	0

ClipMax	EQU	1024			; Max size of the text to paste
Buf	EQU	800h

	DB	'VCX1'
	JMP	NEAR PTR X_Copy		; 4
	JMP	NEAR PTR X_Fetch	; 7

; INT 2Fh keeping all registers except AX, DX.
X_Int:	PUSH	BX
	PUSH	CX
	PUSH	SI
	PUSH	DI
	PUSH	BP
	PUSH	DS
	PUSH	ES
	INT	2Fh
	POP	ES
	POP	DS
	POP	BP
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	RET

; Open the clipboard.  CF=1 - no clipboard.  Destroys: AX, DX, flags.
X_Open:	MOV	AX,1700h
	CALL	X_Int
	CMP	AX,1700h
	JE	XO_01
	MOV	AX,1701h
	CALL	X_Int
	OR	AX,AX
	JNE	XO_02
XO_01:	STC
XO_02:	RET

X_Copy:	CMP	BYTE PTR ES:[DI],0
	JE	XC_09
	PUSH	AX
	PUSH	BX
	PUSH	CX
	PUSH	DX
	PUSH	SI
	PUSH	DI
	CALL	X_Open
	JC	XC_08
	MOV	BX,DI
	XOR	AL,AL
	MOV	CX,0FFFFh
	CLD
	REPNE	SCASB
	NOT	CX			; Length with the final 0
	MOV	AX,1702h		; Empty
	CALL	X_Int
	MOV	AX,1703h		; Set: DX - format, ES:BX - data, SI:CX - size
	MOV	DX,7
	XOR	SI,SI
	CALL	X_Int
	MOV	AX,1708h		; Close
	CALL	X_Int
	CLC
XC_08:	POP	DI
	POP	SI
	POP	DX
	POP	CX
	POP	BX
	POP	AX
XC_09:	RETF

X_Fetch:
	PUSH	DX
	PUSH	ES
	XOR	CX,CX
	CALL	X_Open
	JC	XF_09
	MOV	AX,1704h		; Size: DX:AX
	MOV	DX,7
	CALL	X_Int
	OR	DX,DX
	JNE	XF_08
	CMP	AX,1
	JBE	XF_08
	CMP	AX,ClipMax
	JA	XF_08
	PUSH	CS
	POP	ES
	MOV	BX,Buf
	MOV	BYTE PTR ES:[BX],0
	MOV	DX,7
	MOV	AX,1705h		; Get: ES:BX - buffer
	CALL	X_Int
XF_01:	CMP	BYTE PTR CS:[BX],' '	; End of the first line
	JB	XF_08
	INC	BX
	INC	CX
	JMP	XF_01
XF_08:	MOV	AX,1708h		; Close
	CALL	X_Int
	CLC
XF_09:	POP	ES
	POP	DX
	RETF

	END
