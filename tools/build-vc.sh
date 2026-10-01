#!/bin/sh
# Воспроизводимая сборка Volkov Commander свободными средствами (без TASM).
# Подробности и ограничения: third_party/vc/README.md, docs/VC-BUILD.md.
#
#   tools/build-vc.sh [-c REFDIR] [OUTDIR]
#
# OUTDIR (по умолчанию .cache/vc-build):
#   src/            исходники ddanila/vc на закреплённом коммите (проверены)
#   jwasm-*/jwasm   JWasm, собранный из закреплённого коммита с нашим патчем
#   work/<версия>/  исходники после патчей
#   bin/4.05/       VC.COM VCSETUP.COM
#   bin/4.99.09/    VC.COM VC.OVL
# bin/ имеет ту же раскладку, что .cache/vc из tools/fetch-vc.sh, поэтому
#   GO2DOS_VC_DIR=$PWD/OUTDIR/bin go test ./e2e/
# Нужны: git, make, gcc, patch-совместимый git apply, sha256sum, cmp.
# -c REFDIR: каталог tools/fetch-vc.sh (бинарники, собранные TASM) — для
# побайтового сравнения; без него сравнение идёт только с SHA-256 из PINS.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$here")
tp=$root/third_party/vc
ref=
if [ "${1:-}" = "-c" ]; then ref=$2; shift 2; fi
out=${1:-$root/.cache/vc-build}
mkdir -p "$out"
out=$(cd "$out" && pwd)

# shellcheck disable=SC1091
. "$tp/PINS"

die() { echo "build-vc: $*" >&2; exit 1; }
for t in git make gcc sha256sum cmp; do
  command -v "$t" >/dev/null 2>&1 || die "нет программы: $t"
done

# --- 1. исходники VC на закреплённом коммите -------------------------------
src=$out/src
if [ ! -d "$src/.git" ]; then
  git init -q "$src"
  git -C "$src" remote add origin "$VC_REPO"
fi
git -C "$src" config core.autocrlf false
if [ "$(git -C "$src" rev-parse -q --verify HEAD 2>/dev/null || true)" != "$VC_COMMIT" ]; then
  git -C "$src" fetch -q --depth 1 origin "$VC_COMMIT"
  git -C "$src" checkout -q --force --detach FETCH_HEAD
fi
[ "$(git -C "$src" rev-parse HEAD)" = "$VC_COMMIT" ] || die "коммит VC не $VC_COMMIT"
(cd "$src" && sha256sum -c --quiet "$tp/sources.sha256") \
  || die "исходники VC не совпали с third_party/vc/sources.sha256"
echo "== исходники VC $VC_COMMIT проверены"

# --- 2. JWasm из исходников + патч совместимости с TASM ---------------------
patch_id=$(sha256sum "$tp/patches/jwasm-tasm-compat.patch" | cut -c1-12)
jdir=$out/jwasm-$(echo "$JWASM_COMMIT" | cut -c1-12)-$patch_id
jwasm=$jdir/jwasm
if [ ! -x "$jwasm" ]; then
  echo "== сборка JWasm $JWASM_COMMIT + patches/jwasm-tasm-compat.patch"
  jsrc=$jdir.src
  rm -rf "$jsrc"
  git init -q "$jsrc"
  git -C "$jsrc" remote add origin "$JWASM_REPO"
  git -C "$jsrc" config core.autocrlf false
  git -C "$jsrc" fetch -q --depth 1 origin "$JWASM_COMMIT"
  git -C "$jsrc" checkout -q --force --detach FETCH_HEAD
  [ "$(git -C "$jsrc" rev-parse HEAD)" = "$JWASM_COMMIT" ] || die "коммит JWasm не $JWASM_COMMIT"
  (cd "$jsrc" && git apply -p1 "$tp/patches/jwasm-tasm-compat.patch")
  if ! (cd "$jsrc" && make -f GccUnix.mak) >"$jdir.log" 2>&1; then
    tail -n 30 "$jdir.log" >&2
    die "JWasm не собрался (полный журнал: $jdir.log)"
  fi
  mkdir -p "$jdir"
  # как в ddanila/vc third_party/jwasm/README.md: родной шаг компоновки
  # Makefile нужен не всегда, поэтому линкуем объекты сами
  (cd "$jsrc" && gcc build/GccUnixR/*.o -s -o "$jwasm")
  rm -rf "$jsrc"
fi
echo "== JWasm: $("$jwasm" -? 2>&1 | head -n 1)"

# --- 3. сборка ---------------------------------------------------------------
fresh() { rm -rf "$1" && mkdir -p "$1"; }
asm() { # asm ВЕРСИЯ ФАЙЛ.ASM ВЫХОД ОПЦИИ...
  _v=$1; _f=$2; _o=$3; shift 3
  (cd "$out/work/$_v" && "$jwasm" -q "$@" -Fo "$out/bin/$_v/$_o" "$_f") \
    || die "$_v/$_f: JWasm вернул ошибку"
}

fresh "$out/bin"
mkdir -p "$out/bin/4.05" "$out/bin/4.99.09"

echo "== VC 4.05"
fresh "$out/work/4.05"
cp "$src"/versions/4.05/* "$out/work/4.05/"
(cd "$out/work/4.05" && git apply -p1 "$tp/patches/vc-4.05-jwasm.patch")
asm 4.05 VC.ASM      VC.COM      -Zne -Zg -bin
asm 4.05 VCSETUP.ASM VCSETUP.COM -Zne -Zg -bin

echo "== VC 4.99.09"
fresh "$out/work/4.99.09"
cp "$src"/versions/4.99.09/* "$out/work/4.99.09/"
asm 4.99.09 VC.ASM    VC.COM -Zne -Zg -DOFFICIAL -bin
asm 4.99.09 VCOVL.ASM VC.OVL -Zne -Zg -DOFFICIAL -mz

# --- 4. сравнение с TASM -----------------------------------------------------
echo
echo "== сравнение с эталоном (собран TASM)"
bad=0
cmpone() { # cmpone ВЕРСИЯ ФАЙЛ ЭТАЛОННЫЙ_SHA СТРОГО(1|0)
  f=$out/bin/$1/$2
  got=$(sha256sum "$f" | cut -d' ' -f1)
  size=$(wc -c <"$f" | tr -d ' ')
  if [ "$got" = "$3" ]; then
    printf '%-16s %8s байт  ТОЖДЕСТВЕН эталону TASM\n' "$1/$2" "$size"
    return
  fi
  extra=
  r=
  if [ -n "$ref" ] && [ -f "$ref/$1/$2" ]; then r=$ref/$1/$2; fi
  if [ -n "$r" ]; then
    rs=$(wc -c <"$r" | tr -d ' ')
    nd=$(cmp -l "$f" "$r" 2>/dev/null | wc -l | tr -d ' ')
    extra="; эталон $rs байт, различающихся позиций до конца меньшего: $nd"
  fi
  printf '%-16s %8s байт  отличается от TASM%s\n' "$1/$2" "$size" "$extra"
  printf '%-16s sha256 %s\n' "" "$got"
  if [ "$4" = 1 ]; then bad=1; fi
}
cmpone 4.05    VC.COM      "$REF_405_VC_COM"      0
cmpone 4.05    VCSETUP.COM "$REF_405_VCSETUP_COM" 0
cmpone 4.99.09 VC.COM      "$REF_499_VC_COM"      1
cmpone 4.99.09 VC.OVL      "$REF_499_VC_OVL"      0
echo
if [ "$bad" = 1 ]; then
  die "4.99.09/VC.COM должен совпадать с TASM побайтно (README, раздел «Что установлено»)"
fi
echo "готово: $out/bin"
