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
