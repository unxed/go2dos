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
- **Norton Commander 5.51** (английский, скачивается скриптом): запускается без
  `-lenient`, рисует панели и меню, `F10` выходит кодом 0; e2e `TestNC551TwoPanels`
  (задание `e2e-nc-dn`). Проверено только запуском, панелями, меню `F9` и
  выходом; остальное не проверялось.
- **Одна панель при запуске** — умолчание самих программ без сохранённой
  настройки, не ошибка эмулятора: VC без `VC.INI` включает только правую панель
  (`VC.ASM`, `Init03`); `Ctrl-F1` включает левую, `Shift-F9` сохраняет `VC.INI`
  (e2e `TestVC405TwoPanels`, `TestVC49909TwoPanels`). У NC 5.51 без `NC.INI`
  наблюдается то же, причина не установлена
  ([docs/NC-TESTING.md](docs/NC-TESTING.md)).
- **Dos Navigator 1.51** (готовый бинарник, скачивается скриптом): запускается
  без `-lenient`, рисует две панели, строку клавиш и меню; в CI запуск идёт до
  лимита времени (скрипт клавиш рассчитан на NC). Остальное не проверялось.
  Подробности, ссылки на прогоны CI и список неподдерживаемого —
  [docs/NC-TESTING.md](docs/NC-TESTING.md).
- **Имена файлов с символами вне кодовой страницы** (кириллица при CP437, умляуты при
  CP866): VC 4.05 и 4.99.09 видят их под уникальными обратимыми псевдонимами и ничего не
  теряют при переименовании, копировании, перемещении, удалении и входе в каталог;
  сквозные тесты в CI (`e2e/vc_names_test.go`), подробности и ограничения —
  [docs/NAMES.md](docs/NAMES.md).
- **API `DOS-HOST/TEXTWIN`** (размер текстового окна, событие изменения размера в
  буфере клавиатуры, роль экрана) на AMIS, [docs/TEXTWIN.md](docs/TEXTWIN.md); тесты в
  `machine/textwin_test.go`. Фронтенды ещё не сообщают машине об изменении окна.
- **Режим конвейера** (`pipe`): если stdin или stdout не терминал (или с флагом `-pipe`),
  хэндлы 0/1/2 программы — потоки `go2dos`, текст переводится UTF-8 ↔ кодовая страница,
  экран не рисуется: `go2dos hello.com | cat`, `go2dos filter.com < in.txt > out.txt`;
  тесты в CI (`machine/pipe_test.go`, `cmd/go2dos/main_test.go`), правила — в
  [docs/SCREEN.md](docs/SCREEN.md).
- В `main` сейчас нет (проверено по дереву): моста DOS → WASI,
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

В терминале: `Ctrl-]` затем `q` — выход, `d` — выход с диагностическим дампом, `c` — текст
экрана в буфер, `v` — вставить буфер, `s` — выбрать область экрана: стрелки, `Home`/`End`,
`PgUp`/`PgDn` двигают курсор, `Пробел` или `Enter` отмечают угол, второе нажатие копирует
прямоугольник в буфер, `Esc` — отмена (только при `-display grid`).
По умолчанию (`-display console`) вывод команд идёт прямо в терминал, а
полноэкранные программы рисуются на альтернативном экране; `-display grid` —
всё сеткой.

Буфер обмена внутри VC 4.05: `tools/build-vc405-pts.sh` кладёт рядом с эталонным
`bin/4.05/VC.COM` вариант `bin/4.05-clip/VC.COM` (на 272 байта больше) и модуль `bin/4.05-clip/VCEXT.BIN`, который должен лежать рядом с `VC.COM` (без него клавиши ничего не делают). В полях ввода и
командной строке `Ctrl-Ins` копирует строку в буфер, `Shift-Ins` вставляет первую строку
буфера, `Shift-Del` вырезает (`Del` без Shift удаляет символ, как раньше). Нужен
сервер буфера обмена хоста (в терминальном режиме он есть всегда); без сервера клавиши
ничего не делают.

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
`GO2DOS_NC_DIR` и `GO2DOS_DN_DIR` задают каталоги для e2e-тестов (в `e2e/` есть
тест NC, `TestNC551TwoPanels`; тестов DN пока нет); `tools/trace-nc-dn.sh` снимает трассы запуска (CI,
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
- [NAMES.md](docs/NAMES.md) — имена без потерь для программ, не знающих UTF-8
- [SCREEN.md](docs/SCREEN.md) — окно любого размера, консоль хоста, длинные строки
- [HOSTEXEC.md](docs/HOSTEXEC.md), [HOSTEXEC-API.md](docs/HOSTEXEC-API.md) — команды хоста из DOS: оверлейный режим
- [WASI-BRIDGE.md](docs/WASI-BRIDGE.md) — мост DOS → WASI: решение и план
- [TRANSLATION.md](docs/TRANSLATION.md) — трансляция x86 → wasm (JIT и AOT)
- [DN-PLAN.md](docs/DN-PLAN.md) — Dos Navigator: аудит Turbo Vision, переработка, отдельная TV
- [NC-TESTING.md](docs/NC-TESTING.md) — проверка Norton Commander: источники, SHA-256, наблюдения NC 5.51 и DN 1.51
- [NC-REQUIREMENTS.md](docs/NC-REQUIREMENTS.md) — что нужно NC от BIOS/DOS
- [DN-RESEARCH.md](docs/DN-RESEARCH.md), [HX-RESEARCH.md](docs/HX-RESEARCH.md) (в т. ч. лицензия HX: изменённый HX не распространяем), [WASI-RESEARCH.md](docs/WASI-RESEARCH.md) — исследования
