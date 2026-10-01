;; rt.wat — среда перевода Volkov Commander 4.05 в WAT. Правила — vc-wasm/PLAN.md, §3.
;;
;; Файлы src/*.wat содержат поля модуля без обёртки (module ...): их склеивает
;; в один модуль tools/build.py. Здесь — память, регистры, доступ к памяти,
;; стек, флаги и строковые команды; переведённый код пользуется только ими.

;; Адресное пространство реального режима (1 МиБ) — линейная память с адреса 0:
;; линейный адрес = seg*16 + off по модулю 1 МиБ, как у 8086.
(memory (export "memory") 16)

;; Регистры (значения всегда 0..FFFFh) и FLAGS — глобальные переменные.
;; Экспортируются для харнесса и тестов.
(global $ax (export "ax") (mut i32) (i32.const 0))
(global $cx (export "cx") (mut i32) (i32.const 0))
(global $dx (export "dx") (mut i32) (i32.const 0))
(global $bx (export "bx") (mut i32) (i32.const 0))
(global $sp (export "sp") (mut i32) (i32.const 0))
(global $bp (export "bp") (mut i32) (i32.const 0))
(global $si (export "si") (mut i32) (i32.const 0))
(global $di (export "di") (mut i32) (i32.const 0))
(global $es (export "es") (mut i32) (i32.const 0))
(global $cs (export "cs") (mut i32) (i32.const 0))
(global $ss (export "ss") (mut i32) (i32.const 0))
(global $ds (export "ds") (mut i32) (i32.const 0))
(global $flags (export "flags") (mut i32) (i32.const 0x0202))

;; --- память: rb/wb — байт, rw/ww — слово; слово по смещению FFFFh
;; переносится на смещение 0 того же сегмента, как у 8086.
(func $lin (param $s i32) (param $o i32) (result i32)
  (i32.and
    (i32.add (i32.shl (local.get $s) (i32.const 4)) (local.get $o))
    (i32.const 0xFFFFF)))
(func $rb (param $s i32) (param $o i32) (result i32)
  (i32.load8_u (call $lin (local.get $s) (local.get $o))))
(func $wb (param $s i32) (param $o i32) (param $v i32)
  (i32.store8 (call $lin (local.get $s) (local.get $o)) (local.get $v)))
(func $rw (param $s i32) (param $o i32) (result i32)
  (i32.or
    (call $rb (local.get $s) (local.get $o))
    (i32.shl
      (call $rb (local.get $s) (i32.and (i32.add (local.get $o) (i32.const 1)) (i32.const 0xFFFF)))
      (i32.const 8))))
(func $ww (param $s i32) (param $o i32) (param $v i32)
  (call $wb (local.get $s) (local.get $o) (local.get $v))
  (call $wb (local.get $s)
    (i32.and (i32.add (local.get $o) (i32.const 1)) (i32.const 0xFFFF))
    (i32.shr_u (local.get $v) (i32.const 8))))

;; --- половинки регистров
(func $al (result i32) (i32.and (global.get $ax) (i32.const 0xFF)))
(func $ah (result i32) (i32.shr_u (global.get $ax) (i32.const 8)))
(func $cl (result i32) (i32.and (global.get $cx) (i32.const 0xFF)))
(func $ch (result i32) (i32.shr_u (global.get $cx) (i32.const 8)))
(func $dl (result i32) (i32.and (global.get $dx) (i32.const 0xFF)))
(func $dh (result i32) (i32.shr_u (global.get $dx) (i32.const 8)))
(func $bl (result i32) (i32.and (global.get $bx) (i32.const 0xFF)))
(func $bh (result i32) (i32.shr_u (global.get $bx) (i32.const 8)))
(func $lo (param $r i32) (param $v i32) (result i32)
  (i32.or (i32.and (local.get $r) (i32.const 0xFF00)) (i32.and (local.get $v) (i32.const 0xFF))))
(func $hi (param $r i32) (param $v i32) (result i32)
  (i32.or (i32.and (local.get $r) (i32.const 0xFF))
          (i32.shl (i32.and (local.get $v) (i32.const 0xFF)) (i32.const 8))))
(func $set_al (param $v i32) (global.set $ax (call $lo (global.get $ax) (local.get $v))))
(func $set_ah (param $v i32) (global.set $ax (call $hi (global.get $ax) (local.get $v))))
(func $set_cl (param $v i32) (global.set $cx (call $lo (global.get $cx) (local.get $v))))
(func $set_ch (param $v i32) (global.set $cx (call $hi (global.get $cx) (local.get $v))))
(func $set_dl (param $v i32) (global.set $dx (call $lo (global.get $dx) (local.get $v))))
(func $set_dh (param $v i32) (global.set $dx (call $hi (global.get $dx) (local.get $v))))
(func $set_bl (param $v i32) (global.set $bx (call $lo (global.get $bx) (local.get $v))))
(func $set_bh (param $v i32) (global.set $bx (call $hi (global.get $bx) (local.get $v))))

;; --- стек: настоящий, в памяти по SS:SP (раскладка байт как у оригинала)
(func $push (param $v i32)
  (global.set $sp (i32.and (i32.sub (global.get $sp) (i32.const 2)) (i32.const 0xFFFF)))
  (call $ww (global.get $ss) (global.get $sp) (local.get $v)))
(func $pop (result i32)
  (local $v i32)
  (local.set $v (call $rw (global.get $ss) (global.get $sp)))
  (global.set $sp (i32.and (i32.add (global.get $sp) (i32.const 2)) (i32.const 0xFFFF)))
  (local.get $v))
;; RET n снимает адрес возврата и n байт: (call $ret (i32.const n)) (return).
;; RETF — (call $ret (i32.const 2)) (return) снимает и сегмент.
(func $ret (param $n i32)
  (global.set $sp
    (i32.and (i32.add (global.get $sp) (i32.add (i32.const 2) (local.get $n))) (i32.const 0xFFFF))))

;; --- флаги (маски: CF=1 PF=4 AF=10h ZF=40h SF=80h IF=200h DF=400h OF=800h)
(func $getf (param $m i32) (result i32)
  (i32.ne (i32.and (global.get $flags) (local.get $m)) (i32.const 0)))
(func $setf (param $m i32) (param $on i32)
  (global.set $flags
    (select
      (i32.or (global.get $flags) (local.get $m))
      (i32.and (global.get $flags) (i32.xor (local.get $m) (i32.const -1)))
      (local.get $on))))

;; --- строковые команды: шаг по DF. Источник — сегмент параметром (DS или
;; префикс), приёмник — всегда ES. Недостающие (MOVS, CMPS, SCAS, REP) — сюда.
(func $dstep (param $n i32) (result i32)
  (if (result i32) (call $getf (i32.const 0x400))
    (then (i32.sub (i32.const 0) (local.get $n)))
    (else (local.get $n))))
(func $stosb
  (call $wb (global.get $es) (global.get $di) (call $al))
  (global.set $di (i32.and (i32.add (global.get $di) (call $dstep (i32.const 1))) (i32.const 0xFFFF))))
(func $stosw
  (call $ww (global.get $es) (global.get $di) (global.get $ax))
  (global.set $di (i32.and (i32.add (global.get $di) (call $dstep (i32.const 2))) (i32.const 0xFFFF))))
(func $lodsb (param $seg i32)
  (call $set_al (call $rb (local.get $seg) (global.get $si)))
  (global.set $si (i32.and (i32.add (global.get $si) (call $dstep (i32.const 1))) (i32.const 0xFFFF))))
(func $lodsw (param $seg i32)
  (global.set $ax (call $rw (local.get $seg) (global.get $si)))
  (global.set $si (i32.and (i32.add (global.get $si) (call $dstep (i32.const 2))) (i32.const 0xFFFF))))
