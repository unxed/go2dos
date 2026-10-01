/* vcsub1.c — перевод процедур VC 4.05 в C. Образец правил PLAN.md, §3.
 *
 * Источник: pts-vc405-port vc.asm (= VCSUB1.INC в ddanila/vc versions/4.05),
 * BSD-2-Clause, (c) Vsevolod Volkov; лицензия — ../LICENSE-VC.TXT.
 * Адреса и адреса возврата CALL — из листинга JWasm (PLAN §2).
 */
#include "vc.h"

Regs r;
uint8_t arena[ARENA_SIZE];

EXPORT(vc_regs) Regs *vc_regs(void) { return &r; }
EXPORT(vc_arena) uint8_t *vc_arena(void) { return arena; }

/* A48C HexCod PROC NEAR
 * Вход: AL; ES:DI, DF.  Выход: AL, DI.  Флаги-выход: —.  Сохраняет: —. */
EXPORT(HexCod) void HexCod(void) {
    SET_AL(AL() & 0x0F);                     /* AND  AL,0Fh  */
    SET_AL(AL() + '0');                      /* ADD  AL,'0'  */
    if (AL() <= '9') goto HexCd1;            /* CMP  AL,'9' / JBE HexCd1 */
    SET_AL(AL() + 7);                        /* ADD  AL,7    */
HexCd1:
    STOSB();                                 /* STOSB        */
    r.di += 1;                               /* INC  DI      */
    RET();                                   /* RET          */
}

/* A47D HexByt PROC NEAR
 * Вход: AL; ES:DI, DF.  Выход: AL, DI.  Флаги-выход: —.  Сохраняет: AH, CX. */
EXPORT(HexByt) void HexByt(void) {
    PUSH(r.cx);                              /* PUSH CX      */
    PUSH(r.ax);                              /* PUSH AX      */
    SET_CL(4);                               /* MOV  CL,4    */
    SET_AL(AL() >> CL());                    /* SHR  AL,CL   (флаги не читают) */
    CALL(HexCod, 0xA486);                    /* CALL HexCod  */
    r.ax = POP();                            /* POP  AX      */
    CALL(HexCod, 0xA48A);                    /* CALL HexCod  */
    r.cx = POP();                            /* POP  CX      */
    RET();                                   /* RET          */
}

/* A4B4 TxtNum PROC NEAR
 * Вход: ES:SI.  Выход: AX, SI, DF=0.  Флаги-выход: CF (1 — нет цифр).  Сохраняет: BX, DX. */
EXPORT(TxtNum) void TxtNum(void) {
    PUSH(r.dx);                              /* PUSH DX      */
    PUSH(r.bx);                              /* PUSH BX      */
    CLD();                                   /* CLD          */
    PUSH(r.si);                              /* PUSH SI      */
    r.bx = 0;                                /* XOR  BX,BX   (флаги не читают) */
TxtNm1:
    LODSB(r.es);                             /* LODS ES:Byt  */
    {
        uint8_t a = AL();
        SET_AL(a - '0');                     /* SUB  AL,'0'  */
        if (a < '0') goto TxtNm3;            /* JB   TxtNm3  */
    }
    if (AL() > 9) goto TxtNm3;               /* CMP  AL,9 / JA TxtNm3 */
    PUSH(r.ax);                              /* PUSH AX      */
    r.ax = 10;                               /* MOV  AX,10   */
    {
        uint32_t p = (uint32_t)r.ax * r.bx;  /* MUL  BX      */
        r.ax = (uint16_t)p;
        r.dx = (uint16_t)(p >> 16);
    }
    r.bx = r.ax;                             /* MOV  BX,AX   */
    r.ax = POP();                            /* POP  AX      */
    if (r.dx != 0) goto TxtNm2;              /* OR   DX,DX / JNE TxtNm2 */
    r.ax = (uint16_t)(int16_t)(int8_t)AL();  /* CBW          */
    {
        uint32_t s = (uint32_t)r.bx + r.ax;  /* ADD  BX,AX   */
        r.bx = (uint16_t)s;
        if (s <= 0xFFFF) goto TxtNm1;        /* JNC  TxtNm1  */
    }
TxtNm2:
    r.bx = 0xFFFF;                           /* MOV  BX,0FFFFh */
    goto TxtNm1;                             /* JMP  TxtNm1  */
TxtNm3:
    r.si -= 1;                               /* DEC  SI      */
    r.ax = POP();                            /* POP  AX      */
    SETF(CF, !(r.ax < r.si));                /* CMP  AX,SI / CMC (выход — только CF) */
    r.ax = r.bx;                             /* MOV  AX,BX   */
    r.bx = POP();                            /* POP  BX      */
    r.dx = POP();                            /* POP  DX      */
    RET();                                   /* RET          */
}
