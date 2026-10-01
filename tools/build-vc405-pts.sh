#!/bin/sh
# VC 4.05: VC.COM побайтно как у TASM, из pts-vc405-port (T15a, гейт G1 из
# docs/ASM-GATES.md).
#
#   tools/build-vc405-pts.sh [OUTDIR]
#
# Кладёт OUTDIR/bin/4.05/VC.COM (OUTDIR по умолчанию .cache/vc-build) и сверяет
# его SHA-256 с REF_405_VC_COM из third_party/vc/PINS (сборка TASM). Берёт JWasm
# 2.11a, вшитый в pts-vc405-port (tools/jwasm-2.11a.upx), с теми же ключами, что
# в его compile.sh. Сам compile.sh не вызываем: по умолчанию он падает на
# `cmp golden/vcsetup.com` (такого файла в golden/ нет), см. docs/DOUBTS.md.
# Нужны: git, sha256sum, запуск 32-битных ELF (Linux i386/amd64).
set -eu

here=$(cd "$(dirname "$0")" && pwd)
root=$(dirname "$here")
tp=$root/third_party/vc
out=${1:-$root/.cache/vc-build}
mkdir -p "$out"
out=$(cd "$out" && pwd)

# shellcheck disable=SC1091
. "$tp/PINS"

die() { echo "build-vc405-pts: $*" >&2; exit 1; }
for t in git sha256sum; do
  command -v "$t" >/dev/null 2>&1 || die "нет программы: $t"
done

src=$out/pts-vc405-port
if [ ! -d "$src/.git" ]; then
  git init -q "$src"
  git -C "$src" remote add origin "$PTS_REPO"
fi
git -C "$src" config core.autocrlf false
if [ "$(git -C "$src" rev-parse -q --verify HEAD 2>/dev/null || true)" != "$PTS_COMMIT" ]; then
  git -C "$src" fetch -q --depth 1 origin "$PTS_COMMIT"
  git -C "$src" checkout -q --force --detach FETCH_HEAD
fi
[ "$(git -C "$src" rev-parse HEAD)" = "$PTS_COMMIT" ] || die "коммит pts-vc405-port не $PTS_COMMIT"

jwasm=$src/tools/jwasm-2.11a.upx
[ -f "$jwasm" ] || die "нет $jwasm"
chmod +x "$jwasm"

work=$out/work/4.05-pts
rm -rf "$work"
mkdir -p "$work"
(cd "$src" && "$jwasm" -q -Cp -WX -e999999999 -bin -Fo"$work/VC.COM" vc.asm) \
  || die "JWasm (вшитый в pts-vc405-port) вернул ошибку"

got=$(sha256sum "$work/VC.COM" | cut -d' ' -f1)
[ "$got" = "$REF_405_VC_COM" ] \
  || die "G1 не пройден: sha256 VC.COM $got, ожидался $REF_405_VC_COM (эталон TASM)"

mkdir -p "$out/bin/4.05"
cp "$work/VC.COM" "$out/bin/4.05/VC.COM"
echo "== VC 4.05 VC.COM побайтно как у TASM (pts-vc405-port $PTS_COMMIT)"

# --- VC 4.05 с клиентом буфера обмена (T15b): наш патч поверх vc.asm ----------
# Не побайтно как у TASM (это и есть правка), поэтому отдельный каталог bin/4.05-clip.
# Гейты: G3 (размер) здесь; G2 (NASM) и G4 (листинги) не автоматизированы, см. DOUBTS.md.
clip=$out/work/4.05-clip
rm -rf "$clip"
mkdir -p "$clip"
cp "$src/vc.asm" "$clip/"
# GIT_CEILING_DIRECTORIES: внутри чужого репозитория (наш .cache) git apply молча
# пропускает файлы вне корня («Skipped patch»), поэтому git выше $clip не ищем.
(cd "$clip" && GIT_CEILING_DIRECTORIES=$(dirname "$clip") git apply -p1 "$tp/patches/vc-4.05-clip.patch") \
  || die "patches/vc-4.05-clip.patch не применился к vc.asm из pts-vc405-port $PTS_COMMIT"
cmp -s "$src/vc.asm" "$clip/vc.asm" && die "patches/vc-4.05-clip.patch ничего не изменил (vc.asm совпал с pts)"
(cd "$clip" && "$jwasm" -q -Cp -WX -e999999999 -bin -Fo"$clip/VC.COM" vc.asm) \
  || die "JWasm не собрал VC 4.05 с буфером обмена"
csize=$(wc -c <"$clip/VC.COM" | tr -d ' ')
[ "$csize" -le 65280 ] || die "G3 не пройден: VC.COM с буфером обмена $csize байт, предел 65280"
mkdir -p "$out/bin/4.05-clip"
cp "$clip/VC.COM" "$out/bin/4.05-clip/VC.COM"
echo "== VC 4.05 с буфером обмена: $csize байт (запас до 65280: $((65280 - csize)))"

# Модуль расширений VCEXT.BIN (T15c): third_party/vc/ext/vcext.asm, не больше 2048 байт.
"$jwasm" -q -bin -Fo"$out/bin/4.05-clip/VCEXT.BIN" "$tp/ext/vcext.asm" \
  || die "JWasm не собрал VCEXT.BIN"
xsize=$(wc -c <"$out/bin/4.05-clip/VCEXT.BIN" | tr -d ' ')
[ "$xsize" -le 2048 ] || die "VCEXT.BIN $xsize байт, предел 2048 (раскладка блока в vcext.asm)"
echo "== VCEXT.BIN: $xsize байт (предел 2048)"
