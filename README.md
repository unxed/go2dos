# go2dos

Эмулятор DOS на Go: библиотека и демо-приложение. Ядро 8086 (с расширениями
80186) и высокоуровневая эмуляция BIOS и DOS поверх каталогов хоста. Цель —
запускать файловые менеджеры для DOS (Norton Commander, Volkov Commander) и
отдавать хосту их текст, буфер обмена и имена файлов.

Состояние: Volkov Commander 4.05 работает; подробности и план — в
[docs/DESIGN.md](docs/DESIGN.md), открытые вопросы — в
[docs/DOUBTS.md](docs/DOUBTS.md).

## Запуск

```sh
go build ./cmd/go2dos
./go2dos /path/to/VC.COM            # каталог программы становится C:
./go2dos -drive C=dir -drive D=other C:\VC.COM
```

В терминале: `Ctrl-]` затем `q` — выход, `d` — выход с диагностическим дампом.

Автоматизация (без терминала):

```sh
./go2dos -headless -keys '<waitfor:10Quit><F10><Enter>' VC.COM
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
- [DOUBTS.md](docs/DOUBTS.md) — сомнения и расследования
- [UTF8NAMES.md](docs/UTF8NAMES.md) — API UTF-8 имён файлов для DOS (черновик)
- [WASI-BRIDGE.md](docs/WASI-BRIDGE.md) — мост DOS → WASI: решение и план
- [TRANSLATION.md](docs/TRANSLATION.md) — трансляция x86 → wasm (JIT и AOT)
- [DN-PLAN.md](docs/DN-PLAN.md) — Dos Navigator: аудит Turbo Vision, переработка, отдельная TV
- [NC-TESTING.md](docs/NC-TESTING.md) — проверка Norton Commander
