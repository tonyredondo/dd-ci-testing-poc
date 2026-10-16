import unittest

import minitracer_benchmark as benchmark


class ParseTests(unittest.TestCase):
    def test_benchmark_lines(self):
        output = """goos: linux
BenchmarkTestEventLifecycle 	  248616	      4595 ns/op	    2336 B/op	      16 allocs/op
BenchmarkSpanMetadata/ci/tags=0-4  	 1409478	       848.4 ns/op	    1344 B/op	      10 allocs/op
PASS
"""
        self.assertEqual({"BenchmarkTestEventLifecycle": (4595.0, 2336.0, 16.0),
                          "BenchmarkSpanMetadata/ci/tags=0": (848.4, 1344.0, 10.0)},
                         benchmark.parse_benchmarks(output))

    def test_fixture_source(self):
        source = benchmark.fixture_source(2)
        self.assertIn("func TestEmpty00000(t *testing.T) {}\n", source)
        self.assertIn("func TestEmpty00001(t *testing.T) {}\n", source)
        self.assertEqual(2, source.count("func Test"))


class ReportTests(unittest.TestCase):
    def test_tables(self):
        run = lambda wall, cpu: {"wall": wall, "cpu": cpu, "maxrss_kb": 2048, "cycle_requests": 3, "cycle_bytes": 1 << 20,
                                 "events": {"test": 4, "test_suite_end": 1}}
        results = {
            "base": {"commit": "b" * 40, "local_diff_sha256": ""},
            "candidate": {"commit": "c" * 40, "local_diff_sha256": "d" * 64},
            "go": "go version go1.27.0 linux/amd64", "cpu": "CPU", "os": "Linux 7", "reps": 2,
            "tests": 2, "count": 2, "gomaxprocs": 4, "benchtime": "1s",
            "microbenchmarks": {"base": {"BenchmarkTestEventLifecycle": [(200, 30, 3), (200, 30, 3)]},
                                "candidate": {"BenchmarkTestEventLifecycle": [(150, 20, 2), (150, 20, 2)]}},
            "test_binaries": {"native": [run(1.0, 1.0)], "base": [run(1.000004, 1.000008)], "candidate": [run(1.000002, 1.000004)]},
        }
        text = benchmark.report(results)
        self.assertIn("| `BenchmarkTestEventLifecycle` | 200 | 150 | -25.0% | 30 | 20 | 3 | 2 |", text)
        self.assertIn("| ddto, base | 1.000 s | 1.000 s | 2 MiB | 1.00 µs | 2.00 µs | 3, 1.0 MiB |", text)
        self.assertIn("| ddto, candidate | 1.000 s | 1.000 s | 2 MiB | 0.50 µs | 1.00 µs | 3, 1.0 MiB |", text)
        self.assertIn("candidate with local changes (sha256 dddddddddddd)", text)
        self.assertIn("The first base run delivered 4 `test`, 1 `test_suite_end` events.", text)


if __name__ == "__main__":
    unittest.main()
