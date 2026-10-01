#!/usr/bin/env python3
"""
Нормализация Pascal-исходников для аудита Turbo Vision / Dos Navigator.

Удаляет комментарии, нормализует пробелы и регистр, токенизирует.
"""

import re
import sys
from enum import Enum
from typing import List, Tuple, Dict, Set


class TokenType(Enum):
    """Тип токена Pascal."""
    KEYWORD = "keyword"
    IDENTIFIER = "identifier"
    NUMBER = "number"
    STRING = "string"
    OPERATOR = "operator"
    PUNCTUATION = "punctuation"
    COMMENT = "comment"
    WHITESPACE = "whitespace"


# Ключевые слова Pascal (базовый набор, может быть расширен)
PASCAL_KEYWORDS = {
    'absolute', 'and', 'array', 'asm', 'begin', 'case', 'const', 'constructor',
    'destructor', 'div', 'do', 'downto', 'else', 'end', 'except', 'exports',
    'external', 'far', 'file', 'for', 'forward', 'function', 'goto', 'if',
    'implementation', 'in', 'inline', 'interface', 'interrupt', 'is', 'label',
    'library', 'mod', 'near', 'nil', 'not', 'object', 'of', 'on', 'or', 'packed',
    'procedure', 'program', 'property', 'raise', 'record', 'repeat', 'resident',
    'set', 'shl', 'shr', 'string', 'then', 'to', 'try', 'type', 'unit', 'until',
    'uses', 'var', 'virtual', 'while', 'with', 'xor',
    # Типы данных
    'boolean', 'byte', 'char', 'integer', 'longint', 'real', 'word',
    'private', 'protected', 'public', 'published',
}


def remove_comments(text: str) -> str:
    """
    Удаляет комментарии из Pascal-кода.

    Поддерживает:
    - Однострочные: // comment
    - Многострочные: { comment } и (* comment *)
    """
    result = []
    i = 0
    while i < len(text):
        # Многострочный комментарий { ... }
        if text[i] == '{':
            while i < len(text) and text[i] != '}':
                i += 1
            if i < len(text):
                i += 1  # пропустить закрывающую '}'
            continue

        # Многострочный комментарий (* ... *)
        if i + 1 < len(text) and text[i:i+2] == '(*':
            i += 2
            while i + 1 < len(text) and text[i:i+2] != '*)':
                i += 1
            if i + 1 < len(text):
                i += 2  # пропустить '*)'
            continue

        # Однострочный комментарий // ...
        if i + 1 < len(text) and text[i:i+2] == '//':
            while i < len(text) and text[i] != '\n':
                i += 1
            if i < len(text):
                result.append('\n')  # сохранить новую строку
                i += 1
            continue

        # Строки в кавычках (не разбираем их внутренность)
        if text[i] == "'":
            result.append("'")
            i += 1
            while i < len(text):
                if text[i] == "'":
                    result.append("'")
                    i += 1
                    if i < len(text) and text[i] == "'":
                        # Экранированная кавычка ''
                        result.append("'")
                        i += 1
                    break
                else:
                    result.append(text[i])
                    i += 1
            continue

        result.append(text[i])
        i += 1

    return ''.join(result)


def tokenize(text: str) -> List[Tuple[str, TokenType]]:
    """
    Разбивает текст на токены.
    Возвращает список кортежей (текст_токена, тип).
    """
    tokens = []
    i = 0

    while i < len(text):
        # Пропустить пробелы (но не переводы строк)
        if text[i] in ' \t\r\n':
            start = i
            while i < len(text) and text[i] in ' \t\r\n':
                i += 1
            # Ненужны пробельные токены для нормализации
            continue

        # Числа
        if text[i].isdigit():
            start = i
            while i < len(text) and (text[i].isdigit() or text[i] in '.eE+-'):
                i += 1
            tokens.append((text[start:i], TokenType.NUMBER))
            continue

        # Строки
        if text[i] == "'":
            start = i
            i += 1
            while i < len(text):
                if text[i] == "'":
                    i += 1
                    if i < len(text) and text[i] == "'":
                        i += 1
                    else:
                        break
                else:
                    i += 1
            tokens.append((text[start:i], TokenType.STRING))
            continue

        # Идентификаторы и ключевые слова
        if text[i].isalpha() or text[i] == '_':
            start = i
            while i < len(text) and (text[i].isalnum() or text[i] == '_'):
                i += 1
            word = text[start:i]
            word_lower = word.lower()

            if word_lower in PASCAL_KEYWORDS:
                tokens.append((word_lower, TokenType.KEYWORD))
            else:
                tokens.append((word, TokenType.IDENTIFIER))
            continue

        # Двусимвольные операторы
        if i + 1 < len(text):
            two_char = text[i:i+2]
            if two_char in (':=', '<=', '>=', '<>', '..', '**'):
                tokens.append((two_char, TokenType.OPERATOR))
                i += 2
                continue

        # Однозначные символы
        char = text[i]
        if char in '+-*/<>=':
            tokens.append((char, TokenType.OPERATOR))
        elif char in '()[],.;:':
            tokens.append((char, TokenType.PUNCTUATION))
        else:
            tokens.append((char, TokenType.OPERATOR))

        i += 1

    return tokens


def normalize_text(text: str, remove_identifiers: bool = False) -> str:
    """
    Нормализует Pascal-текст.

    - Удаляет комментарии
    - Нормализует пробелы
    - Приводит ключевые слова к нижнему регистру
    - Если remove_identifiers=True: заменяет идентификаторы на _ID

    Возвращает нормализованный текст.
    """
    # Удалить комментарии
    text = remove_comments(text)

    # Токенизировать
    tokens = tokenize(text)

    # Фильтровать и трансформировать
    result = []
    identifier_map = {}
    identifier_counter = 0

    for token_text, token_type in tokens:
        if token_type == TokenType.KEYWORD:
            result.append(token_text.lower())
        elif token_type == TokenType.IDENTIFIER:
            if remove_identifiers:
                if token_text not in identifier_map:
                    identifier_map[token_text] = f"_ID{identifier_counter}"
                    identifier_counter += 1
                result.append(identifier_map[token_text])
            else:
                result.append(token_text)
        elif token_type == TokenType.STRING:
            # Строки оставляем как есть (они могут быть важны)
            result.append(token_text)
        elif token_type == TokenType.COMMENT:
            # Комментарии уже удалены
            pass
        elif token_type == TokenType.WHITESPACE:
            # Пропуск пробельных
            pass
        else:
            # Числа, операторы, пунктуация
            result.append(token_text)

    return ' '.join(result)


def normalize_for_comparison(text: str) -> str:
    """Нормализует текст для построчного сравнения (детектор B)."""
    return normalize_text(text, remove_identifiers=False)


def normalize_for_token_matching(text: str) -> str:
    """Нормализует текст для поиска по токенам (детекторы A и C)."""
    return normalize_text(text, remove_identifiers=True)


if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Использование: normalize.py <input.pas> [--remove-ids] [--output output.txt]")
        sys.exit(1)

    input_file = sys.argv[1]
    remove_ids = '--remove-ids' in sys.argv
    output_file = None

    if '--output' in sys.argv:
        idx = sys.argv.index('--output')
        if idx + 1 < len(sys.argv):
            output_file = sys.argv[idx + 1]

    # Прочитать входной файл
    with open(input_file, 'r', encoding='cp1252', errors='replace') as f:
        text = f.read()

    # Нормализовать
    normalized = normalize_for_token_matching(text) if remove_ids else normalize_for_comparison(text)

    # Вывести результат
    if output_file:
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(normalized)
        print(f"Результат сохранён в {output_file}")
    else:
        print(normalized)
