# go2dos

Эмулятор DOS на Go: библиотека и демо-приложение. Ядро 8086 (с расширениями
80186) и высокоуровневая эмуляция BIOS и DOS поверх каталогов хоста. Цель —
запускать файловые менеджеры для DOS (Norton Commander, Volkov Commander) и
отдавать хосту их текст, буфер обмена и имена файлов.

Состояние: Volkov Commander 4.05 и 4.99.09 работают; подробности и план — в
[docs/DESIGN.md](docs/DESIGN.md), открытые вопросы — в
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
sh tools/fetch-vc.sh .cache/vc && GO2DOS_VC_DIR=.cache/vc go test ./e2e/
sh tools/fetch-sst.sh .cache/sst && GO2DOS_SST_DIR=.cache/sst go test ./cpu/
```

Norton Commander в тестах не используется; как проверить его самому —
[docs/NC-TESTING.md](docs/NC-TESTING.md).

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
- [WASI-BRIDGE.md](docs/WASI-BRIDGE.md) — мост DOS → WASI: решение и план
- [TRANSLATION.md](docs/TRANSLATION.md) — трансляция x86 → wasm (JIT и AOT)
- [DN-PLAN.md](docs/DN-PLAN.md) — Dos Navigator: аудит Turbo Vision, переработка, отдельная TV
- [NC-TESTING.md](docs/NC-TESTING.md) — проверка Norton Commander
- [NC-REQUIREMENTS.md](docs/NC-REQUIREMENTS.md) — что нужно NC от BIOS/DOS
- [DN-RESEARCH.md](docs/DN-RESEARCH.md), [HX-RESEARCH.md](docs/HX-RESEARCH.md), [WASI-RESEARCH.md](docs/WASI-RESEARCH.md) — исследования
