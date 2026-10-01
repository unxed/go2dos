# Запуск аудита (D3): полный цикл

## Предусловия

1. **Исходники DN 1.51** доступны локально (или будут скачаны скриптом)
   ```bash
   # Вариант 1: скачать самостоятельно
   curl -o /tmp/dn151src.zip https://download.ritlabs.com/dn/dn151src.zip
   unzip /tmp/dn151src.zip -d /tmp/dn151-src
   
   # Вариант 2: использовать скрипт prep.py (когда будет готов)
   ```

2. **Исходники Turbo Vision из BP7** (или Free Vision эквивалент)
   - Если есть BP7: обычно `TVISION.ZIP` или папка `LIB/` в дистрибутиве
   - Если нет: можно использовать Free Vision из Free Pascal, затем вручную проверить совместимость

3. **Python 3.8+** для запуска детекторов

## Фаза 1: Валидация детекторов (1-2 часа)

### Шаг 1.1: Запустить детекторы на контрольных данных

```bash
cd /home/ivan/go2dos

# Детектор A (k-граммы)
python3 dn/detectors/detector_a.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_a.json \
  --k 5

# Детектор B (построчное)
python3 dn/detectors/detector_b.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_b.json

# Детектор C (попроцедурное)
python3 dn/detectors/detector_c.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_c.json \
  --threshold 0.7
```

### Шаг 1.2: Валидировать результаты

```bash
python3 dn/detectors/validate-tests.py \
  dn/detectors/test-data \
  /tmp/test_a.json \
  /tmp/test_b.json \
  /tmp/test_c.json
```

**Ожидаемый вывод:**
```
✅ Detector A: Found TV:tv_original.pas vs DN:dn_copied_renamed.pas with similarity 0.XX
✅ ALL TESTS PASSED - Detectors are ready for audit
```

### Шаг 1.3: Если тесты не прошли

Отрегулировать параметры в зависимости от результатов:

| Проблема | Решение |
|----------|---------|
| Детектор A не находит копию | Уменьшить `--k` (например, с 5 на 4) или `window_threshold` в коде |
| Слишком много ложноположительных | Увеличить пороги |
| Детектор B слишком чувствителен | Увеличить `block_min_length` |
| Детектор C не работает | Проверить парсер Pascal в detector_c.py |

Затем повторить шаги 1.1–1.2.

## Фаза 2: Подготовка исходников (2-4 часа)

### Шаг 2.1: Получить исходники DN и TV

```bash
# DN 1.51 (если ещё не скачаны)
mkdir -p ~/.audit-work
curl -o ~/.audit-work/dn151src.zip https://download.ritlabs.com/dn/dn151src.zip

# Проверить SHA256 (из DN-RESEARCH.md)
echo "d2d12bad4a040e751d5a6f4186a8caabd5ba8b3540e3dc7cab69a58a4f210440  ~/.audit-work/dn151src.zip" | sha256sum -c

# Распаковать
unzip ~/.audit-work/dn151src.zip -d ~/.audit-work/dn151-src

# TV: предположим, что уже есть в BP7 или другом месте
# Или использовать Free Vision
# (скрипт prep.py должен помочь с этим)
```

### Шаг 2.2: Подготовить данные для аудита (когда prep.py будет готов)

```bash
python3 dn/audit/prep.py \
  ~/.audit-work/dn151-src \
  /path/to/tv-bp7-sources \
  ~/.audit-work/audit-prepared
```

Результат: `~/.audit-work/audit-prepared/` с нормализованными файлами и индексами.

## Фаза 3: Полный прогон аудита (4-8 часов)

### Шаг 3.1: Запустить все три детектора на DN и TV

```bash
# Когда prep.py и полный audit-run.sh будут готовы:
bash dn/audit/run-audit.sh \
  ~/.audit-work/dn151-src \
  /path/to/tv-bp7-sources \
  ~/.audit-work/audit-results
```

Результат:
- `~/.audit-work/audit-results/detector_a.json` — k-граммы
- `~/.audit-work/audit-results/detector_b.json` — построчное
- `~/.audit-work/audit-results/detector_c.json` — попроцедурное
- `~/.audit-work/audit-results/merged.json` — объединённые результаты

### Шаг 3.2: Заполнить реестр аудита

```bash
python3 dn/audit/merge-results.py \
  ~/.audit-work/audit-results/merged.json \
  dn/audit/register.csv
```

Этот скрипт (когда будет готов) преобразует результаты детекторов в реестр CSV с полями:
- `dn_file`, `dn_lines` — файл и строки в DN
- `tv_module`, `tv_lines` — модуль и строки в TV
- `detector` — какой детектор срабатывает
- `similarity` — мера сходства
- `decision` — требуется ручное решение

## Фаза 4: Ручной анализ результатов (8-16 часов)

### Шаг 4.1: Просмотреть результаты с высокой мерой сходства

```bash
# Отсортировать реестр по сходству (убывание)
sort -t, -k8 -rn dn/audit/register.csv | head -50
```

### Шаг 4.2: Для каждого результата решить

| Решение | Когда | Действие |
|---------|-------|---------|
| `переписать` | Код явно скопирован; реализация переписывается | Включить в D4 спецификацию |
| `совпадение по интерфейсу` | Имена одинаковые, но это требуется для совместимости | Не трогать, отметить в реестре |
| `ложное` | Детектор ошибся (похожие алгоритмы, но независимые) | Добавить в negative test-data |

Заполнить колонки в `dn/audit/register.csv`:
- `decision` — решение
- `decision_by` — инициалы или имя
- `decision_date` — дата
- `notes` — комментарии

### Шаг 4.3: Сгруппировать результаты для D4

```bash
# Выведет список файлов/модулей для переписи
grep "^[^#].*переписать" dn/audit/register.csv | cut -d, -f1,2 | sort -u
```

Результат -> список файлов для спецификаций в `dn/specs/`.

## Фаза 5: Двойная проверка (после D4)

После того как D4 (переписка) завершена, повторить тесты D3 на новом коде:

```bash
python3 dn/detectors/detector_a.py \
  dn/tv-clean/tv-rewritten \
  ~/.audit-work/tv-bp7-sources \
  --output /tmp/verify_a.json

# ... аналогично B и C

# Ожидание: ноль или почти ноль срабатываний выше порога
```

## Инструмент для исследования результатов

```bash
# Просмотреть все срабатывания для конкретного файла DN
grep "OBJECTS.PAS" dn/audit/register.csv | head -5

# Просмотреть все соответствия с файлом TV
grep "VIEWS.PAS" dn/audit/register.csv | tail -10
```

## Чек-лист для завершения D3

- [ ] Детекторы скомпилированы/проверены на контрольных данных
- [ ] Параметры подобраны (k, пороги)
- [ ] Получены исходники DN 1.51 и TV
- [ ] Прогон всех трёх детекторов завершён
- [ ] Результаты объединены в один JSON
- [ ] Реестр `dn/audit/register.csv` заполнен
- [ ] Ручной анализ завершён (все строки с решением)
- [ ] D4 список подготовлен (какие файлы переписывать)

После завершения D3 можно начинать D4 (спецификации и переписка).

## Возможные проблемы

| Проблема | Решение |
|----------|---------|
| `ModuleNotFoundError: No module named 'X'` | Установить: `pip3 install X` |
| Детектор очень медленный на больших файлах | Оптимизировать алгоритм или разбить на параллельные потоки |
| Результаты противоречивые (детекторы не согласны) | Ожидаемо; ручная проверка для разрешения конфликтов |
| Не хватает памяти при нормализации | Разбить файлы на части или увеличить лимит памяти |
