import unittest

import preparation_benchmark as benchmark

LOG = """TestOptimization.build v0.1.1-dev DEBUG: +0s command="test"
TestOptimization.build v0.1.1-dev DEBUG: +44.9ms runtime own_source_packages=51
TestOptimization.build v0.1.1-dev DEBUG: +74.951ms prepare finished duration=74.731ms status=ok
something else on stderr
TestOptimization.build v0.1.1-dev DEBUG: +179.7ms go test finished duration=104.8ms status=ok
TestOptimization.build v0.1.1-dev DEBUG: +179.781ms ddto finished duration=0.18s status=ok
"""


class PreparationBenchmarkTests(unittest.TestCase):
    def test_parse_reads_durations_and_own_packages(self):
        self.assertEqual({'own_packages': 51, 'prepare': 74.731, 'go_test': 104.8, 'ddto': 180.0},
                         benchmark.parse(LOG))

    def test_parse_ignores_other_lines(self):
        self.assertEqual({}, benchmark.parse('prepare finished duration=1ms\n'))

    def test_report_gives_medians(self):
        samples = [{'wall': w, 'prepare': p, 'go_test': 100.0, 'ddto': 180.0, 'own_packages': 51}
                   for w, p in ((190.0, 70.0), (200.0, 80.0), (210.0, 75.0))]
        results = {'subject': '/x/gin', 'subject_commit': '73726dc606790000', 'go': 'go version go1.27.1',
                   'cpus': 4, 'runs': 3, 'samples': {'release': samples}}
        self.assertIn('| release | 200.0 ms | 75.00 ms | 100.00 ms | 180.00 ms | 51 |', benchmark.report(results))


if __name__ == '__main__':
    unittest.main()
