# go2dos

Эмулятор DOS на Go: библиотека и демо-приложение. Ядро 8086 (с расширениями
80186) и высокоуровневая эмуляция BIOS и DOS поверх каталогов хоста. Цель —
запускать файловые менеджеры для DOS (Norton Commander, Volkov Commander) и
отдавать хосту их текст, буфер обмена и имена файлов.

Состояние на 2026-10-01 (только то, что видно в CI и в дереве `main`):

- **Volkov Commander 4.05 и 4.99.09.** CI (задание `e2e-vc`) запускает их в
  go2dos и проверяет панели и выход (`e2e/vc_test.go`), командную строку,
  запуск команд хоста (`e2e/vc_shell_test.go`) и, для 4.99.09, длинные имена.
  VC также собирается свободными средствами (задание `build-vc`,
  [docs/VC-BUILD.md](docs/VC-BUILD.md)).
- **Ядро 8086:** задание `cpu-singlestep` прогоняет SingleStepTests 8088.
- **Norton Commander 5.51** (английский, скачивается скриптом): без `-lenient`
  останавливается на `INT 10h AH=FFh` (не поддержан); с `-lenient` доходит до
  панелей и меню и выходит кодом 0 (219 таких вызовов). Проверено только
  запуском, показом панели, меню `F9` и выходом `F10`; остальное не проверялось.
- **Dos Navigator 1.51** (готовый бинарник, скачивается скриптом): без
  `-lenient` останавливается на `INT 15h AH=10h`; с `-lenient` рисует панели и
  меню (ещё один неподдерживаемый вызов, `INT 10h AH=1Ch`). Остальное не
  проверялось.
  Подробности, ссылки на прогоны CI и список неподдерживаемого —
  [docs/NC-TESTING.md](docs/NC-TESTING.md).
- В `main` сейчас нет (проверено по дереву): режима `pipe`, моста DOS → WASI,
  кода графических режимов экрана; это открытые задачи ([docs/TASKS.md](docs/TASKS.md)).
  Пометки «сделано» в старых документах относятся к веткам и не гарантируют
  наличия кода в `main`; верить стоит CI и коду.

План — в [docs/DESIGN.md](docs/DESIGN.md), открытые вопросы — в
[docs/DOUBTS.md](docs/DOUBTS.md).

## Запуск

```sh
go build ./cmd/go2dos
./go2dos /path/to/VC.COM            # каталог программы становится C:
./go2dos -drive C=dir -drive D=other C:\VC.COM
```

В терминале: `Ctrl-]` затем `q` — выход, `d` — выход с диагностическим дампом.
По умолчанию (`-display console`) вывод команд идёт прямо в терминал, а
полноэкранные программы рисуются на альтернативном экране; `-display grid` —
всё сеткой.

Автоматизация (без терминала):

```sh
./go2dos -headless -keys '<waitfor:10Quit><F10><Enter>' VC.COM
```

Нестрогий режим: `-lenient` не останавливает программу на неподдерживаемом
вызове BIOS/DOS, а отвечает «не поддерживается» (для INT 21h — CF=1, AX=1) и в
конце печатает в stderr сводку: какие вызовы, сколько раз, откуда. Так можно
пройти дальше первой ошибки и собрать полный список недостающего (для NC и
других программ без исходников). Без флага остаётся fail fast с дампом.

```sh
./go2dos -lenient -headless -timeout 20s -keys '<waitfor:10Quit><F10>' VC.COM
```

Фронтенд на `vtui` (вложенный модуль `cmd/go2dos-vtui`, основной модуль vtui не
тянет): `cd cmd/go2dos-vtui && go build -tags vtui_noebiten,vtui_nogogpu,vtui_nococoa`.
Подробности — [docs/FRONTEND.md](docs/FRONTEND.md).

Флаги: `go2dos -h`. Формат скриптов клавиш — в пакете `keys`.

## Использование как библиотеки

```go
m, _ := machine.New(machine.Config{Drives: map[byte]string{'C': dir}})
m.Load(`C:\VC.COM`, "")
go m.RunScript(ctx, steps, machine.ScriptOptions{})
err := m.Run(ctx)               // *machine.ExitError, *machine.FaultError, ...
fmt.Println(m.Screen().Text())  // текст экрана в UTF-8
```

## Тесты

```sh
go test ./...
sh tools/fetch-vc.sh .cache/vc && GO2DOS_VC_DIR=$PWD/.cache/vc go test ./e2e/
sh tools/fetch-sst.sh .cache/sst && GO2DOS_SST_DIR=$PWD/.cache/sst go test ./cpu/
```

Двоичные файлы чужих программ в репозитории не лежат. Их скачивают скрипты с
проверкой SHA-256: `tools/fetch-vc.sh` (Volkov Commander, BSD-2),
`tools/fetch-nc.sh` (Norton Commander 5.51, `.cache/nc/5.51`; нужны `unrar` и
`7z`), `tools/fetch-dn.sh` (Dos Navigator 1.51, `.cache/dn/1.51`). Переменные
`GO2DOS_NC_DIR` и `GO2DOS_DN_DIR` задают каталоги для e2e-тестов (тестов NC и DN
в `e2e/` пока нет); `tools/trace-nc-dn.sh` снимает трассы запуска (CI,
задание `e2e-nc-dn`, артефакт `nc-dn-traces`). Если источник недоступен,
скрипт завершается кодом 3 и CI пропускает эти шаги, а не падает.

## Документы

- [AGENTS.md](AGENTS.md) — как продолжать работу (для LLM-агентов)
- [DESIGN.md](docs/DESIGN.md) — дизайн и план
- [TASKS.md](docs/TASKS.md) — очередь задач с уровнями сложности
- [ASM-GATES.md](docs/ASM-GATES.md) — гейты для правок ассемблерного кода
- [FRONTEND.md](docs/FRONTEND.md) — фронтенд на vtui: исследование и план
- [VC-BUILD.md](docs/VC-BUILD.md) — сборка VC свободными средствами
- [DOUBTS.md](docs/DOUBTS.md) — сомнения и расследования
- [UTF8NAMES.md](docs/UTF8NAMES.md) — API UTF-8 имён файлов для DOS (черновик)
- [SCREEN.md](docs/SCREEN.md) — окно любого размера, консоль хоста, длинные строки
- [HOSTEXEC.md](docs/HOSTEXEC.md), [HOSTEXEC-API.md](docs/HOSTEXEC-API.md) — команды хоста из DOS: оверлейный режим
- [WASI-BRIDGE.md](docs/WASI-BRIDGE.md) — мост DOS → WASI: решение и план
- [TRANSLATION.md](docs/TRANSLATION.md) — трансляция x86 → wasm (JIT и AOT)
- [DN-PLAN.md](docs/DN-PLAN.md) — Dos Navigator: аудит Turbo Vision, переработка, отдельная TV
- [NC-TESTING.md](docs/NC-TESTING.md) — проверка Norton Commander: источники, SHA-256, наблюдения NC 5.51 и DN 1.51
- [NC-REQUIREMENTS.md](docs/NC-REQUIREMENTS.md) — что нужно NC от BIOS/DOS
- [DN-RESEARCH.md](docs/DN-RESEARCH.md), [HX-RESEARCH.md](docs/HX-RESEARCH.md) (в т. ч. лицензия HX: изменённый HX не распространяем), [WASI-RESEARCH.md](docs/WASI-RESEARCH.md) — исследования
