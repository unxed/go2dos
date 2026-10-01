#!/usr/bin/env python3
"""
Детектор A: k-граммы токенов с винноингом.

Аналогично MOSS (Moss Similarity Score). Находит совпадения между файлами
по скользящему окну k-грамм, что ловит копии даже с переименованием.
"""

import sys
import json
from typing import List, Dict, Set, Tuple
from pathlib import Path
from collections import defaultdict
import hashlib

from normalize import normalize_for_token_matching, tokenize


class KGramDetector:
    """Детектор совпадений по k-граммам токенов."""

    def __init__(self, k: int = 5, window_threshold: int = 10):
        """
        Инициализация детектора.

        Args:
            k: размер k-граммы (по умолчанию 5)
            window_threshold: минимальная длина общей последовательности в токенах
        """
        self.k = k
        self.window_threshold = window_threshold
        self.file_hashes = {}  # file -> set(hash of k-grams)
        self.hash_to_files = defaultdict(list)  # hash -> list((file, position))
        self.files_data = {}  # file -> normalized_text

    def add_file(self, filepath: str, text: str):
        """Добавляет файл для анализа."""
        self.files_data[filepath] = text

        # Нормализовать текст и токенизировать
        normalized = normalize_for_token_matching(text)
        tokens = normalized.split()

        # Генерировать k-граммы
        kgrams_hashes = set()
        for i in range(len(tokens) - self.k + 1):
            kgram = ' '.join(tokens[i:i+self.k])
            kgram_hash = hashlib.md5(kgram.encode()).hexdigest()
            kgrams_hashes.add(kgram_hash)
            self.hash_to_files[kgram_hash].append((filepath, i))

        self.file_hashes[filepath] = kgrams_hashes

    def find_matches(self) -> List[Dict]:
        """
        Находит все совпадения между файлами.

        Возвращает список словарей с совпадениями:
        [
            {
                "file1": path,
                "file2": path,
                "similarity_score": float (0-1),
                "matches": [
                    {"kgram": "...", "pos1": int, "pos2": int},
                    ...
                ]
            },
            ...
        ]
        """
        results = []
        files = list(self.file_hashes.keys())

        for i in range(len(files)):
            for j in range(i+1, len(files)):
                file1, file2 = files[i], files[j]

                # Найти пересечение k-грамм
                common_hashes = self.file_hashes[file1] & self.file_hashes[file2]

                if not common_hashes:
                    continue

                # Вычислить совпадения с позициями
                matches = []
                text1_tokens = self.files_data[file1].split()
                text2_tokens = self.files_data[file2].split()

                for kgram_hash in common_hashes:
                    positions1 = [pos for f, pos in self.hash_to_files[kgram_hash] if f == file1]
                    positions2 = [pos for f, pos in self.hash_to_files[kgram_hash] if f == file2]

                    for pos1 in positions1:
                        for pos2 in positions2:
                            kgram_text = ' '.join(text1_tokens[pos1:pos1+self.k])
                            matches.append({
                                'kgram': kgram_text,
                                'pos1': pos1,
                                'pos2': pos2,
                            })

                # Вычислить меру сходства
                union_size = len(self.file_hashes[file1] | self.file_hashes[file2])
                similarity = len(common_hashes) / union_size if union_size > 0 else 0

                # Отфильтровать по порогу
                if len(matches) >= self.window_threshold:
                    results.append({
                        'file1': file1,
                        'file2': file2,
                        'similarity_score': similarity,
                        'match_count': len(matches),
                        'matches': matches[:10],  # Первые 10 совпадений
                    })

        return sorted(results, key=lambda x: x['similarity_score'], reverse=True)

    def report(self, output_file: str = None):
        """Выводит отчёт о совпадениях."""
        matches = self.find_matches()

        output = {
            'detector': 'A (k-grams)',
            'k': self.k,
            'window_threshold': self.window_threshold,
            'files_count': len(self.files_data),
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
        print("Использование: detector_a.py <dn_source_dir> <tv_source_dir> [--output report.json] [--k K]")
        sys.exit(1)

    dn_dir = Path(sys.argv[1])
    tv_dir = Path(sys.argv[2])
    k = 5
    output_file = None

    if '--k' in sys.argv:
        idx = sys.argv.index('--k')
        if idx + 1 < len(sys.argv):
            k = int(sys.argv[idx + 1])

    if '--output' in sys.argv:
        idx = sys.argv.index('--output')
        if idx + 1 < len(sys.argv):
            output_file = sys.argv[idx + 1]

    detector = KGramDetector(k=k)

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
