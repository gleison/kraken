from dataclasses import replace
import unittest
from kraken.domain import Check
from kraken.harness import normalize
from kraken.process import Result


class HarnessTests(unittest.TestCase):
    def setUp(self):
        self.check = Check('CHECK', ('python', '-m', 'unittest'), ('C1',))

    def test_unittest_all_skipped_never_passes(self):
        result = Result(0, '', 'Ran 3 tests in 0.001s\nOK (skipped=3)')
        outcome = normalize(self.check, result)
        self.assertFalse(outcome.ok)
        self.assertEqual((outcome.passed, outcome.skipped), (0, 3))
        self.assertFalse(normalize(replace(self.check, allow_skipped=True), result).ok)

    def test_partial_skip_requires_explicit_policy(self):
        result = Result(0, '', 'Ran 3 tests in 0.001s\nOK (skipped=1)')
        self.assertFalse(normalize(self.check, result).ok)
        self.assertTrue(normalize(replace(self.check, allow_skipped=True), result).ok)

    def test_pytest_differentiates_xfail_and_disabled(self):
        check = replace(self.check, argv=('pytest',), runner='pytest')
        result = Result(0, '2 passed, 1 skipped, 1 xfailed, 3 deselected in 0.2s', '')
        out = normalize(check, result)
        self.assertEqual((out.passed, out.skipped, out.expected_failures, out.disabled), (2, 1, 1, 3))
        self.assertFalse(out.ok)
        self.assertTrue(normalize(replace(check, allow_skipped=True, allow_expected_failures=True), result).ok)

    def test_truncated_runner_output_is_unverified(self):
        self.assertEqual(normalize(self.check, Result(0, 'Ran 2 tests\nOK', '', True)).status, 'unverified')

    def test_junit_artifact_and_exit_status_both_matter(self):
        check = replace(self.check, runner='junit', report='/tmp/result.xml')
        xml = '<testsuite tests="3" disabled="1"><testcase name="ok"/><testcase name="skip"><skipped/></testcase></testsuite>'
        result = Result(0, '', '', artifacts={check.report: xml})
        out = normalize(check, result)
        self.assertEqual((out.passed, out.skipped, out.disabled), (1, 1, 1))
        self.assertFalse(out.ok)
        self.assertTrue(normalize(replace(check, allow_skipped=True), result).ok)
        self.assertFalse(normalize(replace(check, allow_skipped=True), replace(result, code=1)).ok)

    def test_missing_or_summary_only_or_entity_report_is_not_passed(self):
        check = replace(self.check, runner='junit', report='/tmp/result.xml')
        self.assertEqual(normalize(check, Result(0, '', '')).status, 'unverified')
        for xml in ('<testsuite tests="100"/>', '<!DOCTYPE x><testsuite/>', 'broken'):
            self.assertEqual(normalize(check, Result(0, '', '', artifacts={check.report: xml})).status, 'unverified')

    def test_functional_command_does_not_need_test_count(self):
        check = replace(self.check, argv=('cmp', 'actual', 'expected'))
        self.assertTrue(normalize(check, Result(0, '', '')).ok)
        self.assertFalse(normalize(check, Result(7, '', '')).ok)

    def test_junit_expected_failure_and_disabled_are_separate(self):
        check = replace(self.check, runner='junit', report='/tmp/results.xml',
                        allow_skipped=True, allow_expected_failures=True)
        xml = '<testsuite tests="3" disabled="1"><testcase/><testcase><skipped type="pytest.xfail"/></testcase><testcase status="disabled"><skipped/></testcase></testsuite>'
        out = normalize(check, Result(0, '', '', artifacts={check.report: xml}))
        self.assertEqual((out.passed, out.expected_failures, out.disabled, out.skipped), (1, 1, 1, 0))
        self.assertTrue(out.ok)

    def test_junit_summary_cannot_hide_reported_failures(self):
        check = replace(self.check, runner='junit', report='/tmp/results.xml')
        xml = '<testsuite tests="1" failures="1"><testcase/></testsuite>'
        self.assertEqual(normalize(check, Result(0, '', '', artifacts={check.report: xml})).status, 'unverified')
