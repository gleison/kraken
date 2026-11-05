from kraken.domain import PLAN_REVIEW_CRITERIA
from dataclasses import replace
from hashlib import sha256
import json
from pathlib import Path
import tempfile

from kraken.domain import Check, Criterion, Limits, Plan, Task
from kraken.provider import Reply
from kraken.git import git
from kraken.session import initialize

IMAGE = 'busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec'


def plan(task_id='TASK-1', output='result.txt', dependencies=()):
    task = Task(task_id, 'Write approved output', 'Produce hello exactly', 'backend', (output,),
                (Criterion('C1', 'Output equals the approved fixture'),),
                (Check('CHECK-1', ('sh', '-c', f'cmp {output} expected.txt'), ('C1',)),),
                dependencies)
    return Plan('Write deterministic output', 'Only the requested file', 'No network, push or deploy',
                ('Use the existing project and preserve other files',), (task,), IMAGE,
                Limits(counter_only=True), fixtures={'expected.txt': sha256(b'hello\n').hexdigest()})


async def project(root):
    await git(root, 'init', '-q')
    await git(root, 'config', 'user.name', 'Fixture')
    await git(root, 'config', 'user.email', 'fixture@example.invalid')
    (root / 'expected.txt').write_text('hello\n')
    await git(root, 'add', '--', 'expected.txt')
    await git(root, 'commit', '-qm', 'fixture')
    initialize(root)
    (root / '.kraken/project.toml').write_text('[provider]\nmodel="fixture"\napi_key_env=""\n')


def approve(session, p=None):
    p = p or plan()
    session.save_plan(p)
    for role in ('qa', 'reviewer'):
        session.store.review(p, session.config.revision, role,
                             {'passed': True, 'feedback': 'Fixture reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
    session.approve(p.revision, session.config.revision)
    return p


class FakeModel:
    def __init__(self, real_client, *, qa_failures=0, delay=0, outputs=None):
        self.budget, self.store = real_client.budget, real_client.store
        self.qa_failures, self.delay = qa_failures, delay
        self.outputs = outputs or {}
        self.probed = []
        self.calls = []

    async def probe(self, role):
        self.budget.reserve()
        self.probed.append(role.name)

    async def complete(self, role, messages, tools=(), tool_choice=None):
        import asyncio
        self.budget.reserve()
        self.calls.append((role.name, messages))
        if self.delay:
            await asyncio.sleep(self.delay)
        if role.name == 'orchestrator':
            return Reply(json.dumps({'action': 'execute', 'feedforward': 'Produce approved hello output',
                                     'feedback': '', 'reason': 'Bounded fixture task'}), [], None)
        if role.name in {'backend', 'frontend'}:
            if messages[-1]['role'] == 'tool':
                return Reply(json.dumps({'status': 'implemented', 'summary': 'Wrote approved output',
                                         'changes': [], 'risks': [], 'blockers': []}), [], None)
            contract = json.loads(messages[1]['content'].split('\nGuidance')[0])['contract']
            output = contract['allowed_paths'][0]
            return Reply('', [{'id': 'call_1', 'type': 'function', 'function': {
                'name': 'run_command', 'arguments': json.dumps({'argv': ['sh', '-c', f'printf "{self.outputs.get(contract["id"], "hello")}\\n" > {output}']})}}], None)
        if role.name in {'qa', 'reviewer'}:
            passed = not (role.name == 'qa' and self.qa_failures > 0)
            if not passed:
                self.qa_failures -= 1
            return Reply(json.dumps({'passed': passed, 'feedback': 'Fixture independently checked' if passed
                       else 'C1: injected semantic failure; inspect approved output', 'criteria': ['C1']}), [], None)
        raise AssertionError('Unexpected role')
