# third_party/vc: сопровождаемая сборка Volkov Commander

Первый шаг этапа M9 (DESIGN §18): воспроизводимая сборка VC **без
проприетарных инструментов**, пока без наших изменений в самом VC. Все
результаты исследования (что чем собирается, чем отличается от TASM) —
в [docs/VC-BUILD.md](../../docs/VC-BUILD.md).

## Откуда исходники

- Репозиторий `https://github.com/ddanila/vc`, ветка `build`, коммит
  `7331dbb8e536692ec09147eb363d3d9b65478ec1` (он же — коммит, из которого
  собран релиз `latest-build`, откуда `tools/fetch-vc.sh` берёт готовые
  бинарники; совпадение проверено по телу релиза).
- В репозиторий go2dos исходники **пока не копируются**: `tools/build-vc.sh`
  получает их `git fetch` по SHA-1 коммита и проверяет каждый файл
  `versions/4.05` и `versions/4.99.09` по `sources.sha256`. Коммит и хэши
  закреплены в `PINS` и `sources.sha256`; менять их — только осознанно.
- Лицензия: BSD-2-Clause, Copyright 1991-2000 Vsevolod V. Volkov
  (`LICENSE` — копия `versions/*/LICENSE.TXT`, в обеих версиях текст
  совпадает). Происхождение и лицензии остального — в `NOTICE`.
- Ветка `master` ddanila/vc — историческая заморозка оригинальных архивов
  (`vc405.zip`, `vc49909.zip`); ветка `build` добавляет правки, чтобы 4.99.09
  собиралась JWasm, и тесты. Версия 4.05 в обеих ветках идентична.

## Что собирается и чем

| Версия | Файл | Как | Результат |
|---|---|---|---|
| 4.05 | `VC.COM` | JWasm 2.11a из pts-vc405-port (`tools/build-vc405-pts.sh`) | совпадает с TASM побайтно (проверено локально; проверка в скрипте и CI) |
| 4.05 | `VCSETUP.COM` | JWasm + наш патч исходников | работает, не побайтно как TASM |
| 4.99.09 | `VC.COM` | JWasm | **побайтно совпадает с TASM** |
| 4.99.09 | `VC.OVL` | JWasm `-mz` | собирается, не побайтно, не проверен в работе |

TASM 4.1 / TLINK 7.1 (в `ddanila/vc` — подмодуль `zajo/TASM`, исполняется под
MS-DOS 4.0 в QEMU) **не используются**: в том репозитории нет файла лицензии.
JWasm собирается из исходников (Sybase Open Watcom Public License v1.0) на
закреплённом коммите, готовых бинарников в репозитории нет.

## Что лежит здесь

- `PINS` — закреплённые коммиты (VC, JWasm) и SHA-256 эталонных бинарников
  TASM.
- `sources.sha256` — SHA-256 всех файлов `versions/` на закреплённом коммите.
- `patches/jwasm-tasm-compat.patch` — две правки JWasm (подробнее в
  docs/VC-BUILD.md): форма «аккумулятор, непосредственное» для
  `ADD/OR/ADC/SBB/AND/SUB/XOR/CMP AX,imm`, как у TASM; в модели Tiny
  `.DATA`/`.CONST` не сбрасывают `ASSUME CS`.
- `patches/vc-4.05-jwasm.patch` — перенос 4.05 с синтаксиса TASM на JWasm
  (54 изменённые строки в 8 файлах, функциональных изменений нет).
- `LICENSE`, `NOTICE`.

Патчи — байтовые (`.gitattributes`: `-text`): исходники VC в CP866 с CRLF.

## Как собрать

```sh
sh tools/build-vc.sh                   # .cache/vc-build/bin/{4.05,4.99.09}
sh tools/fetch-vc.sh .cache/vc         # эталон TASM, по желанию
sh tools/build-vc.sh -c .cache/vc      # со сравнением с эталоном
GO2DOS_VC_DIR=$PWD/.cache/vc-build/bin go test ./e2e/   # на своей сборке
python3 tools/vc-diff.py .cache/vc-build/bin/4.05/VC.COM .cache/vc/4.05/VC.COM
```

Нужны `git`, `make`, `gcc`, `sha256sum`, `cmp`; для `vc-diff.py` —
`python3` и `ndisasm` (пакет `nasm`). Работа проверена на Ubuntu (CI, job
`build-vc`); на macOS и Windows JWasm таким скриптом не собирался.

В CI: job `build-vc` в `.github/workflows/ci.yml` строит всё, печатает
сравнение с TASM и дизассемблерные различия, гонит `TestVC405PanelsAndQuit`
на собранном VC 4.05 (и контрольно — на эталонном).

## Что нужно для настоящих правок (следующие шаги M9)

Скрипт и CI готовы; сами правки (WinOldAp, LFN для 4.05, UTF-8 имена) не
сделаны. Что для них требуется — в docs/VC-BUILD.md, раздел «Что нужно для
правок».
