#!/usr/bin/env python3
"""
Детектор B: построчное сравнение.

Находит точные совпадения строк между файлами (после нормализации),
затем расширяет совпадения в блоки непрерывных строк.
"""

import sys
import json
from typing import List, Dict, Set, Tuple
from pathlib import Path
from collections import defaultdict

from normalize import normalize_for_comparison


class LineDetector:
    """Детектор совпадений по строкам."""

    def __init__(self):
        """Инициализация детектора."""
        self.files_lines = {}  # file -> list(normalized_line)
        self.line_to_files = defaultdict(list)  # line -> list((file, line_number))

    def add_file(self, filepath: str, text: str):
        """Добавляет файл для анализа."""
        normalized = normalize_for_comparison(text)
        # Разбить по строкам и нормализовать каждую
        lines = [line.strip() for line in normalized.split('\n') if line.strip()]

        self.files_lines[filepath] = lines

        for line_num, line in enumerate(lines):
            self.line_to_files[line].append((filepath, line_num))

    def find_matching_lines(self) -> List[Dict]:
        """
        Находит совпадающие строки между файлами.

        Возвращает список совпадений с номерами строк.
        """
        results = []
        files = list(self.files_lines.keys())

        for i in range(len(files)):
            for j in range(i+1, len(files)):
                file1, file2 = files[i], files[j]
                lines1 = self.files_lines[file1]
                lines2 = self.files_lines[file2]

                # Найти все совпадающие строки
                matches = []
                for line in self.line_to_files:
                    positions1 = [pos for f, pos in self.line_to_files[line] if f == file1]
                    positions2 = [pos for f, pos in self.line_to_files[line] if f == file2]

                    if positions1 and positions2:
                        for pos1 in positions1:
                            for pos2 in positions2:
                                matches.append({
                                    'line': line[:80],  # Первые 80 символов
                                    'line1': pos1,
                                    'line2': pos2,
                                })

                if matches:
                    # Найти блоки непрерывных совпадений
                    blocks = self._find_blocks(matches)

                    similarity = len(matches) / max(len(lines1), len(lines2))

                    results.append({
                        'file1': file1,
                        'file2': file2,
                        'matching_lines': len(matches),
                        'similarity': similarity,
                        'blocks': blocks,
                    })

        return sorted(results, key=lambda x: x['similarity'], reverse=True)

    def _find_blocks(self, matches: List[Dict]) -> List[Dict]:
        """
        Расширяет совпадения в блоки непрерывных строк.

        Группирует совпадения по близости позиций.
        """
        if not matches:
            return []

        # Отсортировать по позициям
        matches_sorted = sorted(matches, key=lambda x: (x['line1'], x['line2']))

        blocks = []
        current_block = {
            'start_line1': matches_sorted[0]['line1'],
            'start_line2': matches_sorted[0]['line2'],
            'length': 1,
            'lines': [matches_sorted[0]['line']],
        }

        for i in range(1, len(matches_sorted)):
            curr = matches_sorted[i]
            prev = matches_sorted[i-1]

            # Проверить, продолжается ли блок
            if (curr['line1'] == prev['line1'] + 1 and
                curr['line2'] == prev['line2'] + 1):
                # Продолжение блока
                current_block['length'] += 1
                current_block['lines'].append(curr['line'])
            else:
                # Новый блок
                if current_block['length'] >= 3:  # Минимум 3 строки в блоке
                    blocks.append(current_block)

                current_block = {
                    'start_line1': curr['line1'],
                    'start_line2': curr['line2'],
                    'length': 1,
                    'lines': [curr['line']],
                }

        if current_block['length'] >= 3:
            blocks.append(current_block)

        return blocks

    def report(self, output_file: str = None):
        """Выводит отчёт о совпадениях."""
        matches = self.find_matching_lines()

        output = {
            'detector': 'B (line-by-line)',
            'files_count': len(self.files_lines),
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
        print("Использование: detector_b.py <dn_source_dir> <tv_source_dir> [--output report.json]")
        sys.exit(1)

    dn_dir = Path(sys.argv[1])
    tv_dir = Path(sys.argv[2])
    output_file = None

    if '--output' in sys.argv:
        idx = sys.argv.index('--output')
        if idx + 1 < len(sys.argv):
            output_file = sys.argv[idx + 1]

    detector = LineDetector()

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
