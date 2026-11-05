"""Deterministic scheduling, approval gates, evidence and serialized integration."""
import asyncio
import json
import re
from dataclasses import asdict
import time

from .agent import agent, json_object, verdict, verdict_instruction
from .config import Config
from .domain import DecisionRequired, KrakenError, canonical, require
from .git import Repository, git
from .tools import DockerExecutor, Tools


class Orchestrator:
    def __init__(self, root, store, config, plan, client, executor=None):
        self.root, self.store, self.config = root, store, config
        self.plan, self.client = plan, client
        self.client.budget.limits = plan.limits
        self.repo = Repository(root, store)
        self.executor = executor or DockerExecutor(store, client.budget, plan.image,
                                                  docker=('docker', '--context', config.docker_context), policy=self.gate)
        self.integration_lock = asyncio.Lock()
        self.active = set()

    def gate(self):
        current = Config(self.root)
        self.store.assert_approved(self.plan, current.revision)
        require(current.revision == self.config.revision, 'Configuração/skills mudaram')
        require(self.plan.revision == self.store.get('approved')['revision'], 'Plano mudou')
        path = self.root / '.kraken/plan.json'
        require(path.is_file(), 'Plano persistente ausente')
        from .domain import Plan
        require(Plan.from_dict(json.loads(path.read_text())).revision == self.plan.revision,
                'Arquivo do plano mudou; nova revisão/aprovação necessária')
        self.client.budget.check()

    async def run(self):
        require(not self.active, 'Já existe execução ativa')
        self.gate()
        self.repo.fixtures(self.root, self.plan)
        await self.executor.preflight()
        await self.repo.prepare()
        # Protocol checks happen before any implementation, and consume approved quota.
        profiles = set()
        for name in {t.role for t in self.plan.tasks} | {'orchestrator', 'qa', 'reviewer'}:
            role = self.config.role(name)
            key = canonical(asdict(role.profile))
            if key not in profiles:
                await self.client.probe(role)
                profiles.add(key)
        self.store.set('control', 'running')
        self.store.emit('run_started', revision=self.plan.revision)
        try:
            while self.store.get('control') == 'running':
                self.gate()
                rows = {r['id']: r for r in self.store.tasks()}
                ready = [t for t in self.plan.tasks if rows[t.id]['status'] == 'ready'
                         and all(rows[d]['status'] == 'done' for d in t.depends_on)]
                capacity = self.plan.limits.concurrency - len(self.active)
                for task in ready[:capacity]:
                    self.gate()
                    attempt = self.store.claim(task.id)
                    future = asyncio.create_task(self.execute(task, attempt))
                    self.active.add(future)
                if not self.active:
                    break
                done, _ = await asyncio.wait(self.active, return_when=asyncio.FIRST_COMPLETED)
                self.active.difference_update(done)
                for future in done:
                    if not future.cancelled():
                        future.result()
            if self.store.get('control') == 'pausing':
                await asyncio.gather(*self.active)
                self.active.clear()
                self.store.set('control', 'paused')
            elif self.store.get('control') == 'cancelling':
                for future in self.active:
                    future.cancel()
                await asyncio.gather(*self.active, return_exceptions=True)
                self.active.clear()
                self.store.set('control', 'cancelled')
            else:
                statuses = [r['status'] for r in self.store.tasks()]
                self.store.set('control', 'done' if all(s == 'done' for s in statuses) else 'blocked')
        except BaseException:
            for future in self.active:
                future.cancel()
            await asyncio.gather(*self.active, return_exceptions=True)
            self.active.clear()
            self.store.set('control', 'blocked')
            raise
        finally:
            self.store.emit('run_stopped', status=self.store.get('control'))

    def pause(self):
        if self.store.get('control') == 'running':
            self.store.set('control', 'pausing')
            self.store.emit('pause_requested')

    def cancel(self):
        if self.store.get('control') in {'running', 'pausing'}:
            self.store.set('control', 'cancelling')
            for future in self.active:
                future.cancel()
            self.store.emit('cancel_requested')

    async def check(self, workspace, task, attempt, check):
        version = await git(workspace, 'rev-parse', 'HEAD')
        result = await self.executor.run(workspace, check.argv, readonly=True, fixtures=self.plan.fixtures)
        no_tests = bool(re.search(r'Ran 0 tests|no tests ran', result.stdout + result.stderr, re.I))
        passed = result.code == 0 and not no_tests
        data = {'check': asdict(check), 'result': asdict(result), 'zero_tests_detected': no_tests}
        self.store.evidence(task.id, attempt, version, 'computational', passed, data)
        require(passed, f'Check {task.id}/{check.id} não validou o critério (exit={result.code}, zero_tests={no_tests}): {result.stderr[:500]}')
        return data

    async def evaluate(self, workspace, task, attempt, diff_base, *, integrated=False):
        evidence = []
        targets = [(task, attempt)]
        if integrated:
            rows = {r['id']: r for r in self.store.tasks()}
            targets += [(prior, rows[prior.id]['attempt']) for prior in self.plan.tasks
                        if prior.id != task.id and rows[prior.id]['status'] == 'done']
        for target, target_attempt in targets:
            for check in target.checks:
                if check.timing == 'before_integration' or (integrated and check.timing == 'after_integration'):
                    self.gate()
                    evidence.append(await self.check(workspace, target, target_attempt, check))
        self.repo.fixtures(workspace, self.plan)
        diff = await git(workspace, 'diff', '--no-ext-diff', '--no-textconv', diff_base, 'HEAD')
        require(len(diff) <= 50_000, 'Diff grande demais para revisão; decomponha a tarefa')
        version = await git(workspace, 'rev-parse', 'HEAD')
        for name in ('qa', 'reviewer'):
            self.gate()
            criteria = [c.id for c in task.criteria]
            prompt = canonical({'contract': asdict(task), 'version': version, 'diff': diff,
                                'evidence': evidence, 'integrated': integrated,
                                'decisions': self.plan.decisions}) + '\n' + verdict_instruction(criteria)
            tools = Tools(self.executor, workspace, task, readonly=True, fixtures=self.plan.fixtures)
            result = verdict(await agent(self.client, self.config.role(name), prompt, tools), criteria)
            self.store.evidence(task.id, attempt, version, name, result['passed'], result)
            require(result['passed'], f'{name}: {result["feedback"]}')

    async def execute(self, task, attempt):
        workspace = None
        publication_started = False
        try:
            async with asyncio.timeout(self.plan.limits.attempt_seconds):
                self.gate()
                workspace, base = await self.repo.workspace(task, attempt)
                prior = next(r for r in self.store.tasks() if r['id'] == task.id)
                dependencies = [r for r in self.store.tasks() if r['id'] in task.depends_on]
                prompt = canonical({'contract': asdict(task), 'plan_revision': self.plan.revision,
                                    'project': self.root.name, 'session': self.store.get('session'),
                                    'attempt': attempt, 'base_commit': base, 'dependency_results': dependencies,
                                    'environment': {'image': self.plan.image, 'network': 'none'},
                                    'decisions': self.plan.decisions, 'glossary': self.plan.glossary,
                                    'exclusions': self.plan.exclusions, 'feedback': prior['detail'],
                                    'limits': asdict(self.plan.limits)})
                guidance = json_object(await agent(
                    self.client, self.config.role('orchestrator'),
                    prompt + '\nReturn only JSON: {"action":"execute" or "escalate", '
                    '"feedforward":"concise task guidance", "feedback":"correction guidance if any", '
                    '"reason":"why this is within scope or which human decision is missing"}. '
                    'Do not change the approved contract or permissions.',
                    Tools(self.executor, workspace, task, readonly=True, fixtures=self.plan.fixtures)))
                require(guidance.get('action') in {'execute', 'escalate'} and
                        all(isinstance(guidance.get(k), str) for k in ('feedforward', 'feedback', 'reason')),
                        'Orientação do Orquestrador inválida')
                self.store.emit('handoff', task=task.id, attempt=attempt, guidance=guidance)
                if guidance['action'] == 'escalate':
                    raise DecisionRequired(guidance['reason'])
                prompt += '\nGuidance (does not change contract): ' + canonical(guidance)
                tools = Tools(self.executor, workspace, task, fixtures=self.plan.fixtures)
                prompt += ('\nReturn only JSON: {"status":"implemented" or "blocked", '
                           '"summary":"what changed and why", "changes":["relative paths"], '
                           '"risks":["remaining risks"], "blockers":["missing decisions"]}. '
                           'This report does not approve the task; the runtime runs independent checks.')
                report = json_object(await agent(self.client, self.config.role(task.role), prompt, tools, write=True))
                require(report.get('status') in {'implemented', 'blocked'}
                        and isinstance(report.get('summary'), str) and report['summary'].strip()
                        and all(isinstance(report.get(k), list) and all(isinstance(v, str) for v in report[k])
                                for k in ('changes', 'risks', 'blockers')), 'Relatório de implementação inválido')
                self.store.emit('task_report', task=task.id, attempt=attempt, report=report)
                if report['status'] == 'blocked' or report['blockers']:
                    raise DecisionRequired('; '.join(report['blockers']) or report['summary'])
                self.gate()
                commit = await self.repo.commit_task(workspace, task, self.plan)
                self.store.task(task.id, 'validating', candidate=commit)
                await self.evaluate(workspace, task, attempt, base)
                async with self.integration_lock:
                    self.gate()
                    self.store.task(task.id, 'integrating', candidate=commit)
                    candidate, current = await self.repo.candidate(task, attempt, workspace, commit, base)
                    # Re-evaluate both fast checks and reviewers against the integrated candidate.
                    await self.evaluate(candidate, task, attempt, current, integrated=True)
                    self.gate()
                    publication_started = True
                    self.store.set(f'publication:{self.plan.revision}:{task.id}:{attempt}',
                                   {'status': 'started', 'candidate': str(candidate), 'base': current})
                    integrated = await self.repo.publish_local(candidate, current)
                    self.store.set(f'publication:{self.plan.revision}:{task.id}:{attempt}',
                                   {'status': 'completed', 'commit': integrated})
                    self.store.task(task.id, 'done', 'Integrada e validada', integrated)
        except asyncio.CancelledError:
            self.store.task(task.id, 'blocked', 'Cancelada; reconciliar efeitos antes de repetir')
            raise
        except (KrakenError, TimeoutError, OSError) as exc:
            detail = str(exc) or 'Tempo de tentativa esgotado'
            # A conflicted/incomplete integration is preserved; do not retry uncertain effects.
            blocked = (isinstance(exc, DecisionRequired) or attempt >= self.plan.limits.attempts or publication_started
                       or self.store.get('control') != 'running')
            self.store.task(task.id, 'blocked' if blocked else 'ready', detail)
            self.store.emit('correction_feedback', task=task.id, attempt=attempt,
                            feedback=detail, harness_suggestion='Revisar controles se esta falha se repetir')
