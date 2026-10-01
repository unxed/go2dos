# Тестирование детекторов (контроль, шаг 4 в DN-PLAN.md)

Перед прогоном детекторов A, B, C по реальным исходникам DN и TV нужно 
убедиться, что они работают правильно на контрольных данных.

## Структура тестов

```
test-data/
├── positive/
│   ├── copied_proc_renamed.pas       # Копия из TV с переименованием
│   ├── copied_with_reformat.pas      # Копия с переформатированием
│   └── ...
├── negative/
│   ├── independent_code.pas          # Независимый код
│   ├── similar_but_different.pas     # Похожий алгоритм, но своё
│   └── ...
└── expected.json                     # Ожидаемые результаты
```

## Положительные тесты

Каждый положительный тест содержит:
- Оригинальную процедуру из Turbo Vision (из BP7 или известного источника)
- Копию этой процедуры с изменениями:
  - Переименование всех переменных
  - Переформатирование (добавление/удаление пробелов, переносы строк)
  - Изменение стиля комментариев (однострочные ↔ многострочные)
  - Иногда: переустановка `begin/end` на инлайн-ассемблер и обратно

**Ожидание:** Все три детектора должны дать высокую оценку сходства.

Пример (процедура `MemAvail` из Memory.pas TV):

```pascal
{
  TV оригинал:
  function MemAvail: Longint;
  begin
    if PSP^.Nextblk = 0 then
      MemAvail := FreeList^.Sizes
    else
      MemAvail := 0;
  end;
  
  Копия с переименованием (как в positive/copied_proc_renamed.pas):
  function GetFreeMemory: Longint;
  begin
    if PSPBlock^.NextBlock = 0 then
      GetFreeMemory := FreeMemoryList^.Dims
    else
      GetFreeMemory := 0;
  end;
}
```

## Отрицательные тесты

Каждый отрицательный тест содержит независимый код, который:
- Может быть похож на TV в классе (например, работа со списками)
- Но реализует другой алгоритм или логику
- Не должен пересекаться с конкретной процедурой TV в результатах детекторов

Пример (собственная реализация поиска):

```pascal
{ Собственный поиск в массиве - похож на TV, но независимый }
function FindInArray(const Arr: Array; Value: Integer): Integer;
var
  i: Integer;
begin
  Result := -1;
  for i := Low(Arr) to High(Arr) do
  begin
    if Arr[i] = Value then
    begin
      Result := i;
      Exit;
    end;
  end;
end;
```

## Запуск тестов

```bash
# Тестировать детектор A
python3 dn/detectors/detector_a.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_a.json

# Тестировать детектор B
python3 dn/detectors/detector_b.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_b.json

# Тестировать детектор C
python3 dn/detectors/detector_c.py \
  dn/detectors/test-data/positive \
  dn/detectors/test-data/negative \
  --output /tmp/test_c.json

# Сравнить с ожидаемыми результатами
python3 dn/detectors/validate-tests.py \
  /tmp/test_a.json \
  /tmp/test_b.json \
  /tmp/test_c.json \
  dn/detectors/test-data/expected.json
```

## Ожидаемые результаты (expected.json)

Структура:

```json
{
  "positive_tests": [
    {
      "name": "copied_proc_renamed",
      "detector_a": {
        "should_detect": true,
        "min_similarity": 0.8,
        "description": "K-gram детектор должен найти копию несмотря на переименование"
      },
      "detector_b": {
        "should_detect": false,
        "description": "Построчное сравнение может не найти из-за переформатирования"
      },
      "detector_c": {
        "should_detect": true,
        "min_similarity": 0.75,
        "description": "Процедурный детектор должен найти по сигнатуре и токенам"
      }
    }
  ],
  "negative_tests": [
    {
      "name": "independent_code",
      "all_detectors": {
        "should_detect": false,
        "max_similarity": 0.3,
        "description": "Независимый код не должен срабатывать"
      }
    }
  ]
}
```

## Калибровка порогов

На основе тестов подбираются пороги для каждого детектора:

- **Детектор A:** минимальная длина совпадающей последовательности в токенах (по умолчанию 10)
- **Детектор B:** минимум совпадающих строк (по умолчанию 3)
- **Детектор C:** минимум сходства процедур (по умолчанию 0.7)

Пороги можно корректировать в зависимости от целей (снижение ложноположительных 
или ложноотрицательных срабатываний).

## После успешного тестирования

Когда все тесты проходят с нужной точностью, детекторы готовы к прогону по DN и TV.
Результат: реестр совпадений в `dn/audit/register.csv`.
