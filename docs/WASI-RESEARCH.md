# Мост DOS → WASI: результаты исследования R1–R5

Статус: исследование (W0 из [WASI-BRIDGE.md](WASI-BRIDGE.md)); кода нет. Пометки:
**установлено** (источник и цитата/URL) и **предположение** (вывод без проверки).
Всё проверено чтением источников; ничего не собиралось локально.

## Главное

- Путь к witx в WASI-BRIDGE (`legacy/preview1`) устарел: репозиторий
  `WebAssembly/WASI` перестроен, preview1 живёт в ветке `wasi-0.1`, каталог
  `preview1/` (R1).
- Переиспользовать WASI из wazero «как есть» с памятью DOS нельзя (внутренний
  тип), но можно **обходным путём** — через крошечный wasm-модуль, чья линейная
  память подменена срезом памяти DOS (R2, предположение, нужна проверка).
- Для M12 (go2dos под `GOOS=wasip1`) критично: нет сокетов, нет
  `os/exec`, и `golang.org/x/term` на wasip1 не работает (R3) — терминальный
  фронтенд придётся заменить.
- AMIS: обязательны функции 00h, 02h и 04h; AL=01h необязательна (R4).

## R1. Функции и раскладки preview1

**Источник (установлено).** `https://github.com/WebAssembly/WASI/tree/wasi-0.1`,
коммит `fae981bae14809d91f9bc2d63852d461f331d161`: `preview1/witx/
wasi_snapshot_preview1.witx`, `preview1/witx/typenames.witx`, `preview1/docs.md`
(в нём размеры, выравнивания и смещения полей), `tools/witx/src/abi.rs` (ABI).
README основной ветки: «WASI started with launching what is now called
Preview 1, an API using the witx IDL» со ссылкой на ветку `wasi-0.1`.

**ABI (установлено).** `abi.rs`: единственный ABI `Preview1`: «it can only
return 0 results or one `Result<T, enum>` lookalike» — функция возвращает
`errno` (i32), а значение успеха пишется по **дополнительному указателю в конце
списка параметров**. `string` — пара (указатель, длина), массивы (`iovec_array`)
— тоже (указатель, число элементов). Это согласуется с блоком параметров
в WASI-BRIDGE. `proc_exit` ничего не возвращает.

**Функции (установлено, 46 штук):**
`args_get args_sizes_get environ_get environ_sizes_get clock_res_get
clock_time_get fd_advise fd_allocate fd_close fd_datasync fd_fdstat_get
fd_fdstat_set_flags fd_fdstat_set_rights fd_filestat_get fd_filestat_set_size
fd_filestat_set_times fd_pread fd_prestat_get fd_prestat_dir_name fd_pwrite
fd_read fd_readdir fd_renumber fd_seek fd_sync fd_tell fd_write
path_create_directory path_filestat_get path_filestat_set_times path_link
path_open path_readlink path_remove_directory path_rename path_symlink
path_unlink_file poll_oneoff proc_exit proc_raise sched_yield random_get
sock_accept sock_recv sock_send sock_shutdown`.
Сокетов в preview1 действительно только четыре, и все работают с уже открытыми
сокетами (проверяет утверждение в W7).

**Сигнатуры, важные для DOS (установлено).** 64-битные параметры (`filesize`,
`timestamp`, `rights`, `dircookie`, `filedelta`): `fd_seek(fd, offset:s64,
whence:u8)`, `fd_pread/pwrite(..., offset:u64)`, `clock_time_get(id:u32,
precision:u64)`, `path_open(fd, dirflags, path, oflags:u16, rights_base:u64,
rights_inheriting:u64, fdflags:u16)` — девять wasm-параметров (`path` занимает
два) плюс указатель результата. По блоку «всё по 4 байта, i64 — 8» из
WASI-BRIDGE это даёт для `path_open` 4+4+8+2→4+8+8+4+4(результат) = 44 байта
(арифметика, **предположение** о принятой раскладке малых типов: u8/u16 как
4 байта).

**Раскладки структур (установлено, `docs.md`: Size/Alignment/Offset):**

| Тип | Размер/выравнивание | Поля (смещение) |
|---|---|---|
| `iovec`, `ciovec` | 8 / 4 | buf 0, buf_len 4 |
| `filestat` | 64 / 8 | dev 0, ino 8, filetype 16, nlink 24, size 32, atim 40, mtim 48, ctim 56 |
| `fdstat` | 24 / 8 | fs_filetype 0, fs_flags 2, fs_rights_base 8, fs_rights_inheriting 16 |
| `dirent` | 24 / 8 | d_next 0, d_ino 8, d_namlen 16, d_type 20 |
| `prestat` (вариант) | 8 / 4 | тег (1 байт, `tag_size: 1`) 0; `prestat_dir.pr_name_len` 4 (по размеру/выравниванию) |
| `event` | 32 / 8 | userdata 0, error 8, type 10, fd_readwrite 16 (nbytes 0, flags 8 внутри) |
| `subscription` | 48 / 8 | userdata 0, u 8 (вариант: тег 1 байт, содержимое с 8) |
| `subscription_clock` | 32 / 8 | id 0, timeout 8, precision 16, flags 24 |
| `subscription_fd_readwrite` | 4 / 4 | file_descriptor |

Следствие (**предположение**, логика): структуры содержат выровненные на 8 поля
u64, но 8086 не требует выравнивания, поэтому раскладка «байт в байт как wasm32»
(WASI-BRIDGE) реализуема; опасность — только если DOS-программа разместит
структуру по нечётному адресу и хост будет читать её через `unsafe`-приведение
(читать нужно побайтно, `binary.LittleEndian`).

`errno` — 77 значений (`success`…`notcapable`), 16-битный. Коды надо брать из
`typenames.witx`, а не из DOS (в DOS своя нумерация).

**Подмножество v0 (решение, предложение).** W3–W5 плана: `args_*`, `environ_*`,
`clock_time_get`, `random_get`, `proc_exit`, `sched_yield`, `poll_oneoff` (часы),
`fd_write/read/seek/tell/close`, `fd_fdstat_get`, `fd_prestat_get`,
`fd_prestat_dir_name`, `fd_filestat_get`, `fd_readdir`, `path_open`,
`path_filestat_get`, `path_create_directory`, `path_remove_directory`,
`path_unlink_file`, `path_rename`. Остальное в v0 отвечает `ENOSYS`
(`fd_advise/allocate/datasync/sync/renumber`, `fd_fdstat_set_*`,
`fd_pread/pwrite`, `fd_filestat_set_*`, `path_link/symlink/readlink`,
`path_filestat_set_times`, `proc_raise`, `sock_*`). Причина по ссылкам и
символическим ссылкам: **предположение** — в FAT/DOS их нет, хост-файлы могут
быть, но возвращать их программам DOS смысла мало. `fd_fdstat_get` нужен
рантаймам C (определение типа fd), `rights` в v0 можно отдавать «всё»
(**предположение**, проверить по wasi-libc).

## R2. Можно ли взять WASI из wazero

**Источник (установлено).** `github.com/tetratelabs/wazero`, коммит
`e234f6fe6ecd4589dd0c643b641bd38f6d826ee5` (2026-09-28), лицензия Apache-2.0,
`go 1.25.0` (у нас 1.26), зависимость — только `golang.org/x/sys`.

- **Установлено.** Обработчики в `imports/wasi_snapshot_preview1/fs.go` имеют вид
  `func fdWriteFn(_ context.Context, mod api.Module, params []uint64)` и первым
  делом делают `fsc := mod.(*wasm.ModuleInstance).Sys.FS()` (строки 36, 79, …;
  `wasm` — `internal/wasm`). Значит, WASI-реализация привязана к настоящему
  экземпляру wasm-модуля и его `Sys`-контексту; извне пакета эти функции
  неэкспортируемы, а `internal/` недоступен.
- **Установлено.** `api.Module` и `api.Memory` содержат `internalapi.WazeroOnly`
  (`api/wasm.go`) — реализовать их своим типом нельзя; `experimental/wazerotest`
  даёт фальшивый `Module`, но он не `*wasm.ModuleInstance`, и приведение в
  обработчиках не пройдёт (**предположение** по коду; не запускалось).
- **Установлено.** Публичны (`experimental`): `experimental/sys` (тип `Errno`,
  файловая абстракция) и `experimental/sysfs` (`DirFS` и др.) — файловый слой
  без wasm, но с пометкой «experimental»; плюс
  `experimental.WithMemoryAllocator`/`LinearMemory{Reallocate(size) []byte}` —
  линейную память модуля можно отдать из своего `[]byte`.
- **Вывод.** Прямое переиспользование невозможно без внутренних API.
  Варианты: (а) ожидаемый — писать `wasidos` поверх Go `os` (как в плане);
  (б) **предположение, требует прототипа:** запускать в wazero маленький
  адаптерный wasm-модуль, чья линейная память — срез памяти DOS
  (`MemoryAllocator`), а функции WASI вызывает наш код с линейными адресами
  вместо far-указателей; весь WASI (включая песочницу путей) тогда берётся из
  wazero. Минусы (б): тянет интерпретатор wazero в go2dos, память DOS должна
  быть непрерывным `[]byte` фиксированного адреса, 64-КиБ страницы wasm, API
  `experimental` нестабилен. Рекомендую (а); (б) — только если захочется
  побайтной совместимости с реальным WASI.

## R3. Ограничения Go `GOOS=wasip1` (для M12)

**Источники (установлено):** блог Go «WASI support in Go»
(`https://go.dev/blog/wasi`, 13.09.2023), примечания к выпускам
`https://go.dev/doc/go1.24`, исходники стандартной библиотеки Go 1.26.6 (`syscall/
fs_wasip1.go`, `net_wasip1.go`, `syscall_wasip1.go`, `net/file_wasip1.go`) и
`golang.org/x/term` v0.46.0.

- **Preopen.** `syscall/fs_wasip1.go`: при старте перебираются fd с 3 по
  `fd_prestat_get` до `EBADF`; имена берутся из `fd_prestat_dir_name`. Первый
  preopen считается корнем, остальные — точками монтирования в подпутях;
  путь разрешается самым длинным совпавшим префиксом; текущий каталог — из
  переменной `PWD`, иначе имя первого preopen. Доступа вне preopen нет.
  Для go2dos: каждый диск `C=dir` → отдельный `--dir` рантайма; видимость
  остальной ФС хоста определяет только рантайм (**вывод**).
- **Потоки.** «Wasm is a single threaded architecture… any host function calls
  … will cause all goroutines to block until the host function call has
  returned» (блог) — блокирующий ввод консоли остановит все горутины;
  стандартные in/out/err неблокирующие. Ожидание ввода в go2dos должно идти
  через `poll_oneoff` (в runtime это `os_wasip1.go`).
- **Сокеты.** Блог: «wasip1 only defines functions that operate on already
  opened sockets». В `syscall/net_wasip1.go`: `Socket/Bind/Listen/Connect/
  Accept/…` возвращают `ENOSYS`, реализованы только `sock_accept` и
  `sock_shutdown`; `net.FileListener/FileConn` работают с уже открытыми
  (preopen) сокетами (`net/file_wasip1.go`), `FilePacketConn` — `ENOPROTOOPT`.
  `net.Dial/Listen` недоступны (сторонние обходные пути — в блоге, для
  Wasmer/WasmEdge). Для go2dos: сетевой слой DOS-программ (если когда-то
  понадобится) — только через преднастроенные хостом сокеты.
- **Процессы.** `syscall.StartProcess` на wasip1 возвращает `ENOSYS`
  (`syscall_wasip1.go`, ~410) — `os/exec` не работает; EXEC DOS-программ внутри
  go2dos это не затрагивает (это наша эмуляция), но запуск хост-команд
  невозможен.
- **Терминал.** `golang.org/x/term` v0.46.0: `term_unix.go` собирается для
  `aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd ||
  solaris || zos`, а `term_unsupported.go` — для всего остального, включая
  wasip1 (`!aix && … && !windows && !solaris && !plan9`): raw-режим и размер
  терминала недоступны. `cmd/go2dos/main.go` использует `x/term`, значит,
  терминальный фронтенд под wasip1 — отдельная задача (**предположение:**
  понадобится режим «только headless/клавиши из скрипта» либо фронтенд
  браузера, M12).
- **Версии.** wasip1 с Go 1.21 (`GOOS=wasip1 GOARCH=wasm`); Go 1.24 добавил
  `go:wasmexport` и режим reactor/library (`-buildmode=c-shared`) и расширил
  допустимые типы `go:wasmimport` (примечания 1.24). Для M12 reactor-режим
  полезен: хост (браузер, JS) вызывает go2dos как библиотеку.
- **Не проверено:** поведение `os.Stdin` неблокирующим чтением на конкретных
  рантаймах (блог: «Go wasip1 binaries don’t execute perfectly on all hosts
  yet», issues #59907, #60097); скорость интерпретатора на wasm-рантаймах
  (важна для цикла CPU).

## R4. AMIS 3.6

**Источник (установлено).** Ralf Brown's Interrupt List, раздел INT 2D (AMIS
v3.6), зеркало `https://fd.lod.bz/rbil/interrup/tsr/2d.html` и подстраницы
`2d_00.html`, `2d_01.html`, `2d_02.html`, `2d_04.html`. Сам текст спецификации
AMIS (отдельный документ) не найден; сведения — из RBIL.

- **Проверка наличия, AL=00h.** Вход: `AL=00h`, `AH` = мультиплексный номер.
  Выход: `AL=00h` — свободен, `AL=FFh` — занят; при занятом: `CX` — двоичная
  версия (`CH` старший, `CL` младший; «binary», не BCD), `DX:DI` — подпись.
  Подпись: смещение 0 — 8 байт производитель, 8 — 8 байт продукт (оба с
  заполнением пробелами), 10h — до 64 байт ASCIZ-описание (может быть одним
  нулём). Совпадает с UTF8NAMES.md.
- **Поиск номера.** «programs should not use fixed multiplex numbers; rather, a
  program should scan all multiplex numbers from 00h to FFh, remembering the
  first unused multiplex»; занятость проверяется сравнением **первых 16 байт**
  подписи. Это уточнение к UTF8NAMES (там описан только перебор).
- **AL=01h, частный вход.** Вход `AL=01h`, `AH` — номер; выход `AL=00h` — все
  вызовы только через `INT 2Dh`, `AL=FFh` — вход поддержан, `DX:BX` — адрес
  входа в обход цепочки прерывания. Функция «not valid unless a program is
  installed»; сначала AL=00h. **Установлено:** функция необязательна; мост
  WASI может ответить `AL=00h` и принимать вызовы по `INT 2Dh`, тогда «far
  call» из WASI-BRIDGE — оптимизация, а не требование.
- **Обязательные функции.** «full v3.6 compliance requires … at least functions
  00h, 02h (no resident uninstall code required), and 04h (return value 04h)»;
  05h — если есть горячие клавиши, 06h — если драйвер устройства. Для нашего
  провайдера: 00h, 01h (по желанию), **02h** (может отвечать «не
  реализовано/не удаляемо»), **04h** (`AL=04h` и `DX:BX` на список перехваченных
  прерываний).
- **AL=02h.** Вход `DX:BX` — адрес возврата после выгрузки (TSR может
  игнорировать); выход `AL` — статус (коды 00h–FFh: не реализовано,
  неудача, выгрузить позже, безопасно удалить и т.д.; точные значения в
  RBIL `2d_02`). **Предположение для эмулятора:** провайдер встроен в go2dos и
  неудаляем — ответить кодом «не удалось/неудаляемо» (нужно сверить значение
  по таблице RBIL перед реализацией).
- **AL=04h.** Вход `BL` — номер прерывания (кроме 2Dh); выход `AL=04h` —
  возвращён список, `DX:BX` на таблицу записей по 3 байта: номер прерывания
  (1) и смещение обработчика (2); «last entry in array is 2Dh». Значения
  `00h` — «не реализовано (делает TSR несовместимым)», `FFh` — не перехвачено,
  `01h–03h` — устаревшие (v3.3), использовать нельзя. Наш провайдер: список
  из одной записи `2Dh`.
- **Не проверено:** формат `INT 2Dh` при вектор = 0000:0000 (UTF8NAMES требует
  проверки вектора перед перебором — разумно, но в RBIL этого указания не
  найдено); как должны вести себя DOS-программы, сканирующие все 256 номеров
  (стоимость — 256 вызовов `INT 2Dh`; в go2dos это дёшево). Общий диспетчер
  `INT 2Dh` для UTF-8 имён и моста (WASI-BRIDGE) — да: один номер на оба
  провайдера нельзя (подпись у каждого своя), нужны два разных
  мультиплексных номера под одним диспетчером (**вывод** из формата подписи).

## R5. Компиляторы для клиентов

Как запускать результат: сам go2dos (`go2dos -headless … PROG.EXE`), в CI —
как в `e2e/`; программы кладутся в `testdata/` бинарниками (политика
`testdata/progs`).

- **gcc-ia16 (TK Chia).** Установлено: исходники `github.com/tkchia/gcc-ia16`
  (+ `binutils-ia16`, сборка через `github.com/tkchia/build-ia16`); Ubuntu-
  пакеты из PPA `ppa:tkchia/build-ia16` (`gcc-ia16-elf`; PPA публикует Resolute,
  Oracular, Noble, Jammy, Focal — страница
  `https://launchpad.net/~tkchia/+archive/ubuntu/build-ia16/`);
  цель по умолчанию — MS-DOS, `.exe` (MZ); есть far-указатели (`__far`),
  DJGPP-бинарники — `gitlab.com/tkchia/build-ia16/-/releases`. Для CI:
  `add-apt-repository ppa:tkchia/build-ia16 && apt install gcc-ia16-elf`
  (**предположение:** на `ubuntu-latest` GitHub — Noble, подойдёт; проверить
  прогоном). Практически основной кандидат для заголовка C.
- **Open Watcom 2.0.** Установлено: Linux-хост поддержан (`wcl`, `wcl386`
  работают на Linux, 2.0 — 32/64 бит), для CI есть GitHub Action
  `open-watcom/setup-watcom` (версии 1.8, 1.9, 2.0, 2.0-64), цель DOS —
  `-bt=dos` (`https://github.com/open-watcom/setup-watcom`,
  `https://open-watcom.github.io/open-watcom-v2-wikidocs/c_readme.html`).
  16-битные модели (`-ml`, `-ms`) и far-указатели — родные для Watcom;
  лицензия Sybase Open Watcom Public License (**предположение**, как у JWasm,
  проверить для 2.0). Второй кандидат; полезен тем, что даёт MZ-программы
  любых моделей.
- **Free Pascal.** Установлено: цель `i8086-msdos` — только кросс-компилятор
  (нативного нет); готовые архивы `fpc-3.2.2.x86_64-linux.cross.i8086-msdos.tar.xz`
  (нужен нативный FPC x86_64) на SourceForge
  (`https://sourceforge.net/projects/freepascal/files/msdos/3.2.2/`,
  `https://www.freepascal.org/down/i8086/msdos-canada.var`); из исходников:
  `make clean all OS_TARGET=msdos CPU_TARGET=i8086 OPT="-CX -XXs"
  CROSSOPT="-XP"`, для средней модели `CROSSOPT="-XP -WmMedium"` (по форуму
  Lazarus/FPC — `https://forum.lazarus.freepascal.org/index.php?topic=29769.0`).
  Для CI: скачать нативный FPC и кросс-архив (**предположение:** два
  архива, около сотни МБ — закэшировать). Нужен для прослойки DN (W7).
- **NASM** — для `.COM`-тестов моста (уже используется в `testdata/progs`,
  AGENTS.md).
- **Не проверено:** фактическая сборка тестовой программы каждым компилятором и
  её запуск в go2dos (особенно far call через `LDS/LES` в gcc-ia16 и
  Watcom); размер рантайма в `.exe` против нашего лимита памяти DOS.

## Вопросы владельца

1. R2: нужна ли побайтная совместимость с реальным WASI (тогда стоит прототип
   «адаптерный модуль на памяти DOS в wazero») или достаточно `wasidos` поверх
   Go `os`?
2. R3: допустим ли M12 без терминального фронтенда (только headless и
   браузер), раз `x/term` на wasip1 не работает?
3. R5: какой компилятор C считать основным для клиентского набора W6
   (gcc-ia16 против Open Watcom) — достаточно ли одного?
