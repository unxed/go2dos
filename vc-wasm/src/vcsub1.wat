;; vcsub1.wat — ручной (LLM) перевод процедур Volkov Commander 4.05 в WebAssembly.
;;
;; Источник: github.com/ddanila/vc, versions/4.05/VCSUB1.INC (BSD-2-Clause,
;; (c) Vsevolod Volkov; текст лицензии — ../LICENSE-VC.TXT).
;; Переведены: HexCod, HexByt, TxtNum. Соглашения перевода (D1-D6) — в ../README.md.
;;
;; D1. Память. Один сегмент tiny-модели занимает адреса 0x0000-0xFFFF; окно
;;     текстового экрана (сегмент B800h) отображено с 0x10000. Базу сегмента,
;;     через который идёт обращение (ES:), процедура получает параметром $es.
;; D2. Регистры. Читаемые процедурой регистры — параметры, меняемые и видимые
;;     вызывающему — результаты (несколько значений), CF — последний результат
;;     (0 или 1). Сохраняемые через PUSH/POP регистры в результаты не входят.
;; D3. Флаги вычисляются только там, где их потом читает код (см. комментарии).
;; D4. Переходы — структурные block/loop, без диспетчера.
;; D6. Особенности оригинала (насыщение числа, выходные флаги) сохранены, а не
;;     «исправлены».

(module
  (memory (export "memory") 2)

  ;; HexCod: AL (младший полубайт) -> ASCII-цифра в ES:[DI], DI += 2.
  ;;   AND AL,0Fh / ADD AL,'0' / CMP AL,'9' / JBE / ADD AL,7 / STOSB / INC DI
  ;; Вход: $al, $es, $di. Выход: AL (записанный символ), DI.
  ;; Предполагает DF=0 (STOSB идёт вперёд). Флаги на выходе не моделируются
  ;; (по местам вызова не проверено, см. README, «Сомнения»).
  (func $hexcod (export "hexcod")
        (param $al i32) (param $es i32) (param $di i32) (result i32 i32)
    (local $c i32)
    (local.set $c
      (i32.add (i32.and (local.get $al) (i32.const 0x0F)) (i32.const 0x30)))
    (if (i32.gt_u (local.get $c) (i32.const 0x39))
      (then (local.set $c (i32.add (local.get $c) (i32.const 7)))))
    ;; STOSB
    (i32.store8 (i32.add (local.get $es) (local.get $di)) (local.get $c))
    ;; результаты: AL, DI+2 (STOSB: DI+1, затем INC DI — пропуск байта атрибута)
    (local.get $c)
    (i32.and (i32.add (local.get $di) (i32.const 2)) (i32.const 0xFFFF)))

  ;; HexByt: байт AL -> две шестнадцатеричные цифры в ES:[DI] и ES:[DI+2].
  ;;   PUSH CX / PUSH AX / MOV CL,4 / SHR AL,CL / CALL HexCod / POP AX /
  ;;   CALL HexCod / POP CX
  ;; Выход: AL (последний записанный символ), DI+4. AH и CX сохраняются.
  (func $hexbyt (export "hexbyt")
        (param $al i32) (param $es i32) (param $di i32) (result i32 i32)
    ;; старший полубайт
    (call $hexcod
      (i32.shr_u (i32.and (local.get $al) (i32.const 0xFF)) (i32.const 4))
      (local.get $es)
      (local.get $di))
    local.set $di   ;; DI после первой цифры
    drop            ;; AL первой цифры затирается POP AX
    ;; младший полубайт (HexCod сам делает AND AL,0Fh)
    (call $hexcod (local.get $al) (local.get $es) (local.get $di)))

  ;; TxtNum: десятичное число из строки ES:SI (тип байта — LODS ES:Byt).
  ;; Чтение до первого символа вне '0'..'9'. Число насыщается на 65535.
  ;; Вход: $es, $si. Выход: AX (число), SI (адрес первого символа вне числа),
  ;; CF (1 — ни одной цифры). DX и BX сохраняются, DF сбрасывается (CLD) —
  ;; этот побочный эффект не моделируется.
  (func $txtnum (export "txtnum")
        (param $es i32) (param $si i32) (result i32 i32 i32)
    (local $orig i32) (local $bx i32) (local $b i32) (local $p i32)
    ;; PUSH SI ; XOR BX,BX
    (local.set $orig (local.get $si))
    (block $done
      (loop $next
        ;; TxtNm1: LODS ES:Byt
        (local.set $b
          (i32.load8_u (i32.add (local.get $es) (local.get $si))))
        (local.set $si
          (i32.and (i32.add (local.get $si) (i32.const 1)) (i32.const 0xFFFF)))
        ;; SUB AL,'0' ; JB TxtNm3
        (br_if $done (i32.lt_u (local.get $b) (i32.const 0x30)))
        (local.set $b (i32.sub (local.get $b) (i32.const 0x30)))
        ;; CMP AL,9 ; JA TxtNm3
        (br_if $done (i32.gt_u (local.get $b) (i32.const 9)))
        ;; PUSH AX ; MOV AX,10 ; MUL BX ; MOV BX,AX ; POP AX
        (local.set $p (i32.mul (local.get $bx) (i32.const 10)))
        (local.set $bx (i32.and (local.get $p) (i32.const 0xFFFF)))
        ;; OR DX,DX ; JNE TxtNm2   (DX = старшее слово произведения)
        ;; TxtNm2: MOV BX,0FFFFh ; JMP TxtNm1
        (if (i32.gt_u (local.get $p) (i32.const 0xFFFF))
          (then
            (local.set $bx (i32.const 0xFFFF))
            (br $next)))
        ;; CBW ; ADD BX,AX ; JNC TxtNm1   (при переносе — TxtNm2)
        (local.set $bx (i32.add (local.get $bx) (local.get $b)))
        (if (i32.gt_u (local.get $bx) (i32.const 0xFFFF))
          (then (local.set $bx (i32.const 0xFFFF))))
        (br $next)))
    ;; TxtNm3: DEC SI
    (local.set $si
      (i32.and (i32.sub (local.get $si) (i32.const 1)) (i32.const 0xFFFF)))
    ;; POP AX ; CMP AX,SI ; CMC ; MOV AX,BX ; POP BX ; POP DX ; RET
    ;; CF после CMC = 1 тогда и только тогда, когда исходный SI >= SI (цифр не было)
    (local.get $bx)
    (local.get $si)
    (i32.ge_u (local.get $orig) (local.get $si)))
)
