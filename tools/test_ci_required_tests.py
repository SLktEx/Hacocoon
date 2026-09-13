#!/usr/bin/env python3
import unittest
from ci_required_tests import verify


class RequiredTests(unittest.TestCase):
    def test_require_actual_execution(self):
        for results in ({}, {"TestOther": ["PASS"]}, {"TestContract": ["SKIP"]},
                        {"TestContract": ["FAIL", "PASS"]}, {"TestContract": ["PASS", "PASS"]}):
            self.assertFalse(verify(["TestContract"], results, 0))
        self.assertFalse(verify(["TestContract"], {"TestContract": ["PASS"]}, 1))
        self.assertFalse(verify([], {}, 0))
        self.assertTrue(verify(["TestContract"], {"TestContract": ["PASS"]}, 0))


if __name__ == "__main__":
    unittest.main()
