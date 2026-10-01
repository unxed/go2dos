#!/bin/sh
# Собирает перевод VC в C в wasm32: vc-wasm/c/build/vc.wasm.
# Нужны clang и wasm-ld (проверено на LLVM 18). Запуск из корня репозитория или откуда угодно.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
out=$here/build
mkdir -p "$out"
# Арена (1 МиБ, всё адресное пространство 8086) — глобальный массив C; память модуля — 2 МиБ.
clang --target=wasm32 -Os -std=c11 -Wall -Werror -ffreestanding -nostdlib \
  -fno-builtin -Wl,--no-entry -Wl,--initial-memory=2097152 \
  -o "$out/vc.wasm" "$here"/*.c
echo "build.sh: $(wc -c < "$out/vc.wasm") байт -> $out/vc.wasm"
