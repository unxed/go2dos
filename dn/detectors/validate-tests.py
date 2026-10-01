#!/usr/bin/env python3
"""
Validate detector test results against expected outcomes.

Checks that detectors:
1. Find positive examples (copies) with sufficient similarity
2. Don't flag negative examples (independent code) above threshold
"""

import json
import sys
from pathlib import Path
from typing import Dict, List, Tuple


def load_json(filepath: str) -> Dict:
    """Load JSON file."""
    with open(filepath, 'r') as f:
        return json.load(f)


def validate_detector_results(
    detector_output: Dict,
    expected: Dict,
    detector_name: str,
) -> Tuple[bool, List[str]]:
    """
    Validate detector output against expected results.

    Returns: (passed: bool, messages: List[str])
    """
    messages = []
    all_passed = True

    detector_type = detector_name.split('_')[0].lower()
    expected_section = f"{detector_type.upper()}_" + ("_".join(detector_name.split('_')[1:]))

    if expected_section not in expected['expected_detector_results']:
        messages.append(f"⚠️  No expected results for detector {detector_type}")
        return True, messages

    expected_results = expected['expected_detector_results'][expected_section]
    matches = detector_output.get('matches', [])

    # Check positive examples
    if 'positive_must_detect' in expected_results:
        for positive_case in expected_results['positive_must_detect']:
            file1 = positive_case.get('file1')
            file2 = positive_case.get('file2')
            min_sim = positive_case.get('min_similarity', 0.5)

            found = False
            for match in matches:
                if (match['file1'] == file1 and match['file2'] == file2 or
                    match['file1'] == file2 and match['file2'] == file1):

                    if match.get('similarity_score', 0) >= min_sim:
                        messages.append(f"✅ {detector_name}: Found {file1} vs {file2} with similarity {match['similarity_score']:.2f}")
                        found = True
                        break
                    else:
                        messages.append(f"❌ {detector_name}: Found {file1} vs {file2} but similarity {match['similarity_score']:.2f} < {min_sim}")
                        all_passed = False
                        found = True
                        break

            if not found:
                messages.append(f"❌ {detector_name}: MISSED positive example {file1} vs {file2}")
                all_passed = False

    # Check negative examples
    if 'negative_must_not_detect_above' in expected_results:
        for negative_case in expected_results['negative_must_not_detect_above']:
            file1 = negative_case.get('file1')
            file2 = negative_case.get('file2')
            max_sim = negative_case.get('max_similarity', 0.3)

            for match in matches:
                if (match['file1'] == file1 and match['file2'] == file2 or
                    match['file1'] == file2 and match['file2'] == file1):

                    if match.get('similarity_score', 0) > max_sim:
                        messages.append(f"❌ {detector_name}: FALSE POSITIVE {file1} vs {file2} with similarity {match['similarity_score']:.2f} > {max_sim}")
                        all_passed = False
                    else:
                        messages.append(f"✅ {detector_name}: Correctly skipped {file1} vs {file2}")

    # Check positive_may_not_detect cases
    if 'positive_may_not_detect' in expected_results:
        for case in expected_results['positive_may_not_detect']:
            file1 = case.get('file1')
            file2 = case.get('file2')

            found_high = False
            for match in matches:
                if (match['file1'] == file1 and match['file2'] == file2 or
                    match['file1'] == file2 and match['file2'] == file1):
                    if match.get('similarity_score', 0) >= 0.5:
                        found_high = True
                        break

            if not found_high:
                messages.append(f"ℹ️  {detector_name}: Did not detect {file1} vs {file2} (acceptable for this detector)")

    return all_passed, messages


def main():
    """Main validation."""
    if len(sys.argv) < 2:
        print("Usage: validate-tests.py <expected.json> [detector_a.json] [detector_b.json] [detector_c.json]")
        print("  or: validate-tests.py <test_dir>")
        sys.exit(1)

    arg = sys.argv[1]

    # If argument is a directory, find files in it
    if Path(arg).is_dir():
        test_dir = Path(arg)
        expected_file = test_dir / 'expected.json'
        detector_files = {
            'A': test_dir / 'detector_a.json',
            'B': test_dir / 'detector_b.json',
            'C': test_dir / 'detector_c.json',
        }
    else:
        expected_file = Path(arg)
        detector_files = {
            'A': Path(sys.argv[2] if len(sys.argv) > 2 else 'detector_a.json'),
            'B': Path(sys.argv[3] if len(sys.argv) > 3 else 'detector_b.json'),
            'C': Path(sys.argv[4] if len(sys.argv) > 4 else 'detector_c.json'),
        }

    # Load expected results
    try:
        expected = load_json(str(expected_file))
    except Exception as e:
        print(f"❌ Failed to load expected.json: {e}")
        sys.exit(1)

    print("=" * 80)
    print("DETECTOR VALIDATION")
    print("=" * 80)

    all_tests_passed = True

    for detector_label, detector_file in detector_files.items():
        print(f"\nDetector {detector_label}:")
        print("-" * 40)

        if not detector_file.exists():
            print(f"⚠️  {detector_file} not found, skipping")
            continue

        try:
            detector_output = load_json(str(detector_file))
        except Exception as e:
            print(f"❌ Failed to load {detector_file}: {e}")
            all_tests_passed = False
            continue

        passed, messages = validate_detector_results(
            detector_output,
            expected,
            f"detector_{detector_label.lower()}",
        )

        for msg in messages:
            print(f"  {msg}")

        if not passed:
            all_tests_passed = False

    print("\n" + "=" * 80)
    if all_tests_passed:
        print("✅ ALL TESTS PASSED - Detectors are ready for audit")
        sys.exit(0)
    else:
        print("❌ SOME TESTS FAILED - Adjust detector parameters and retry")
        sys.exit(1)


if __name__ == '__main__':
    main()
