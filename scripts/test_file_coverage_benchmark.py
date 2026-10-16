import unittest

import file_coverage_benchmark as benchmark


class CompareTests(unittest.TestCase):
    def test_metrics_and_missing_files(self):
        reference = {"a_test.go/TestA": ["a.go", "a_test.go"], "b_test.go/TestB": ["b.go", "b_test.go"]}
        rows = {
            "counters": reference,
            "marks": {"a_test.go/TestA": ["a.go", "c.go"], "b_test.go/TestB": ["c.go"]},
        }
        got = benchmark.compare(rows, reference)
        self.assertEqual(2, got["counters"]["tests"])
        self.assertEqual(1, got["counters"]["files_per_test"])
        self.assertEqual(0, got["counters"]["missing_tests"])
        # Each of a.go, b.go and c.go: counters select 1, 1 and 0 tests.
        self.assertEqual(0.7, got["counters"]["tests_per_changed_file"])
        self.assertEqual(1.5, got["marks"]["files_per_test"])
        self.assertEqual({"b_test.go/TestB": ["b.go"]}, got["marks"]["missing"])

    def test_only_common_tests_count(self):
        rows = {"counters": {"T/A": ["a.go"]}, "marks": {"T/A": ["a.go"], "T/B": ["b.go"]}}
        self.assertEqual(1, benchmark.compare(rows, rows["counters"])["marks"]["tests"])


if __name__ == "__main__":
    unittest.main()
