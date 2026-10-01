# Фронтенд на vtui: исследование и план

Статус: исследование (задача B2.1, карточка T14 в `docs/TASKS.md`, DESIGN §6 и
§9). Результат — план, кода нет. Дата сбора: 2026-10-01.
Пометки: **установлено** — проверено по исходникам (репозитории читались в
виде клонов: `unxed/vtui` на коммите `e95f662` от 2026-09-30, `v0.1.390` —
последний тег и версия в `unxed/f4`; `unxed/vtinput`, `unxed/winkeys`),
**предположение** — вывод без проверки запуском. Go в этой работе не
собирался (правило проекта: только CI).

## 1. Что такое vtui

**Установлено** (`README.md`, `ARCHITECTURE.md`, `go.mod`, `LICENSE`):

- `github.com/unxed/vtui` — каркас TUI для Go в духе Turbo Vision и Far
  Manager: диалоги, меню, `Edit`, `ListBox`, `Table`, `KeyBar`; менеджер
  фреймов (`FrameManager`) со стеком окон и фокусом. Лицензия BSD-3-Clause.
  Написан под файловый менеджер f4 (`unxed/f4` зависит от `vtui v0.1.390`).
- Низкий уровень — `ScreenBuf`: логическая сетка `CharInfo` (`Char uint64`,
  `Attributes uint64`: флаги в младших 16 битах, цвет 24-битный RGB или
  индекс палитры, см. `colors.go`) и теневой буфер; `Flush()` пишет только
  изменённые ячейки через `SurfaceRenderer`. Рендереры: ANSI-терминал
  (по умолчанию, `NewScreenBuf()` ставит `AnsiRenderer`), Win32 GDI, X11,
  Wayland, gogpu, Ebiten, Cocoa. Окно выбирается вызовом
  `vtui.RunInGUIWindow(cols, rows, backend, font, size, setupApp)`.
- Ввод — отдельная библиотека `github.com/unxed/vtinput` (тип `InputEvent` —
  алиас на `winkeys.InputEvent`). Она сама включает протоколы терминала
  (kitty keyboard, win32-input-mode, SGR-мышь, bracketed paste, расширения
  far2l) и на Windows читает события консоли через WinAPI.
- Современный Go: `go 1.26.6`, около 30 зависимостей (Ebiten, gogpu, wayland,
  xgb, goffi/purego, goclip, keytrans, …).

## 2. Как получить из vtui события клавиш

**Установлено** (`vtinput/terminal.go`, `parser.go`, `winkeys/events.go`,
`vtui/framemanager.go`):

1. `vtinput.Enable()` (или `EnableProtocols(mask)`) переводит терминал в raw
   и включает протоколы; возвращает функцию восстановления. Читатель —
   `vtinput.NewReader(os.Stdin, bypass bool)`, метод `ReadEvent()` /
   `ReadEventTimeout(d)` / канал `GetEventChan()`. (README `vtinput`
   показывает вызов с одним аргументом — устарел; сверять по `reader.go`.)
2. Событие `InputEvent`: `Type` (`KeyEventType`, `MouseEventType`,
   `FocusEventType`, `PasteEventType`, `ResizeEventType`, `Far2lEventType`),
   клавиши — `VirtualKeyCode` (Windows VK), `VirtualScanCode`, `Char`,
   `UnshiftedChar`, `KeyDown`, `RepeatCount`, `ControlKeyState` (LeftAlt,
   RightAlt, LeftCtrl, RightCtrl, Shift, NumLock, CapsLock, ScrollLock,
   EnhancedKey), `InputSource`, `IsLegacy`; мышь — `MouseX/Y` (в ячейках),
   `ButtonState`, `MouseEventFlags`, `WheelDirection`.
3. **Нажатие и отпускание различаются** (`KeyDown`) при kitty,
   win32-input-mode и на Windows; на «старых» терминалах приходят только
   нажатия (`IsLegacy`).
4. **Скан-коды приходят не всегда:** `VirtualScanCode` заполняется в
   win32-input-mode (`parser.go`, разбор параметров) и far2l; в kitty и legacy
   разбор оставляет его нулевым (кроме Shift). **Следствие:** фронтенду нужна
   своя таблица VK → скан-код набора 1 (в go2dos её роль у пакета `keys`,
   `keys.Named(name, mods)`); брать значение из события — только если оно
   ненулевое.
5. Вставка из терминала (bracketed paste) приходит как `PasteEventType` с
   `PasteStart=true`, затем обычные события клавиш с символами, затем
   `PasteStart=false` (по образцу `vtui/edit.go`, обработка `PasteEventType`).
6. Как подключить к циклу, два способа:
   - **тонкий:** своя горутина с `vtinput.Reader`, без `FrameManager`; вывод
     — напрямую через `ScreenBuf`. Весь жизненный цикл остаётся в go2dos;
   - **полный:** DOS-экран как `vtui.Frame` (встроить `BaseFrame`; методы
     `ProcessKey/ProcessMouse/Show/ResizeConsole`, интерфейс `Frame` из
     `framemanager.go`, ~25 методов) внутри `FrameManager`; события
     приходят в `ProcessKey`, остальные окна (диалоги, меню «Копировать
     область», настройки) рисует vtui поверх экрана DOS. Внешний код может
     слать синтетические события: `FrameManager.PostEvent(ev)`;
     `FrameManager.PostTask(f)` ставит функцию в UI-поток (потокобезопасно).
     Для GUI-окон: `vtui.RunInGUIWindow(..., setupApp)` — те же
     `InputEvent`, клавиши приходят из X11/Wayland/Win32/… (**установлено**
     по сигнатуре и README; поведение не запускалось).

## 3. Буфер обмена хоста

**Установлено** (`vtui/clipboard.go`, `far2l_extensions.go`):

- `vtui.SetClipboard(text string)`: запоминает текст во внутреннем буфере;
  пробует расширение far2l (`SetFar2lClipboard`), затем помощники ОС
  (`setOSClipboard`: `xclip`/`wl-copy`/`pbcopy`/API Windows), затем OSC 52 в
  stdout (если не `DisableTerminalClipboard()`). Текст обрезается до 2 МиБ
  (OSC 52 — до 1 МиБ).
- `vtui.GetClipboard() string`: far2l → ОС → внутренний буфер (если ОС не
  ответила). Пустой ответ ОС — тоже ответ.
- Вызовы блокирующие (запуск внешней утилиты): из горутины машины их надо
  делать так, чтобы не задерживать исполнение дольше нужного (**предположение**,
  по `getOSClipboard` — запуск процесса).
- GUI-хосты вызывают `UseWindowClipboard()`.
- Это совпадает с планом DESIGN §6: «буфер обмена — тонкая обёртка над
  `vtui.SetClipboard`/`GetClipboard`».

## 4. Что есть в go2dos сейчас

**Установлено** (`cmd/go2dos/main.go`, `term.go`, `vt_*.go`, `machine/machine.go`,
`bios/bios.go`):

- Вывод: `renderer.draw(*bios.Screen)` в `term.go` — собственный разностный
  вывод в ANSI (CGA-цвета через `machine.SGR`, курсор, альтернативный экран),
  вызывается из `Config.OnScreen` (машина зовёт не чаще чем раз в
  `FrameInterval`, 15 мс).
- Ввод: `inputParser` в `term.go` — собственный разбор байтов: CSI/SS3
  (стрелки, F1–F12, Home/End/…), Alt+клавиша как ESC+клавиша, пауза 50 мс
  для одиночного Esc, служебная клавиша `Ctrl-]` (`q` — выход, `d` — дамп).
  Нет протоколов kitty/win32, нет различения `Ctrl+Enter` кроме `\n`, нет
  мыши, нет вставки из терминала (она приходит как поток клавиш).
- В машину клавиша передаётся как `bios.KeyEvent{Scan, ASCII, Mods, Gray}`
  через `Machine.PushKey`; нажатие и отпускание синтезируются вместе
  (`DOUBTS.md`, «BIOS и устройства»). Очередь `keys` — канал на 256 событий,
  `PushKey` блокируется, когда он полон.
- Буфера обмена нет: нет интерфейса `Clipboard`, WinOldAp (`INT 2Fh 17xx`)
  не реализован (PR/задача T05, M8).
- Размер: только текущий размер текстового режима машины; события
  изменения размера окна в машину не передаются (`SCREEN.md`, S2/S5).

## 5. Что менять в `cmd/go2dos`

Предложение (**предположение** — решение принимает владелец потока B/A):

### 5.1. Где живёт код и зависимости

- **Установлено:** go2dos (`go.mod`) зависит только от `localecp`,
  `x/sys`, `x/term`, `x/text`, `go 1.26.0`. vtui тянет `go 1.26.6`, около 30
  модулей, и **каждому потребителю** нужны собственные `replace`
  (`purego → unxed/pureffi`, `goffi → unxed/goffi`, `wayland →
  unxed/wayland …`, `hideconsole → ./internal/hideconsole` — каталог лежит в
  самом vtui). Так сделано в `unxed/f4/go.mod` (строки 163–169): `replace` из
  `go.mod` зависимости не наследуются (известное правило Go modules; здесь
  сборкой не проверялось), а каталог `hideconsole` потребитель держит у себя.
- **Предложение:** не добавлять vtui в основной модуль, чтобы библиотека
  (`machine`, `dos`, …) и её CI остались лёгкими и быстрыми, а `-headless`
  сборка не тянула GUI-стек. Варианты: (а) **вложенный модуль**
  `frontend/vtui/go.mod` (с `replace github.com/unxed/go2dos => ../..`) и
  отдельным исполняемым файлом `cmd`-пакетом внутри; (б) тег сборки `vtui`
  в `cmd/go2dos` — не решает: `go.mod` один и зависимости остаются в
  графе. Выбрать (а). В сборках vtui — теги `vtui_noebiten`,
  `vtui_nogogpu` (по `PLATFORMS.md` убирают Ebiten/gogpu, около 9 МБ), как в
  «lite»-сборке f4.
- Текущий `cmd/go2dos` остаётся как лёгкий терминальный фронтенд и как
  резерв; общий код (флаги, запуск, коды выхода) выносится в пакет, чтобы
  не дублировать (`cmd/go2dos/main.go`: `run()` → библиотечная функция с
  интерфейсом фронтенда).

### 5.2. Интерфейс хоста (DESIGN §6) — общий для обоих фронтендов

```go
// предложение, пакет machine или host
type Frontend interface {
    // вызывается из горутины машины при изменении экрана
    Draw(*bios.Screen)
    // события ввода: фронтенд сам вызывает Machine.PushKey / команды
}
type Clipboard interface { GetText() (string, error); SetText(string) error }
```

`Clipboard` нужен и серверу WinOldAp в пакете `dos` (T05), и фронтенду (копирование
с экрана); реализации: «в памяти» (тесты), vtui-обёртка, без буфера.

### 5.3. Вывод: `bios.Screen` → `ScreenBuf`

- Размер: `scr.AllocBuf(s.Cols, s.Rows)` при первом снимке и при смене
  `Cols/Rows`; ячейка: `CharInfo{Char: uint64(c.Rune), Attributes: attr}` с
  `attr` из CGA-атрибута: цвета через палитру из 16 RGB (таблица CGA/VGA —
  в фронтенде) и `SetRGBBoth(0, fg, bg)`; **вопрос на проверку:** бит 7
  (мерцание или яркий фон) — реализован ли `INT 10h/1003h` в машине — не проверялось;
  vtui не имеет мерцания в `CharInfo` (поле `ForegroundIntensity` — яркий
  текст, `BackgroundIntensity` — «флаг стиля»); по умолчанию — яркий фон,
  как делает текущий `machine.SGR` (**установлено**: его комментарий — «blink
  bit shown as bright background»).
- Запись: `scr.Write(0, y, rowCells)` по строкам снимка, затем курсор:
  `SetCursorPos`, `SetCursorVisible`, `SetCursorShape` (форма — из CRTC
  начала/конца курсора), затем `Flush()`. Разностный вывод, квантование
  цветов под профиль терминала и ширина символов (`runewidth`) — уже в vtui;
  собственный `renderer` из `term.go` не нужен.
- Символы 00–1F и 7F (глифы ☺ ☻ …) уже переведены в `Cell.Rune` слоем `cp`.
- Окно хоста: из `FrameManager` (событие `ResizeEventType`, `ResizeConsole`)
  размер идёт в `Machine` — это зависит от S2/S5 (`SCREEN.md`).
- **Режим `console`** (`SCREEN.md`, S4) конфликтует с полным каркасом:
  `FrameManager.Init` берёт терминал под себя (`initTerminalOS`, сброс
  палитры OSC 104). Для поток-канала нужны: тонкий вариант (§2, п. 6)
  или `ScreenBuf.WritePassthrough` (запись в stdout мимо теневого буфера,
  под мьютексом кадра). Выбор режима — решение S4; для S4 vtui — только в
  режиме `grid`.

### 5.4. Ввод: `vtinput.InputEvent` → `bios.KeyEvent`

| `InputEvent` | → `bios.KeyEvent` |
|---|---|
| `ControlKeyState` | `Mods`: Shift → `ModLShift`, Ctrl (L/R) → `ModCtrl`, Alt (только L) → `ModAlt`; правый Alt без Ctrl (AltGr) — как ввод символа |
| `KeyDown && Char >= 32` (без Ctrl/Alt) | `keys.Char(rune, codepage)` (символ → байт DOS-страницы; невозможный символ — как сейчас, скан-код 0) |
| Ctrl+буква, Alt+буква/цифра | `keys.Ctrl`, `keys.Alt` |
| функциональные, стрелки, Home…, Ins/Del, PgUp/PgDn, Tab, Enter, Backspace, Esc, цифровой блок | таблица `VirtualKeyCode` → имя → `keys.Named(name, mods)`; `EnhancedKey` → `Gray` |
| `KeyDown=false` | пока игнорировать (отпускание синтезирует машина); позже — раздельные make/break (ниже) |
| `RepeatCount > 1` / автоповтор | повторять `PushKey` |
| `Ctrl-]` | служебная клавиша сохраняется (двойное — передать в программу); `q` — выход, `d` — дамп. Под vtui — отдельная команда меню/сочетание, решить при реализации |

**Раздельные make/break** (нужны программам, читающим порт 60h напрямую, играм):
vtinput отдаёт `KeyDown=false` лишь при kitty/win32/Windows. Для `Machine`
нужны методы `KeyDown(k)`/`KeyUp(k)` рядом с `PushKey` (изменение в
`bios`/`machine` — поток A; фронтенд пока использует `PushKey`).

### 5.5. Буфер обмена (T05, T06)

1. **Копирование с экрана.** Выделение мышью: `MouseEventType` с
   `MouseX/Y` в ячейках; между нажатием и отпусканием — прямоугольник (или
   потоковое выделение; режим решает владелец); `Screen.Region(rect)` →
   `Clipboard.SetText`. Подсветка выделения — `ScreenBuf.ApplyColor` или
   инверсия `InvertColors` при отрисовке; сочетания — по образцу VC/NC
   (Ctrl-Ins, Shift-Ins), чтобы не мешать программе, когда мышь ей нужна
   (`INT 33h`: сейчас мыши нет, `DOUBTS.md`).
2. **Вставка.** Источники: `PasteEventType` (bracketed paste), `Shift-Ins` /
   `Ctrl-V` → `Clipboard.GetText()`. Текст → нажатия: символ → байт
   кодовой страницы, `\n` → Enter (DESIGN §9). Отправлять из отдельной
   горутины: `PushKey` блокируется при переполнении очереди (256), буфер BIOS
   вмещает 15 нажатий, остальное ждёт в канале (**установлено** по
   `machine.go`).
3. **Сервер WinOldAp** (пакет `dos`, T05) использует тот же `Clipboard`.
   Из-за блокирующего чтения (§3) читать хост-буфер при `1701h` (open) один
   раз, а писать при `1708h` (close) — не на каждый вызов.
4. Кодировка `CF_TEXT`/`CF_OEMTEXT` — отдельный вопрос (`DOUBTS.md`).

## 6. План (мелкие шаги, у каждого проверка)

| Шаг | Что | Проверка |
|---|---|---|
| F0 | Интерфейсы `Frontend`/`Clipboard`, вынос общего кода `cmd/go2dos`; существующий фронтенд — первая реализация | тесты `cmd/go2dos` (как для `-lenient`) и e2e VC без изменений |
| F1 | Вложенный модуль `frontend/vtui`: сборка с `replace` и тегами `vtui_noebiten vtui_nogogpu`; пустое окно на `ScreenBuf` | CI-job `go build`/`go vet` отдельно от основного |
| F2 | Вывод: `bios.Screen` → `ScreenBuf` (цвета CGA, курсор, размер) | тест на `NewSilentScreenBuf`: `GetCell` совпадает со снимком; снимок VC в pty |
| F3 | Ввод: `InputEvent` → `bios.KeyEvent` (таблица VK → имя), модификаторы | табличный тест на синтетических `InputEvent` (без терминала); скрипт `-keys` как эталон |
| F4 | Буфер обмена: `Clipboard` на vtui, копирование выделением, вставка порциями | тест с `vtui.SkipOSClipboard(true)` и `DisableTerminalClipboard()` (так делает сам vtui); вставка → нажатия |
| F5 | Раздельные make/break (с потоком A) | `.COM` читает порт 60h |
| F6 | Размер окна и режим `console` (после `SCREEN.md` S2/S4) | pty-тест |
| F7 | GUI-окно: `--gui=x11|wayland|win32|…` через `RunInGUIWindow`; для браузера/M12 — `ProtocolSession` (`cmd/vtui-wasm`, JSON Lines поверх stdin/stdout, сборка `wasip1`) | запуск в CI под `xvfb` (**предположение**), в wasm — M12 |

## 7. Открытые вопросы

1. **Вложенный модуль или отдельный репозиторий** для vtui-фронтенда
   (установлено, что в основной модуль добавлять нельзя без утяжеления CI;
   выбор остаётся за потоком B/владельцем).
2. **Мерцание/яркий фон** в CGA-атрибуте (§5.3).
3. **Выделение мышью и `INT 33h`:** когда программа включила мышь, выделять
   по Shift+мышь (как в терминалах) — решить вместе с реализацией мыши.
4. **Режим `console`** и vtui (§5.3) — конфликт за терминал.
5. Вынесен ли ввод для **Windows** на WinAPI-режим vtinput (`InputMode`):
   там `isWindowsNative` подавляет ANSI-протоколы; проверить в CI на
   `windows-latest` (**не установлено**).
6. На Linux GUI-хосты требуют `DISPLAY`/`WAYLAND_DISPLAY`
   (`gui_api.go`) — CI без дисплея: только сборка и юнит-тесты.
