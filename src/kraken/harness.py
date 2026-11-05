"""Normalize execution outcomes without confusing skip/xfail/disabled with passes."""
from dataclasses import asdict, dataclass
import json
import re
import xml.etree.ElementTree as ET

from .domain import KrakenError, require


@dataclass
class Outcome:
    status: str
    passed: int = 0
    failed: int = 0
    errors: int = 0
    skipped: int = 0
    expected_failures: int = 0
    unexpected_successes: int = 0
    disabled: int = 0
    reason: str = ''
    runner: str = 'command'

    @property
    def ok(self):
        return self.status == 'passed'


def _unittest(text):
    count = re.search(r'Ran\s+(\d+)\s+tests?\b', text)
    require(count, 'Resumo unittest ausente; declare um relatório JUnit para saída não padrão')
    counts = {}
    for label in ('failures', 'errors', 'skipped', 'expected failures', 'unexpected successes'):
        match = re.search(r'\b' + label + r'=(\d+)', text)
        counts[label] = int(match[1]) if match else 0
    total = int(count[1])
    passed = total - sum(counts.values())
    require(passed >= 0 and re.search(r'\bOK\b|\bFAILED\b', text), 'Resumo unittest inconsistente')
    return Outcome('pending', passed, counts['failures'], counts['errors'], counts['skipped'],
                   counts['expected failures'], counts['unexpected successes'], runner='unittest')


def _pytest(text):
    counts = {}
    for name in ('passed', 'failed', 'error', 'errors', 'skipped', 'xfailed', 'xpassed', 'deselected'):
        matches = re.findall(r'\b(\d+)\s+' + name + r'\b', text)
        counts[name] = int(matches[-1]) if matches else 0
    require(any(counts.values()) or 'no tests ran' in text.lower(), 'Resumo pytest ausente/incompleto')
    return Outcome('pending', counts['passed'], counts['failed'], counts['error'] + counts['errors'],
                   counts['skipped'], counts['xfailed'], counts['xpassed'], counts['deselected'], runner='pytest')


def _junit(text):
    require(len(text.encode()) <= 1_000_000 and '<!DOCTYPE' not in text and '<!ENTITY' not in text,
            'Relatório XML inseguro ou excessivo')
    try:
        root = ET.fromstring(text)
    except ET.ParseError as exc:
        raise KrakenError('Relatório JUnit inválido') from exc
    require(root.tag in {'testsuite', 'testsuites'}, 'Raiz JUnit inválida')
    result = Outcome('pending', runner='junit')
    cases = list(root.iter('testcase'))
    for case in cases:
        skipped = case.find('skipped')
        if case.get('status') in {'disabled', 'notrun'}:
            result.disabled += 1
        elif case.find('failure') is not None:
            result.failed += 1
        elif case.find('error') is not None:
            result.errors += 1
        elif skipped is not None:
            if 'xfail' in skipped.get('type', '').lower() or 'expected' in skipped.get('type', '').lower():
                result.expected_failures += 1
            else:
                result.skipped += 1
        else:
            result.passed += 1
    # Some producers summarize disabled tests without testcase elements.
    suites = [s for s in root.iter('testsuite') if not list(s.iter('testsuite'))[1:]]
    declared_disabled = sum(int(s.get('disabled', '0')) for s in suites)
    result.disabled = max(result.disabled, declared_disabled)
    for suite in root.iter('testsuite'):
        declared = suite.get('tests')
        actual = len(list(suite.iter('testcase')))
        require(declared is None or int(declared) >= actual, 'Contagens JUnit inconsistentes')
        for tag, attribute in (('failure', 'failures'), ('error', 'errors')):
            count = sum(case.find(tag) is not None for case in suite.iter('testcase'))
            require(attribute not in suite.attrib or int(suite.get(attribute)) == count, 'Resumo JUnit contradiz casos')
        disabled = max(int(suite.get('disabled', '0')),
                       sum(int(s.get('disabled', '0')) for s in suite.iter('testsuite') if not list(s.iter('testsuite'))[1:]))
        require(disabled >= 0, 'Contagem de disabled inválida')
        if declared is not None and int(declared) > actual + disabled:
            raise KrakenError('JUnit sem resultados individuais suficientes; não inferir aprovação do resumo')
    return result


def normalize(check, result):
    runner = check.runner
    text = result.stdout + '\n' + result.stderr
    try:
        if runner == 'auto':
            joined = ' '.join(check.argv)
            if check.report:
                runner = 'junit'
            elif 'unittest' in joined or re.search(r'Ran\s+\d+\s+tests?\b', text):
                runner = 'unittest'
            elif 'pytest' in joined or 'no tests ran' in text.lower():
                runner = 'pytest'
            else:
                runner = 'command'
        if runner == 'junit':
            require(check.report in result.artifacts, 'Relatório JUnit desta execução está ausente')
            outcome = _junit(result.artifacts[check.report])
        elif runner == 'unittest':
            require(not result.truncated, 'Saída truncada: use relatório para comprovar resultados')
            outcome = _unittest(text)
        elif runner == 'pytest':
            require(not result.truncated, 'Saída truncada: use relatório para comprovar resultados')
            outcome = _pytest(text)
        else:
            outcome = Outcome('pending', passed=int(result.code == 0), runner='command')
        issues = []
        if result.code != 0:
            issues.append(f'exit={result.code}')
        if outcome.failed or outcome.errors or outcome.unexpected_successes:
            issues.append('falhas, erros ou sucesso inesperado')
        if outcome.passed < check.min_passed:
            issues.append('resultados aprovados insuficientes')
        if (outcome.skipped or outcome.disabled) and not check.allow_skipped:
            issues.append('skip/disabled não autorizado no plano')
        if outcome.expected_failures and not check.allow_expected_failures:
            issues.append('xfail não autorizado no plano')
        outcome.status = 'failed' if issues else 'passed'
        outcome.reason = '; '.join(issues) or 'Critérios de execução satisfeitos'
        return outcome
    except (KrakenError, ValueError, TypeError, OverflowError) as exc:
        return Outcome('unverified', reason=str(exc), runner=runner)
