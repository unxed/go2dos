#!/usr/bin/env python3
"""
Детектор C: попроцедурное сравнение.

Парсит процедуры/функции/методы из Pascal-кода и вычисляет меру сходства
между ними на основе совпадения токенов.
"""

import sys
import json
import re
from typing import List, Dict, Tuple, Optional
from pathlib import Path
from collections import defaultdict

from normalize import normalize_for_token_matching, tokenize


class PascalProcedureParser:
    """Парсер Pascal-процедур."""

    def __init__(self):
        self.procedures = {}  # (file, proc_name) -> (signature, body_tokens)

    def extract_procedures(self, filepath: str, text: str) -> Dict[str, Tuple[str, List[str]]]:
        """
        Извлекает процедуры/функции/методы из Pascal-файла.

        Возвращает словарь: proc_name -> (signature, body_tokens)
        """
        procedures = {}

        # Простой регулярный парсер для Pascal
        # Ищем паттерны: procedure/function/method <name>(...);
        pattern = r'(?:procedure|function|constructor|destructor)\s+(\w+)\s*(\([^)]*\))?'

        for match in re.finditer(pattern, text, re.IGNORECASE):
            proc_name = match.group(1)
            signature = match.group(0)

            # Найти начало и конец процедуры (приблизительно)
            start = match.end()

            # Ищем следующую процедуру или конец файла
            next_match = None
            remaining_text = text[start+1:]
            next_proc_match = re.search(pattern, remaining_text, re.IGNORECASE)
            if next_proc_match:
                end = start + 1 + next_proc_match.start()
            else:
                end = len(text)

            body = text[start:end]
            normalized = normalize_for_token_matching(body)
            tokens = normalized.split()

            procedures[proc_name] = {
                'signature': signature,
                'tokens': tokens,
                'file': filepath,
                'start': match.start(),
            }

        self.procedures[filepath] = procedures
        return procedures

    def get_procedure(self, filepath: str, proc_name: str) -> Optional[Dict]:
        """Получает процедуру из файла."""
        if filepath in self.procedures and proc_name in self.procedures[filepath]:
            return self.procedures[filepath][proc_name]
        return None


class ProcedureDetector:
    """Детектор совпадений на уровне процедур."""

    def __init__(self, similarity_threshold: float = 0.7):
        """
        Инициализация детектора.

        Args:
            similarity_threshold: минимальная мера сходства для срабатывания
        """
        self.similarity_threshold = similarity_threshold
        self.parser = PascalProcedureParser()
        self.files = {}

    def add_file(self, filepath: str, text: str):
        """Добавляет файл для анализа."""
        self.files[filepath] = text
        self.parser.extract_procedures(filepath, text)

    def compute_similarity(self, tokens1: List[str], tokens2: List[str]) -> float:
        """
        Вычисляет меру сходства между двумя списками токенов.

        Использует простую метрику: доля совпадающих токенов.
        """
        if not tokens1 or not tokens2:
            return 0.0

        # Создать множества токенов
        set1 = set(tokens1)
        set2 = set(tokens2)

        if not set1 or not set2:
            return 0.0

        # Пересечение и объединение
        intersection = len(set1 & set2)
        union = len(set1 | set2)

        return intersection / union if union > 0 else 0.0

    def find_matching_procedures(self) -> List[Dict]:
        """
        Находит совпадающие процедуры между файлами.

        Возвращает список словарей с совпадениями выше порога.
        """
        results = []
        files = list(self.files.keys())

        # Извлечь все процедуры
        all_procedures = {}  # (file, proc_name) -> proc_info
        for filepath in self.parser.procedures:
            for proc_name, proc_info in self.parser.procedures[filepath].items():
                all_procedures[(filepath, proc_name)] = proc_info

        # Сравнить все пары
        procedures_list = list(all_procedures.items())

        for i in range(len(procedures_list)):
            for j in range(i+1, len(procedures_list)):
                (file1, name1), proc1 = procedures_list[i]
                (file2, name2), proc2 = procedures_list[j]

                # Пропустить если из одного файла
                if file1 == file2:
                    continue

                # Вычислить сходство
                similarity = self.compute_similarity(proc1['tokens'], proc2['tokens'])

                if similarity >= self.similarity_threshold:
                    results.append({
                        'file1': file1,
                        'proc1': name1,
                        'file2': file2,
                        'proc2': name2,
                        'similarity': similarity,
                        'tokens1': len(proc1['tokens']),
                        'tokens2': len(proc2['tokens']),
                        'signature1': proc1['signature'],
                        'signature2': proc2['signature'],
                    })

        return sorted(results, key=lambda x: x['similarity'], reverse=True)

    def report(self, output_file: str = None):
        """Выводит отчёт о совпадениях."""
        matches = self.find_matching_procedures()

        output = {
            'detector': 'C (procedure-level)',
            'similarity_threshold': self.similarity_threshold,
            'files_count': len(self.files),
            'matches': matches,
        }

        if output_file:
            with open(output_file, 'w') as f:
                json.dump(output, f, indent=2)
        else:
            print(json.dumps(output, indent=2))

        return output


def main():
    """Пример использования."""
    if len(sys.argv) < 3:
        print("Использование: detector_c.py <dn_source_dir> <tv_source_dir> [--output report.json] [--threshold THRESHOLD]")
        sys.exit(1)

    dn_dir = Path(sys.argv[1])
    tv_dir = Path(sys.argv[2])
    output_file = None
    threshold = 0.7

    if '--output' in sys.argv:
        idx = sys.argv.index('--output')
        if idx + 1 < len(sys.argv):
            output_file = sys.argv[idx + 1]

    if '--threshold' in sys.argv:
        idx = sys.argv.index('--threshold')
        if idx + 1 < len(sys.argv):
            threshold = float(sys.argv[idx + 1])

    detector = ProcedureDetector(similarity_threshold=threshold)

    # Добавить файлы из DN
    for pas_file in sorted(dn_dir.glob('**/*.pas')):
        with open(pas_file, 'r', encoding='cp1252', errors='replace') as f:
            text = f.read()
            detector.add_file(f"DN:{pas_file.name}", text)

    # Добавить файлы из TV
    for pas_file in sorted(tv_dir.glob('**/*.pas')):
        with open(pas_file, 'r', encoding='cp1252', errors='replace') as f:
            text = f.read()
            detector.add_file(f"TV:{pas_file.name}", text)

    # Найти и вывести совпадения
    detector.report(output_file)


if __name__ == '__main__':
    main()
