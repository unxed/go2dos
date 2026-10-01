# Подготовка исходников для аудита (D3)

Этап D3 требует исходников как Dos Navigator 1.51, так и Turbo Vision из BP7. 
Ни то, ни другое в репозиторий не кладётся; вместо этого скрипты работают с 
файлами в каталогах вне репозитория, которые указывает пользователь.

## Как запустить подготовку

```bash
# 1. Скачайте исходники DN 1.51 в каталог /path/to/dn151-src/
#    (архив https://download.ritlabs.com/dn/dn151src.zip, SHA256 в DN-RESEARCH.md)

# 2. Скачайте или подготовьте исходники Turbo Vision из BP7 в /path/to/tv-bp7/
#    (обычно файл TVISION.ZIP из дистрибутива BP7)

# 3. Запустите подготовку (когда детекторы будут готовы):
#    python3 dn/audit/prep.py /path/to/dn151-src /path/to/tv-bp7 /path/to/audit-work/

# Результат: каталог /path/to/audit-work/ с нормализованными и индексированными версиями
```

## Структура исходников

### DN 1.51 (`dn151src.zip`)

Ожидаемая структура каталога (после распаковки `dn151src.zip`):

```
dn151-src/
├── *.PAS              # Pascal-модули (OBJECTS, VIEWS, DRIVERS, etc.)
├── *.ASM              # Ассемблер-модули  
├── RESOURCE/          # Языковые файлы и диалоги
│   ├── ENGLISH/
│   └── RUSSIAN/
├── BUILD.BAT          # Скрипты сборки
└── BPC.CFG            # Конфигурация компилятора
```

Модули по интересующему нас подмножеству:
- `OBJECTS.PAS` (TV Objects Unit)
- `DRIVERS.PAS` (TV Drivers Unit)
- `VIEWS.PAS` (TV Views Unit)
- `MENUS.PAS` (TV Menus Unit)
- `DIALOGS.PAS` (TV Dialogs Unit)
- `MEMORY.PAS` (TV Memory Unit)
- `HISTLIST.PAS` (TV History List Unit)
- `VALIDATE.PAS` (TV Validation Unit)
- `COLORSEL.PAS` (TV Color Selection Unit)
- `STRINGS.PAS` (TV Strings Unit)
- Собственные модули DN: `DNAPP.PAS`, `MICROED.PAS`, `DNSTDDLG.PAS`, `MESSAGES.PAS`, etc.

### Turbo Vision (BP7)

Ожидаемая структура (TVISION.ZIP или эквивалент из дистрибутива BP7):

```
tv-bp7/
├── OBJECTS.PAS
├── DRIVERS.PAS
├── VIEWS.PAS
├── MENUS.PAS
├── DIALOGS.PAS
├── MEMORY.PAS
├── HISTLIST.PAS
├── VALIDATE.PAS
├── COLORSEL.PAS
├── APP.PAS
├── STDDLG.PAS
├── MSGBOX.PAS
├── OUTLINE.PAS
├── TEXTVIEW.PAS
├── EDITORS.PAS
└── ...
```

## Что делает скрипт подготовки

1. **Сканирование файлов.** Находит все `.PAS` файлы в исходниках DN и TV

2. **Нормализация.**
   - Удаление комментариев (однострочные `//` и многострочные `{...}`)
   - Удаление лишних пробельных символов
   - Приведение ключевых слов к нижнему регистру (Pascal нечувствителен к регистру)
   - Токенизация

3. **Трансформации для детекторов:**
   - (A) Нормализованный текст с заменой идентификаторов на заглушки
   - (B) Нормализованный текст без замены (для построчного сравнения)
   - (C) Парсинг процедур и методов

4. **Индексирование.** Построение индексов для быстрого поиска

5. **Сохранение.** Результаты в `/path/to/audit-work/`:
   - `dn/` — нормализованные версии DN
   - `tv/` — нормализованные версии TV
   - `index.json` — метаданные (имена файлов, диапазоны, хэши)

## Контрольные данные

Для проверки детекторов заранее подготовлены контрольные файлы:
- `dn/audit/test-data/copied-proc.pas` — процедура из TV, скопированная с переименованием
- `dn/audit/test-data/independent.pas` — независимый код
- Ожидаемые результаты детекторов в `test-data/expected.json`

Запуск контроля: `python3 dn/detectors/test-detectors.py`
