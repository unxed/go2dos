# Фронтенд на vtui (задача T14)

Цель: окно go2dos на `github.com/unxed/vtui` — точный ввод (скан-коды,
модификаторы, отпускание клавиш), буфер обмена хоста, GUI-бэкенды vtui.
ANSI-фронтенд (`cmd/go2dos/term.go`) остаётся запасным.

## Что известно о vtui (проверено по исходникам, `e9d509b`)

- Ввод — `vtinput.InputEvent` = `winkeys.InputEvent`, запись в духе win32
  `INPUT_RECORD`: `VirtualKeyCode`, `VirtualScanCode`, `Char`, `KeyDown`,
  `ControlKeyState` с флагами `LeftAltPressed`, `RightAltPressed`,
  `LeftCtrlPressed`, `RightCtrlPressed`, `ShiftPressed`, `EnhancedKey`.
  `VirtualScanCode` — это set-1 скан-код, то есть то, что ждёт BIOS.
- Своё окно — тип с `vtui.BaseFrame`, реализующий интерфейс `vtui.Frame`
  (`framemanager.go`): `ProcessKey`, `Show(scr *vtui.ScreenBuf)`,
  `ResizeConsole` и т.д.
- Рисование: `ScreenBuf.Write(x, y, []vtui.CharInfo)`, `SetCursorPos`,
  `SetCursorVisible`; цвет — `vtui.SetIndexFore` / `SetIndexBack`
  (`colors.go`).
- Вызов из другой горутины в поток UI — `vtui.FrameManager.PostTask(func())`.
- Буфер обмена — `vtui.SetClipboard(string)`, `vtui.GetClipboard() string`.
- Тестирование UI — `UI_TESTING.md`, `SCREEN_DUMP.md` в vtui.
- **Образец:** встроенный терминал f4 (`github.com/unxed/f4`,
  `internal/terminal/view.go`) — окно vtui, которое рисует сетку ячеек из
  другой горутины (`PostTask`) и передаёт клавиши.

## Шаги

Каждый шаг — отдельный патч с тестом.

- **V1. Преобразование клавиш** — чистая функция без UI:
  `InputEvent → bios.KeyEvent`, файл `frontend/vtuiview/keys.go`.
  - `Scan` = `VirtualScanCode`.
  - `Mods` — из `ControlKeyState`.
  - `Gray` = `EnhancedKey`.
  - `ASCII`: `Char`, переведённый в кодовую страницу через `cp.Byte`;
    0 для функциональных клавиш; `E0h` для серых.
  - Alt/Ctrl/Shift с F-клавишами и серыми — через `keys.Named` по таблице
    `VirtualKeyCode → имя`.
  - Тест — табличный: для каждой клавиши из `keys` (`namedKeys`, буквы,
    цифры, Ctrl/Alt-комбинации) событие vtui даёт то же, что `keys.Named`
    или `keys.Char`.
- **V2. Отрисовка:** `Show` пишет последний `machine.Screen()` в
  `ScreenBuf`. Цвета CGA → индексы 0–15 в порядке ANSI (как
  `machine.SGR`), курсор — из снимка. Тест — через средства `UI_TESTING.md`
  или `SCREEN_DUMP.md`: известный снимок даёт известный дамп экрана.
- **V3. Связка с машиной:** `Config.OnScreen` сохраняет снимок и вызывает
  `FrameManager.PostTask` для перерисовки. Нажатия идут в `m.PushKey`.
  Выход — по `Ctrl-]` `q`, как в ANSI-фронтенде. Флаг `-ui vtui` в
  `cmd/go2dos`.
- **V4. Отпускание клавиш:** события `KeyDown=false` передаются как break-коды,
  синтез отпускания в `bios` для этого фронтенда отключается. Тест: шаги
  `PushKey` дают в порту 60h последовательность make/break.
- **V5. Буфер обмена** — после T05/T06: копирование и вставка через
  `vtui.SetClipboard`/`GetClipboard`.

## Стоп-условия

Если нужного API нет в документации vtui и в образце f4, остановись и
спроси владельца: vtui — его библиотека. Не обходи API через внутренние
поля vtui.
