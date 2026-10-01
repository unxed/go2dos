; VCEXT.BIN - extension module for Volkov Commander 4.05 (format VCX1, T15c).
;
; VC.COM loads this file at start (see vc-4.05-clip.patch, ExtLoad) into a 4 KB
; block and calls the entries with CALL FAR. Without the file VC works as before.
;
; Layout of the block (segment = the module's CS):
;   0000h..07FFh  this file (header + code, at most 2048 bytes)
;   0800h..0FFFh  Buf: the text of the clipboard.  Before the module starts the
;                 loader keeps the path of the file at 0C00h; it is not needed
;                 afterwards, so Buf may cover it.
; Header: 'VCX1', then entries: 3 bytes each (JMP NEAR) at 4, 7.
;
; Both entries get the key in AX (AH = scan code: 52h Ins, 53h Del, 92h Ctrl-Ins; AL=0)
; and return CF=1 when the key is not theirs (no Shift for Ins/Del, no clipboard
; server): the caller then does what the key did before.  Registers other than
; AX, DX (and CX in Text paste) are preserved.  Shift is read with INT 16h AH=02h.
;
;   4  Field  line editing (EdLin): ES:DI - ASCIIZ line (field of CX chars, CX+1 bytes),
;             DX - cursor, CX - max length.
;             Ctrl-Ins  copy the line                     out: CF=0, AL=0
;             Shift-Ins paste the first line at DX        out: CF=0, AL=1, DX - cursor
;             Shift-Del copy the line and clear it        out: CF=0, AL=1, DX=0
;   7  Text   the text editor: DS=ES - VC's segment, SI - the line (CX bytes, not ASCIIZ),
;             DI - UndoBuf (the line to be pasted goes there).
;             Ctrl-Ins  copy the line                     out: CF=0, AL=0
;             Shift-Del copy the line                     out: CF=0, AL=1 (VC then does ^Y)
;             Shift-Ins the first line of the clipboard + CR LF to ES:DI
;                                                         out: CF=0, AL=2, CX - length (VC then does ^U)
; The clipboard keeps at most ClipMax-1 characters of a line.
; Clipboard: INT 2Fh AX=17xxh (WinOldAp), format 7 (CF_OEMTEXT).

	.8086
	.MODEL	TINY
	.CODE
	ORG	0

ClipMax	EQU	2048			; The whole Buf
Buf	EQU	800h

	DB	'VCX1'
	JMP	NEAR PTR X_Field	; 4
	JMP	NEAR PTR X_Text		; 7

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

; Shift pressed?  ZF=0 - yes.  Keeps AX.
X_Shift:
	PUSH	AX
	MOV	AH,2
	INT	16h
	TEST	AL,3
	POP	AX
	RET

; Put the text DS:SI (CX bytes at most, up to a 0) on the clipboard.  An empty text
; is not put.  CF=1 - no clipboard.  Destroys: AX, DX, flags.
X_Put:	PUSH	BX
	PUSH	CX
	PUSH	SI
	PUSH	DI
	PUSH	ES
	CLD
	PUSH	CS
	POP	ES
	MOV	DI,Buf
	CMP	CX,ClipMax-1
	JBE	XU_01
	MOV	CX,ClipMax-1
XU_01:	JCXZ	XU_03
XU_02:	LODSB
	OR	AL,AL
	JE	XU_03
	STOSB
	LOOP	XU_02
XU_03:	XOR	AL,AL
	STOSB
	MOV	CX,DI
	SUB	CX,Buf			; Length with the final 0
	CMP	CX,1
	JE	XU_09			; Empty (CF=0)
	CALL	X_Open
	JC	XU_09
	MOV	AX,1702h		; Empty
	CALL	X_Int
	MOV	AX,1703h		; Set: DX - format, ES:BX - data, SI:CX - size
	MOV	BX,Buf
	MOV	DX,7
	XOR	SI,SI
	CALL	X_Int
	MOV	AX,1708h		; Close
	CALL	X_Int
	CLC
XU_09:	POP	ES
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	RET

; The first line of the clipboard text -> CS:Buf.  Out: AX - its length.  CF=1 - no
; clipboard or no text.  Keeps all but AX, flags.
X_Get:	PUSH	BX
	PUSH	CX
	PUSH	DX
	PUSH	ES
	CALL	X_Open
	JC	XG_09
	MOV	AX,1704h		; Size: DX:AX
	MOV	DX,7
	CALL	X_Int
	OR	DX,DX
	JNE	XG_08
	CMP	AX,1
	JBE	XG_08
	CMP	AX,ClipMax
	JA	XG_08
	PUSH	CS
	POP	ES
	MOV	BX,Buf
	MOV	AX,1705h		; Get: ES:BX - buffer
	MOV	DX,7
	CALL	X_Int
XG_01:	CMP	BYTE PTR CS:[BX],' '	; End of the first line
	JB	XG_02
	INC	BX
	JMP	XG_01
XG_02:	SUB	BX,Buf
	MOV	AX,1708h		; Close
	CALL	X_Int
	MOV	AX,BX
	CLC
	JMP	XG_09
XG_08:	MOV	AX,1708h
	CALL	X_Int
	STC
XG_09:	POP	ES
	POP	DX
	POP	CX
	POP	BX
	RET

;---------------------------------------------------------------- Field
X_Field:
	PUSH	BX
	PUSH	CX
	PUSH	SI
	PUSH	DI
	PUSH	DS
	PUSH	AX
	PUSH	ES
	POP	DS			; DS = ES = the line's segment
	CMP	AH,92h
	JE	XF_01
	CALL	X_Shift
	JE	XF_def
	CMP	AH,52h
	JE	XF_04
	MOV	SI,DI			; Shift-Del: cut
	MOV	CX,ClipMax-1
	CALL	X_Put
	JC	XF_def
	MOV	BYTE PTR DS:[DI],0
	XOR	DX,DX
	MOV	AL,1
	JMP	XF_ok
XF_01:	MOV	SI,DI			; Ctrl-Ins: copy
	MOV	CX,ClipMax-1
	CALL	X_Put
	XOR	AL,AL
	JMP	XF_ok
XF_04:	CALL	X_Get			; Shift-Ins: paste.  AX - n
	JC	XF_def
	MOV	BX,AX
	MOV	SI,CX			; M - the max length
	SUB	SI,DX			; M - cursor
	JAE	XF_05
	XOR	SI,SI
XF_05:	CMP	BX,SI
	JBE	XF_06
	MOV	BX,SI			; n' = min (n, M - cursor)
XF_06:	SUB	SI,BX			; The tail: M - cursor - n' bytes move right by n'
	PUSH	CX
	MOV	CX,SI
	PUSH	DI
	ADD	DI,DX
	MOV	SI,DI
	ADD	SI,CX
	DEC	SI
	LEA	DI,[BX+SI]
	STD
	JCXZ	XF_07
	REP	MOVSB
XF_07:	CLD
	POP	DI
	PUSH	DI
	ADD	DI,DX
	XOR	SI,SI
	MOV	CX,BX
	JCXZ	XF_09
XF_08:	MOV	AL,CS:[SI+Buf]
	STOSB
	INC	SI
	LOOP	XF_08
XF_09:	ADD	DX,BX
	POP	DI
	POP	CX
	PUSH	DI
	ADD	DI,CX
	MOV	BYTE PTR DS:[DI],0	; The field's last byte is always 0
	POP	DI
	MOV	AL,1
XF_ok:	ADD	SP,2			; AL is the result; the saved AX is not needed
	POP	DS
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	CLC
	RETF
XF_def:	POP	AX
	POP	DS
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	STC
	RETF

;---------------------------------------------------------------- Text
X_Text:	PUSH	BX
	PUSH	CX
	PUSH	SI
	PUSH	DI
	PUSH	DS
	PUSH	AX
	CMP	AH,92h
	JE	XT_01
	CALL	X_Shift
	JE	XT_def
	CMP	AH,52h
	JE	XT_02
	CALL	X_Put			; Shift-Del
	JC	XT_def
	MOV	AL,1
	JMP	XT_ok
XT_01:	CALL	X_Put			; Ctrl-Ins
	XOR	AL,AL
	JMP	XT_ok
XT_02:	CALL	X_Get			; Shift-Ins: AX - n
	JC	XT_def
	MOV	CX,AX
	XOR	SI,SI
	JCXZ	XT_04
XT_03:	MOV	AL,CS:[SI+Buf]
	STOSB
	INC	SI
	LOOP	XT_03
XT_04:	MOV	AL,0Dh
	STOSB
	MOV	AL,0Ah
	STOSB
	MOV	DX,DI			; End of the line in UndoBuf
	ADD	SP,2			; The saved AX
	POP	DS
	POP	DI			; UndoBuf
	POP	SI
	SUB	DX,DI			; n + 2
	POP	CX			; The saved CX is dropped, CX is the result
	MOV	CX,DX
	POP	BX
	MOV	AL,2
	CLC
	RETF
XT_ok:	ADD	SP,2
	POP	DS
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	CLC
	RETF
XT_def:	POP	AX
	POP	DS
	POP	DI
	POP	SI
	POP	CX
	POP	BX
	STC
	RETF

	END
