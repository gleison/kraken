#!/usr/bin/env python3
"""Reproducible local/CI runner. Preserve counts, skips and the actual exit status."""
import argparse
import json
import os
from pathlib import Path
import platform
import sys
import time
import unittest


class Result(unittest.TextTestResult):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.cases = []

    def record(self, test, status, detail=''):
        self.cases.append({'id': test.id(), 'status': status, 'detail': detail})

    def addSuccess(self, test):
        super().addSuccess(test)
        self.record(test, 'passed')

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self.record(test, 'failed', self._exc_info_to_string(err, test))

    def addError(self, test, err):
        super().addError(test, err)
        self.record(test, 'error', self._exc_info_to_string(err, test))

    def addSkip(self, test, reason):
        super().addSkip(test, reason)
        self.record(test, 'skipped', reason)

    def addExpectedFailure(self, test, err):
        super().addExpectedFailure(test, err)
        self.record(test, 'expected_failure')

    def addUnexpectedSuccess(self, test):
        super().addUnexpectedSuccess(test)
        self.record(test, 'unexpected_success')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--report', type=Path, default=Path('test-results.json'))
    parser.add_argument('--require-docker', action='store_true')
    args = parser.parse_args()
    if args.require_docker:
        os.environ['KRAKEN_TEST_DOCKER'] = '1'
    started = time.monotonic()
    suite = unittest.defaultTestLoader.discover('tests')
    result = unittest.TextTestRunner(verbosity=2, resultclass=Result).run(suite)
    counts = {key: sum(c['status'] == key for c in result.cases) for key in
              ('passed', 'failed', 'error', 'skipped', 'expected_failure', 'unexpected_success')}
    report = {'python': platform.python_version(), 'platform': sys.platform,
              'kernel': platform.release(), 'wsl': 'microsoft' in platform.release().lower(),
              'docker_required': args.require_docker, 'duration_seconds': time.monotonic() - started,
              'executed': result.testsRun, 'counts': counts, 'cases': result.cases}
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    return 0 if result.wasSuccessful() and result.testsRun > 0 and not (args.require_docker and result.skipped) else 1


if __name__ == '__main__':
    sys.exit(main())
