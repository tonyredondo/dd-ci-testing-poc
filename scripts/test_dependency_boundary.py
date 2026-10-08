import json
import unittest

from dependency_boundary import MODULE, packages_from_json, violations


class DependencyBoundaryTests(unittest.TestCase):
    def test_accepts_standard_library_and_own_packages(self):
        module = {"Module": {"Path": MODULE}, "Require": None}
        packages = [{"Standard": True, "ImportPath": "sync"},
                    {"ImportPath": MODULE + "/testopt", "Module": {"Path": MODULE}}]
        self.assertEqual(violations(module, packages), [])

    def test_rejects_unused_requirements_and_external_packages(self):
        module = {"Module": {"Path": MODULE}, "Require": [{"Path": "example.com/test-only"}]}
        packages = [{"ImportPath": "example.com/runtime", "Module": {"Path": "example.com/runtime"}}]
        errors = violations(module, packages)
        self.assertEqual(len(errors), 2)
        self.assertIn("test-only", errors[0])
        self.assertIn("example.com/runtime", errors[1])

    def test_rejects_wrong_root_and_missing_module_identity(self):
        self.assertEqual(len(violations({"Module": {"Path": "wrong"}}, [{"ImportPath": MODULE + "/testopt"}])), 2)

    def test_reads_concatenated_go_list_json(self):
        rows = [{"Standard": True}, {"ImportPath": MODULE + "/testopt"}]
        self.assertEqual(list(packages_from_json("\n".join(map(json.dumps, rows)))), rows)


if __name__ == "__main__":
    unittest.main()
