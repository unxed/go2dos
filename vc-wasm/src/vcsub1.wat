;; vcsub1.wat — перевод процедур VC 4.05 в WAT. Образец правил vc-wasm/PLAN.md, §3.
;;
;; Источник: pts-vc405-port vc.asm (= VCSUB1.INC в ddanila/vc versions/4.05),
;; BSD-2-Clause, (c) Vsevolod Volkov; лицензия — ../LICENSE-VC.TXT.
;; Адреса процедур и адреса возврата CALL — из листинга JWasm (PLAN §2).
;; Поля модуля без обёртки: склеивает tools/build.py.
;; Экспорт по имени — временный, для тестов: после этапа A вход — vc_call(addr).

;; A48C HexCod PROC NEAR
;; Вход: AL; ES:DI, DF.  Выход: AL, DI.  Флаги-выход: —.  Сохраняет: —.
;; Метки: 0 — вход, 1 — HexCd1.
(func $HexCod (export "HexCod") (param $L i32)
  (loop $next
    (block $L1
      (block $L0
        (br_table $L0 $L1 (local.get $L)))
      ;; --- 0: вход
      (call $set_al (i32.and (call $al) (i32.const 0x0F)))     ;; AND  AL,0Fh
      (call $set_al (i32.add (call $al) (i32.const 0x30)))     ;; ADD  AL,'0'
      (if (i32.le_u (call $al) (i32.const 0x39))               ;; CMP  AL,'9'
        (then (local.set $L (i32.const 1)) (br $next)))        ;; JBE  HexCd1
      (call $set_al (i32.add (call $al) (i32.const 7))))       ;; ADD  AL,7
    ;; --- 1: HexCd1
    (call $stosb)                                              ;; STOSB
    (global.set $di                                            ;; INC  DI
      (i32.and (i32.add (global.get $di) (i32.const 1)) (i32.const 0xFFFF)))
    (call $ret (i32.const 0)) (return)))                       ;; RET

;; A47D HexByt PROC NEAR
;; Вход: AL; ES:DI, DF.  Выход: AL, DI.  Флаги-выход: —.  Сохраняет: AH, CX.
;; Метки: 0 — вход (переходов нет, диспетчер не нужен).
(func $HexByt (export "HexByt") (param $L i32)
  (call $push (global.get $cx))                                ;; PUSH CX
  (call $push (global.get $ax))                                ;; PUSH AX
  (call $set_cl (i32.const 4))                                 ;; MOV  CL,4
  (call $set_al (i32.shr_u (call $al) (call $cl)))             ;; SHR  AL,CL (CL=4; флаги не читают)
  (call $push (i32.const 0xA486)) (call $HexCod (i32.const 0)) ;; CALL HexCod
  (global.set $ax (call $pop))                                 ;; POP  AX
  (call $push (i32.const 0xA48A)) (call $HexCod (i32.const 0)) ;; CALL HexCod
  (global.set $cx (call $pop))                                 ;; POP  CX
  (call $ret (i32.const 0)) (return))                          ;; RET

;; A4B4 TxtNum PROC NEAR
;; Вход: ES:SI.  Выход: AX, SI, DF=0.  Флаги-выход: CF (1 — нет цифр).  Сохраняет: BX, DX.
;; Метки: 0 — вход, 1 — TxtNm1, 2 — TxtNm2, 3 — TxtNm3.
(func $TxtNum (export "TxtNum") (param $L i32)
  (local $a i32) (local $p i32)
  (loop $next
    (block $L3
      (block $L2
        (block $L1
          (block $L0
            (br_table $L0 $L1 $L2 $L3 (local.get $L)))
          ;; --- 0: вход
          (call $push (global.get $dx))                        ;; PUSH DX
          (call $push (global.get $bx))                        ;; PUSH BX
          (call $setf (i32.const 0x400) (i32.const 0))         ;; CLD
          (call $push (global.get $si))                        ;; PUSH SI
          (global.set $bx (i32.const 0)))                      ;; XOR  BX,BX (флаги не читают)
        ;; --- 1: TxtNm1
        (call $lodsb (global.get $es))                         ;; LODS ES:Byt
        (local.set $a (call $al))
        (call $set_al (i32.sub (local.get $a) (i32.const 0x30))) ;; SUB AL,'0'
        (if (i32.lt_u (local.get $a) (i32.const 0x30))         ;; JB   TxtNm3
          (then (local.set $L (i32.const 3)) (br $next)))
        (if (i32.gt_u (call $al) (i32.const 9))                ;; CMP  AL,9
          (then (local.set $L (i32.const 3)) (br $next)))      ;; JA   TxtNm3
        (call $push (global.get $ax))                          ;; PUSH AX
        (global.set $ax (i32.const 10))                        ;; MOV  AX,10
        (local.set $p (i32.mul (global.get $ax) (global.get $bx))) ;; MUL BX
        (global.set $ax (i32.and (local.get $p) (i32.const 0xFFFF)))
        (global.set $dx (i32.shr_u (local.get $p) (i32.const 16)))
        (global.set $bx (global.get $ax))                      ;; MOV  BX,AX
        (global.set $ax (call $pop))                           ;; POP  AX
        (if (i32.ne (global.get $dx) (i32.const 0))            ;; OR   DX,DX
          (then (local.set $L (i32.const 2)) (br $next)))      ;; JNE  TxtNm2
        (global.set $ax                                        ;; CBW
          (i32.and (i32.extend8_s (call $al)) (i32.const 0xFFFF)))
        (local.set $p (i32.add (global.get $bx) (global.get $ax))) ;; ADD BX,AX
        (global.set $bx (i32.and (local.get $p) (i32.const 0xFFFF)))
        (if (i32.le_u (local.get $p) (i32.const 0xFFFF))       ;; JNC  TxtNm1
          (then (local.set $L (i32.const 1)) (br $next))))
      ;; --- 2: TxtNm2
      (global.set $bx (i32.const 0xFFFF))                      ;; MOV  BX,0FFFFh
      (local.set $L (i32.const 1)) (br $next))                 ;; JMP  TxtNm1
    ;; --- 3: TxtNm3
    (global.set $si                                            ;; DEC  SI
      (i32.and (i32.sub (global.get $si) (i32.const 1)) (i32.const 0xFFFF)))
    (global.set $ax (call $pop))                               ;; POP  AX
    (call $setf (i32.const 1)                                  ;; CMP  AX,SI / CMC: выход — только CF
      (i32.ge_u (global.get $ax) (global.get $si)))
    (global.set $ax (global.get $bx))                          ;; MOV  AX,BX
    (global.set $bx (call $pop))                               ;; POP  BX
    (global.set $dx (call $pop))                               ;; POP  DX
    (call $ret (i32.const 0)) (return)))                       ;; RET
