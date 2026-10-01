# HX DOS Extender: исследование (имена файлов, буфер обмена, Far, сборка, лицензия)

Статус: исследование к `docs/DESIGN.md` §20 п. 3. Дата сбора: 2026-10-01.
Пометки: **установлено** — проверено по первоисточнику (путь в дереве или URL
приведены), **предположение** — вывод без проверки запуском.

Читалось дерево форка `https://github.com/unxed/HX` (ветка `master`, вершина
`f2276db` «v2.24pre1»; **установлено**: совпадает с вершиной
`Baron-von-Riedesel/HX` — форк пока без собственных изменений), релиз
`Baron-von-Riedesel/HX` v2.23 (пакеты `HXRT223.zip`, `HXGUI223.zip` с `DOC/*.TXT`)
и форум bttr-software. Пути ниже — относительно корня дерева форка
(`Src/…`, `SrcEmu/…`) или каталога `DOC` пакета HXRT. В дереве форка собираются
только исходники: двоичные файлы и `DOC/` лежат в релизных пакетах.

## 1. Как HX реализует имена файлов

**Установлено** (`SrcEmu/DKRNL32/*.ASM`, `SrcEmu/NTLFNHLP/ntlfnhlp.txt`,
`SrcEmu/DKRNL32/DKRNL32.TXT` §3.4, `DOC/LFN.TXT`, `Src/HDPMI/INT21API.ASM`).

1. **LFN API используется, но только через DOS.** Файловые функции Win32
   (`DKRNL32.DLL`, эмуляция `KERNEL32`) всегда сначала пробуют длинные
   версии функций DOS `INT 21h AX=71xx` и при отказе (признак «LFN не
   установлен») откатываются на старые 8.3-функции (`DKRNL32.TXT` §3.4: «will
   always try to use the LFN versions of DOS file functions. If these calls
   return with an error indicating LFN is not installed, the older, non-LFN
   versions are used»). Сам HX файловую систему не реализует — `DKRNL32`
   вызывает DOS. Подсчёт обращений `mov ax,71xxh`/`73xxh` в `SrcEmu/DKRNL32` и
   `NTLFNHLP`: `7100h` (проверка, 18 мест), `7139h/713Ah/713Bh` (каталоги),
   `7141h` (удаление), `7143h` (атрибуты), `7147h` (текущий каталог),
   `714Eh/714Fh/71A1h` (поиск), `7156h` (переименование), `7160h` (полное имя),
   `716Ch` (открыть/создать), `71A0h` (сведения о томе), `71A6h` (информация о
   файле по хэндлу), `7300h/7303h` (FAT32: сведения о диске).
2. **Перевод в защищённый режим делает DPMI-сервер.** HDPMI (`Src/HDPMI/INT21API.ASM`,
   процедура `intr2171`, включается `?LFNSUPPORT=1`) транслирует вызовы
   `71xx` из защищённого режима в реальный: `39h–3Bh`, `41h`, `43h`, `47h`, `4Eh`,
   `4Fh`, `56h`, `60h`, `6Ch`, `A0h`, `A2h`, `A6h`, `A7h` (преобразование времени),
   `A8h` (генерация короткого имени), `AAh` (SUBST); коды `71xx` ниже `39h` и
   неперечисленные ниже `AAh` уходят в реальный режим DOS без перевода
   указателей (CF заранее взведён), выше `AAh` — ответ «не поддерживается»
   (`AL=00`, CF=1). Трансляция отключается переменной `HDPMI=128`
   (`DOC/LFN.TXT`: для старых DOS, где вызов 71xx без драйвера возвращает CF=0 — см.
   там же). `NTLFNHLP` делает то же для NT-подсистемы DOS, где перевода нет
   (`ntlfnhlp.txt`: «will only get active if it runs in a DOS box on NT
   platforms»).
3. **Функции `...W` — оболочки над `...A`, не Unicode.** `CreateFileW`
   (`CREFILEW.ASM`), `FindFirstFileW` (`FNDFILEW.ASM`) и остальные превращают
   строку UTF-16 в байтовую, **отбрасывая старший байт каждого символа**
   (`CONVWSTR.ASM`, `ConvertWStr`: цикл `lodsw` / `stosb`; в `FNDFILEW.ASM`
   `ConvW2A`/`ConvA2W` — то же для `WIN32_FIND_DATA`), и вызывают `...A`. Эту
   процедуру используют 48 файлов `.ASM` в `DKRNL32` (подсчёт `grep`). Так же
   устроена `WideCharToMultiByte` (`WC2MB.ASM`: кодовая страница игнорируется,
   копируется младший байт). `CP_UTF8`/`65001` в дереве не встречается.
   Следствие: символы с кодом больше 255 теряются, имена вне OEM-страницы
   недоступны (**установлено** чтением кода).
4. **`...A`-функции передают байты DOS как есть**, то есть имена в OEM-странице
   DOS (по умолчанию). Среднюю часть цепочки (HDPMI, DOS) наши UTF-8 имена
   (`docs/UTF8NAMES.md`) не меняют: режим включается на процесс через AMIS.

**Что это значит для нас.** (**предположение**, логический вывод из пунктов
выше): точка вставки UTF-8 одна — `ConvertWStr`/`ConvertAStrN`
(`CONVWSTR.ASM`) и `ConvW2A`/`ConvA2W` (`FNDFILEW.ASM`), плюс `WC2MB.ASM`:
вместо отбрасывания старшего байта делать UTF-16 ↔ UTF-8 и один раз при
запуске включить режим AMIS для процесса. Это совпадает с пунктом DESIGN §20
(`...W` через UTF-8 имена §19). `...A`-функции не меняются. Сначала нужно
убедиться, что у `...A` длинных имён хватает буферов (255/260 байт — п. 3 §UTF8NAMES).

## 2. Буфер обмена в SrcEmu

**Установлено** (`SrcEmu/DUSER32/CLIPBOAR.ASM`, 348 строк; `DOC/DUSER32.TXT`:
«clipboard ok»).

- Реализованы `OpenClipboard`, `CloseClipboard`, `EmptyClipboard`,
  `GetClipboardData`, `SetClipboardData`, `IsClipboardFormatAvailable`,
  `CountClipboardFormats`, `EnumClipboardFormats`, `RegisterClipboardFormatA`,
  `GetClipboardFormatNameA`, `GetClipboardOwner`, `GetOpenClipboardWindow`;
  `SetClipboardViewer`, `ChangeClipboardChain`, `GetPriorityClipboardFormat` —
  заглушки. Вариантов `...W` для имён форматов в файле нет.
- Хранение — **внутренний список в процессе** (`LISTITEM`: формат + хэндл
  `GlobalAlloc`). Между процессами и с DOS-стороной буфер не связан.
- Единственный мост к реальному буферу — переключатель `HXVDD` (`HXVDD equ 1`):
  на **Windows NT** `DUSER32` загружает драйвер виртуальной машины DOS
  `HXVDD.DLL` (`SrcEmu/DKRNL32/LOADVDD.ASM`, `Include/HXVDD.INC`;
  `DOC/HXVDD.TXT`) и перенаправляет вызовы буфера на хост (`VDD_OPENCLIPBOARD`,
  `VDD_GETCLIPBOARDDATA`, `VDD_SETCLIPBOARDDATA`, `VDD_ISCLIPBOARDFORMATAVAILABLE`).
  Исходника `HXVDD.DLL` в дереве нет: `Src/MAKEFILE` (строки 177–178) копирует
  `HXVDD\Release\HXVDD.DLL`, а каталога `Src/HXVDD` в форке нет
  (**установлено** листингом `Src`).
- **Вызовов `INT 2Fh AX=17xxh` (WinOldAp) в HX нет**: значений `1700h`, `1701h`,
  `1702h`, `1703h`, `1705h`, `1708h` нет нигде в `Src`, `SrcEmu`, `Include`
  (поиск `grep`); `INT 2Fh` используется для `1680h` (простой), `150Bh`,
  `1508h` (CD-ROM, физические диски), `GETDRVTP.ASM`, `PHYSDRV.ASM`.

**Что это значит.** (**предположение**) Две возможности подключить буфер
хоста: (а) в `CLIPBOAR.ASM` заменить ветку `HXVDD` вызовами WinOldAp
`INT 2Fh AX=17xxh` (наш сервер — DESIGN §9: `1700h–1705h`, `1708h`), с форматами
`CF_TEXT`/`CF_OEMTEXT` ↔ DOS-байты и `CF_UNICODETEXT` ↔ UTF-16/UTF-8 перевод
в `DUSER32`; (б) оставить `DUSER32` как есть и поднять наш `HXVDD`-подобный
хост — второе сложнее (нужен BOP-механизм NT). Выбор (а) согласуется с
правилом «готовый документированный интерфейс». Кодировку, которую Far кладёт
в `CF_TEXT`, надо установить по самому Far (`DOUBTS.md`, «Буфер обмена»).

## 3. Какая версия Far Manager работала под HX

**Установлено** (форум bttr-software «DOS ain't dead», тема «HX-DOS and FAR
manager», `https://www.bttr-software.de/forum/mix_entry.php?id=3628`):

- 26.03.2008, Laaca: «Unfortunately it doesn't work yet (at least the 1.71
  version — I didn't tried the earlier ones)» — версия **1.71** названа как
  не работавшая; там же RayeR: пробовал «more than one FAR version», какие —
  не помнит.
- 28.03.2008, rr и 28.04.2008, grompe: не хватало импортов
  `KERNEL32` (`DefineDosDeviceA`), `USER32` (`ShowWindowAsync`,
  `GetKeyboardLayoutList`, `VkKeyScanExA`, `IsCharLowerA`, `IsCharUpperA`),
  `WINSPOOL.drv` (`ClosePrinter`, `EndDocPrinter`, `WritePrinter`,
  `StartDocPrinterA`, `OpenPrinterA`, `EnumPrintersA`), `SHELL32`
  (`FindExecutableA`, `ShellExecuteExA`); grompe «hacked far.exe to remove these
  imports, and it ran! Although filenames were shown strangely with mixed case, it
  was able to browse directories, drives, launch programs, view and edit files»,
  плагины не загружались.
- 22.03.2009, Japheth (автор HX): «With the current hx prerelease v2.16 one
  might be able to run FAR manager in DOS … there are surely things which
  won't work (yet)»; работал FTP-клиент Far (нужны правки реестрового файла `DOS`
  в каталоге `DADVAPI.DLL`, затем исправлено 28.03.2009: «The bugs are
  fixed»); «Please note that FAR needs LFN support in DOS». Laaca 23.03.2009
  и 29.03.2009: «it works very well», «Yes, it works!».
- **Какая именно версия Far работала в 2009 — в теме не названа.** Об
  экземпляре говорит лишь то, что у Far был FTP-клиент (подключаемый модуль
  поставляется с Far 1.x) и что Far использовал реестр Win32 с ключом
  `Software\Far\Plugins\FTP`. **Предположение:** это Far 1.7x (ANSI/OEM-ветка,
  вызывает `...A`-функции); как раз их HX и поддерживает. Far 2.x и 3.x
  (Unicode, `...W`) этим экспериментом не подтверждены, и последний требует
  более новых API (**не проверялось**).
- В списке совместимости `DOC/COMPAT.TXT` релиза 2.23 (список успешно проверенных
  Win32-консольных программ) Far **отсутствует** (поиск по тексту).
- Другие свидетельства (`necromancer's DN` под HXRT; тот же форум, 26.03.2008,
  Rugxulo/Laaca): Win32-вариант NDN запускался под HX с ошибками сокетов
  (`Winsock error 11001`, `EXCEPTION 0D8h`) — это тот же класс задач.

**Чего нет и надо выяснить:** собрать прогон Far 1.71 (дистрибутив с farmanager.com
или сайта Far Group) под HX на нашем стенде. Скачивать его нужно только на
машине, не в репозиторий.

## 4. Чем собирать HX в CI

**Установлено** (`HXsrc.txt` §2, `Src/MAKEFILE`, `Src/DIRS`, `SrcEmu/DIRS`,
`SrcEmu/DKRNL32/MAKEFILE`):

- Нужны: пакет **HXDEV** (заголовки, библиотеки, утилиты — релизный артефакт
  `HXDEV223.zip` и т.п.), **JWasm**, **JWlink ≥ v19beta15**, **JWlib**
  (входит в JWlink), **NMake или Open Watcom WMake** (`wmake -ms`), **WRC**
  (Open Watcom, нужен для `DUSER32.DLL`) — это перечень из `HXsrc.txt`.
- Makefile написаны для **NMake** с путями через обратную косую (`cd
  ..\HDPMI`, `!include <dirs>`, `\hx`); в `DIRS` пути `HXINST`, `LIBCOFF`
  задаются вручную и перед сборкой правятся.
- В процессе сборки самих себя используются собственные утилиты HX — `EditPE`,
  `ExtrMZ`, `PEStub`, `PatchPE` (`Src/HDPMI/HDPMI16I.MAK`, строки с `@EditPE
  …`); они — Win32-консольные PE и поставляются в HXDEV.
- Исходники JWasm — `https://github.com/JWasm/JWasm` (есть `GccUnix.mak`,
  `OWLinux.mak`, `CMakeLists.txt`; сборка на Linux описана в `Readme.txt`),
  JWlink — `https://github.com/Baron-von-Riedesel/jwlink` (Makefile `GccUnix.mak`).
  Бинарники Linux для JWlink по ссылке SourceForge — v19b11 (**установлено**
  по листингу страницы результатов поиска), то есть ниже требуемой 19beta15;
  нужна сборка из исходников. Open Watcom 2.0: релизы
  `open-watcom/open-watcom-v2` («Current-build», «2026-09-01-Build») содержат
  Linux x64 (`open-watcom-2_0-c-linux-x64`).

**Вывод (предположение, не проверено сборкой):**

1. **Самый простой путь — runner `windows-latest`**: NMake входит в состав Visual
   Studio Build Tools на runner'е, JWasm/JWlink/JWlib и WRC — готовые
   Windows-архивы, HXDEV — релизный пакет, пути `\hx` на `C:\hx`. Утилиты HX
   (EditPE и др.) запускаются как обычные Win32-программы. Так никакой
   эмуляции не требуется, и CI видит результат в виде артефакта.
2. **Linux-runner потребует:** сборки JWasm/JWlink/JWlib из исходников, Open
   Watcom `wmake` (понимает `-ms`, но пути с `\` и команда `cd ..\X` ему нужно
   переписать — **предположение**; вариант — Wine для Win32-утилит HX).
   Рассматривать, только если Windows-runner не подойдёт.
3. Первый шаг — пробная сборка `Src` без изменений, затем `SrcEmu`
   (отдельный PR с workflow, потом артефакт `HXRT*.zip`). `WSOCK32` из сборки
   исключён (`HXsrc.txt`: «some modules are missing due to copyright issues»).
4. Проверка результата — запустить собранные `HDPMI32`/`DPMILD32` в **go2dos
   нельзя** до появления 386 и защищённого режима (§5). Поэтому на первом этапе
   проверка — запуск HX-бинарников в DOSBox-X/FreeDOS QEMU в CI (как для VC)
   либо вообще только сборка.

## 5. Лицензия: условия использования и изменения

**Установлено:**

- `HXsrc.txt` §5: «The HX DOS extender is freeware. Copyright 1996-2026
  Japheth.» `Readme.txt` форка — без условий. `HXRT.TXT` (§8, релиз 2.23): «The HX
  DOS extender runtime is freeware and may be used for any purpose. Copyright
  Japheth 1996-2024. The HX runtime can be freely distributed with any
  application.»
- **Условия на изменение и распространение изменённых исходников нигде не
  оговорены.** Файла лицензии (`LICENSE`/`COPYING`) в репозитории нет, поле
  лицензии на GitHub пустое (`gh api repos/Baron-von-Riedesel/HX`: `license: null`).
- Issue #1 «Any chance to clarify the licence?»
  (`https://github.com/Baron-von-Riedesel/HX/issues/1`, 2018–2022): автор
  15.06.2018: «I guess the license will be changed, once I have done the code
  cleanup»; 17.04.2022: «Honestly, I don't like gpl … and haven't had time and
  energy to read MIT or similiar things»; на 2026 год лицензии в репозитории нет
  (**установлено** листингом корня на 2026-10-01).
- В дереве есть чужой код под другими условиями: `Src/OWSUPP/src_*/MDEF.INC` и
  другие файлы — «Portions Copyright (c) 1983-2002 Sybase, Inc.», Sybase Open
  Watcom Public License 1.0; `SrcEmu/WSOCK32/WinSock.h` — Copyright Microsoft с
  оговорками Berkeley. JWasm — тоже Sybase Open Watcom Public License
  (`JWasm/License.txt`; поле лицензии на GitHub — `NOASSERTION`).
- Среди комментариев в issue есть просьбы разных людей выбрать лицензию (MIT,
  LGPL, публичное достояние) — автор не ответил по существу.

**Предположение (не юридическое заключение):** слово «freeware» без
дополнительных условий допускает бесплатное использование и распространение
исходников и бинарников, но не даёт ясного права *изменять и распространять
изменённые* версии. Для нашего плана (изменения HX живут в форке
`unxed/HX`, в go2dos — ссылка и тесты — DESIGN §20) разумно **спросить
Japheth напрямую** (в issue #1 или письмом) о разрешении на форк с
изменениями и их распространение; решение и вопрос — на владельце.

### 5.1. Проверка 2026-10-01: ответ автора найден

**Установлено** (`gh api`, ссылки ниже; все цитаты дословные):

- Issue [#66](https://github.com/Baron-von-Riedesel/HX/issues/66) «Commercial
  usage allowed?», 03.06.2026, ответ Japheth (`Baron-von-Riedesel`): «Yes, so
  long as it isn't modified.» То есть использование неизменённого HX
  разрешено, изменение — нет (во всяком случае не разрешено явно).
- PR [#69](https://github.com/Baron-von-Riedesel/HX/pull/69) «Added the MIT
  license» (leiradel, 15.07.2026, не влит, открыт), комментарий Japheth
  17.07.2026: «There's already a "derivative" variant of HDPMI32i, created by
  crazii and used by SBEMU. It's incompatible with the standard HDPMI. The
  prospect of dozens "improved" variants emerging isn't something I want to
  encourage...» Лицензию он по-прежнему не выбрал; на 2026-10-01 в корне
  репозитория нет файла лицензии.
- Issue [#1](https://github.com/Baron-von-Riedesel/HX/issues/1) (лицензия, 17
  комментариев, последний 04.08.2022, открыт), [#8](https://github.com/Baron-von-Riedesel/HX/issues/8)
  «Distribution of HX DOS Extender» (вопрос, можно ли изменять и включать
  изменённый HX в открытое ПО, 22.06.2020; без ответа), [#67](https://github.com/Baron-von-Riedesel/HX/issues/67)
  («bus factor», ответ без касательства к лицензии).
- Форки `Baron-von-Riedesel/HX` (`gh api repos/Baron-von-Riedesel/HX/forks`, 21
  форк): ни у одного нет лицензии (`license: null`); с изменениями — только
  `crazii/HX` (47 коммитов вперёд, последний 2024-03-01; описание «IO port
  trapping of HDPMI», та самая несовместимая ветка HDPMI32i для SBEMU);
  `leiradel/HX` — ветка PR #69; остальные совпадают с оригиналом или отстают от него.
- Форк `unxed/HX`: публичный, лицензии нет, `gh api repos/unxed/HX/compare/Baron-von-Riedesel:master...unxed:master`
  — `identical` (0 вперёд, 0 назад; вершина v2.24pre1, 2026-05-31), то есть своих
  изменений в нём нет. Оставлен как есть.
- Как связаться с автором: адреса электронной почты нет ни в `HXsrc.txt`,
  `Readme.txt`, исходниках, ни в `DOC/*.TXT` пакета HXRT 2.23, ни в профиле GitHub
  `Baron-von-Riedesel` (поле почты пусто); сайт `japheth.de` на 2026-10-01 отвечает
  «Sorry, the website has been stopped». Работающий канал — issues/PR в
  `Baron-von-Riedesel/HX` (автор отвечает там в июне–июле 2026) и форум
  bttr-software (`https://www.bttr-software.de/forum/`, где он писал в 2008–2009).

**Что из этого следует для go2dos** (вывод, не юридическое заключение): пока
автор не разрешил явно, мы **не распространяем изменённый HX** — ни форк, ни
изменённые бинарники. Неизменённые релизные пакеты (`HXRT*.zip`) использовать
можно. Вопрос, допустимо ли распространять только патчи к оригиналу, автору не
задан и ответа нет; патчей к HX у проекта на 2026-10-01 нет, поэтому каталога
`third_party/hx-patches/` и скрипта применения не создано. Прежнее решение
DESIGN §20 п. 3 («изменения HX живут в форке») этим уточнено: форк можно держать
только как зеркало без изменений. Вопрос автору — тикет в `unxed/go2dos`
(«HX: ask Japheth …»).

## 6. Чего не хватает в go2dos для HX

**Установлено** по требованиям HX (`DOC/HDPMI.TXT` §1–2; `SrcEmu/DKRNL32/DKRNL32.TXT`
§2) и по текущему коду go2dos (`docs/DESIGN.md` §4, §13; ветка `feat/lfn`):

| Область | Что требует HX | Что есть в go2dos |
|---|---|---|
| CPU | HDPMI: «80386 or better»; клиенты — 32-битный код (регистры EAX.., префиксы `66`/`67`, `0F`-опкоды: `MOVZX`, `MOVSX`, `SETcc`, `Jcc rel32`, `BT*`, `SHLD/SHRD`, `IMUL r,rm`, …) | 8086 + 80186; `66`/`67` и `0F` — fail fast |
| Защищённый режим | дескрипторы, GDT/LDT/IDT, TSS, привилегии, исключения; HDPMI («paging») при `-a` — отдельные адресные контексты | нет; в `cpu.State` есть кэш дескриптора сегмента (задел §4) |
| Страничная память | таблицы страниц, 4 ГБ | нет; только 1 МБ + HMA |
| Расширенная память | ≥ 72 КБ, XMS или raw A20 | A20 есть, XMS/INT 15h расширенной памяти — не проверено |
| DPMI | версия 0.9 (HDPMI: «0.9 plus a large subset of 1.0»); `INT 2Fh AX=1687h` вход, `INT 31h` | нет |
| Железо для `DKRNL32` | контроллер клавиатуры 8042 + IRQ1 + BDA; VGA в текстовом режиме с прямым доступом; RTC IRQ8; PIT; мышь `INT 33h` | PIC/PIT, порт 60h/IRQ1, видеопамять — есть; RTC (IRQ8, порты 70h/71h) — не проверено |
| DOS API | `INT 21h` 71xx: `7139h/3Ah/3Bh`, `41h`, `43h`, `47h`, `4Eh/4Fh`, `56h`, `60h`, `6Ch`, `A0h`, `A1h`, `A6h`, `A7h`, `A8h`, `AAh`; FAT32 `7300h/7303h`; проверка `7100h` | `feat/lfn`: `0Dh, 39h, 3Ah, 3Bh, 41h, 43h, 47h, 4Eh, 4Fh, A2h, A1h, 56h, 60h, 6Ch, A0h`; **нет** `A6h`, `A7h`, `A8h`, `AAh`, `7300h/7303h`; `7100h` — проверить |
| `INT 2Fh` | `1680h` (простой), `150xh`/`1508h` (CD-ROM, физические диски; `GETDRVTP.ASM`, `PHYSDRV.ASM`) | `1680h` есть; `150xh` — нет |
| Буфер обмена | `INT 2Fh AX=17xxh` (после замены ветки HXVDD, §2) | `17xxh` планируется (M8), не реализован |

**Два пути к DPMI** (DESIGN §20): (1) **настоящий HDPMI32 как код** — нужны все
строки таблицы (386, защищённый режим, страницы, A20/XMS, переход между
режимами); сборка HDPMI уже описана в §4. Преимущество: точность, HX целиком
как есть; (2) **HLE-хост DPMI** — в Go, поверх `cpu` с 386 и плоскими 32-битными
сегментами, но без страничной адресации и без собственного кода HDPMI: селекторы
LDT, выделение памяти, трансляция вызовов DOS напрямую в пакет `dos` (в том
числе `71xx` без `real-mode translation`). Минус: требуется эмулировать
поведение, на которое рассчитывает `DPMILD32`/`DKRNL32` (ловушка 31h, режим
переключения). **Предположение:** (2) короче для запуска Win32-консоли, (1) нужен,
если цель — тестировать сам HX. Выбор — отдельный этап после M12; для обоих
необходимы 32-битные инструкции с префиксами и флагами 386, и значит расширение
набора тестов (`docs/DOUBTS.md`, «CPU»).

## 7. Открытые вопросы и следующие шаги

1. Пробная сборка `unxed/HX` без изменений на `windows-latest` (отдельный PR с
   workflow; артефакт — `HXRT*.zip`). Не проверено, что `master` собирается.
2. Установить версию Far, выбранную для эксперимента, и проверить работу под
   HX 2.23/2.24 в DOSBox-X (до появления 386 в go2dos).
3. Спросить Japheth о лицензии на изменения (вопрос владельцу ниже).
4. Определить, чего именно ждёт Far от `CF_TEXT`/`CF_OEMTEXT` (Far 1.7x).
5. Реализовать `A6h/A7h/A8h/AAh` и `7300h/7303h` в go2dos — это поток A, не
   этот документ.

**Вопрос владельцу (настоящий):** можно ли написать Japheth (issue #1 в
`Baron-von-Riedesel/HX` или письмо) с просьбой разрешить изменение и
распространение изменённого HX (форк `unxed/HX`)? Без ответа форк имеет
лишь «freeware» без явных прав на изменения.
