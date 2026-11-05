"""Deterministic scheduling, approval gates, evidence and serialized integration."""
import asyncio
import json
from pathlib import Path
import re
from dataclasses import asdict
import time

from .agent import agent, json_object, verdict, verdict_instruction
from .config import Config
from .domain import DecisionRequired, KrakenError, VerificationFailed, canonical, require
from .git import Repository, git
from .tools import DockerExecutor, Tools
from .harness import normalize


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

    def gate(self, allow_recovery=False):
        current = Config(self.root)
        self.store.assert_approved(self.plan, current.revision)
        require(current.revision == self.config.revision, 'Configuração/skills mudaram')
        require(self.plan.revision == self.store.get('approved')['revision'], 'Plano mudou')
        path = self.root / '.kraken/plan.json'
        require(path.is_file(), 'Plano persistente ausente')
        from .domain import Plan
        require(Plan.from_dict(json.loads(path.read_text())).revision == self.plan.revision,
                'Arquivo do plano mudou; nova revisão/aprovação necessária')
        if not allow_recovery:
            require(self.store.get('control') != 'recovery_required', 'Recuperação Git precisa terminar antes de continuar')
            self.client.budget.check()

    async def run(self):
        require(not self.active, 'Já existe execução ativa')
        self.gate(allow_recovery=True)
        self.repo.fixtures(self.root, self.plan)
        await self.executor.preflight()
        await self.executor.cleanup_orphans()
        await self.repo.prepare()
        require(set(self.store.pending_publications()) == set(self.store.pending_publications(self.plan.revision)),
                'Há transferência de outra revisão/versão; reconciliar antes de iniciar novo trabalho')
        async def revalidate(*args, **kwargs):
            await self.check_requirements({'qa', 'reviewer'}, args[0])
            checked = set()
            for name in ('qa', 'reviewer'):
                role = self.config.role(name)
                signature = canonical(asdict(role.profile))
                if signature not in checked:
                    await self.client.probe(role)
                    checked.add(signature)
            return await self.evaluate(*args, **kwargs)
        await self.repo.reconcile(self.plan, revalidate=revalidate)
        for task_id in self.store.get('interrupted', []):
            row = next((r for r in self.store.tasks() if r['id'] == task_id), None)
            if row and row['status'] == 'blocked' and row['attempt'] < self.plan.limits.attempts:
                require(not self.store.get(f'publication:{self.plan.revision}:{task_id}:{row["attempt"]}'),
                        'Journal ainda pendente de reconciliação')
                self.store.task(task_id, 'ready', 'Tentativa isolada interrompida; nova tentativa segura')
        self.store.set('interrupted', [])
        if all(row['status'] == 'done' for row in self.store.tasks()):
            self.store.set('control', 'done')
            self.store.emit('run_recovered', status='done')
            return
        self.gate()
        # Protocol checks happen before any implementation, and consume approved quota.
        profiles = set()
        for name in {t.role for t in self.plan.tasks} | {'orchestrator', 'qa', 'reviewer'}:
            role = self.config.role(name)
            key = canonical(asdict(role.profile))
            if key not in profiles:
                await self.client.probe(role)
                profiles.add(key)
        await self.check_requirements({t.role for t in self.plan.tasks} | {'orchestrator', 'qa', 'reviewer'}, self.repo.integration)
        self.store.set('control', 'running')
        self.store.emit('run_started', revision=self.plan.revision)
        try:
            await self.restore_reuse()
            while self.store.get('control') == 'running':
                self.gate()
                rows = {r['id']: r for r in self.store.tasks()}
                ready = [t for t in self.plan.tasks if rows[t.id]['status'] == 'ready'
                         and all(rows[d]['status'] == 'done' for d in t.depends_on)]
                capacity = self.plan.limits.concurrency - len(self.active)
                occupied = [t for t in self.plan.tasks if rows[t.id]['status'] in {'running', 'validating', 'integrating'}]
                for task in ready:
                    if capacity <= 0:
                        break
                    if any(task.overlaps(other) for other in occupied):
                        continue
                    self.gate()
                    attempt = self.store.claim(task.id)
                    future = asyncio.create_task(self.execute(task, attempt))
                    self.active.add(future)
                    occupied.append(task)
                    capacity -= 1
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
            elif self.store.get('control') == 'recovery_required':
                uncertain = [r['id'] for r in self.store.tasks() if r['status'] in {'running', 'validating', 'integrating'}]
                self.store.set('interrupted', sorted(set(self.store.get('interrupted', [])) | set(uncertain)))
                for future in self.active:
                    future.cancel()
                await asyncio.gather(*self.active, return_exceptions=True)
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

    async def check_requirements(self, names, workspace):
        self.config.validate_environment(names)
        for name in sorted(names):
            commands = self.config.role(name).requirements.get('required_commands', [])
            if commands:
                checked = await self.executor.run(workspace,
                    ['sh', '-c', 'for executable do command -v "$executable" >/dev/null || exit 127; done',
                     'kraken-requirements', *commands], readonly=True)
                require(checked.code == 0, f'Executável exigido pela skill do papel {name} está ausente')

    async def restore_reuse(self):
        carry = self.store.get('reuse', {})
        pending = [t for t in self.plan.tasks if any(r['id'] == t.id and r['status'] == 'reuse_pending'
                                                    for r in self.store.tasks())]
        while pending:
            progress = False
            for task in list(pending):
                rows = {r['id']: r for r in self.store.tasks()}
                if any(rows[d]['status'] == 'reuse_pending' for d in task.depends_on):
                    continue
                progress = True
                pending.remove(task)
                artifact = carry.get(task.id)
                if not artifact or any(rows[d]['status'] != 'done' for d in task.depends_on):
                    self.store.task(task.id, 'ready', 'Dependência mudou; resultado anterior não reutilizado')
                    continue
                publication_started = False
                try:
                    self.gate()
                    source = Path(artifact['source'])
                    require(source.resolve().is_relative_to(self.store.directory.resolve()), 'Origem de reuse fora do runtime')
                    candidate, current = await self.repo.candidate(task, 0, source, artifact['commit'], artifact['base'])
                    self.store.task(task.id, 'validating', 'Revalidando resultado da revisão anterior')
                    proofs = await self.evaluate(candidate, task, 0, current, integrated=True)
                    publication_started = True
                    commit = await self.repo.publish_local(candidate, current, task=task.id, attempt=0, proofs=proofs)
                    self.store.task(task.id, 'done', 'Reutilizada após checks e revisão independentes', commit)
                    self.store.emit('task_reused', task=task.id, source_revision=artifact['revision'], commit=commit)
                except (KrakenError, OSError, TimeoutError) as exc:
                    if publication_started:
                        self.store.task(task.id, 'blocked', str(exc))
                        raise
                    self.store.task(task.id, 'ready', 'Resultado anterior não passou na revalidação: ' + str(exc))
            require(progress, 'Dependências de reutilização não resolvidas')

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
        result = await self.executor.run(workspace, check.argv, readonly=True, fixtures=self.plan.fixtures,
                                         capture_paths=(check.report,) if check.report else ())
        outcome = normalize(check, result)
        data = {'check': asdict(check), 'result': asdict(result), 'outcome': asdict(outcome)}
        data['evidence_id'] = self.store.evidence(task.id, attempt, version, 'computational', outcome.ok, data)
        if not outcome.ok:
            observation = {'outcome': asdict(outcome), 'argv': check.argv,
                           'result': {'code': result.code, 'stdout': result.stdout[:2000],
                                      'stderr': result.stderr[:2000], 'truncated': result.truncated,
                                      'feedback_excerpt': len(result.stdout) > 2000 or len(result.stderr) > 2000,
                                      'artifact_paths': list(result.artifacts)}}
            raise VerificationFailed({'task': task.id, 'version': version, 'check': check.id,
                'criteria': [asdict(c) for c in task.criteria if c.id in check.criteria],
                'expected': 'Satisfy these criteria and the approved check policy',
                'observed': observation, 'evidence_id': data['evidence_id'],
                'next_action': 'Inspect the recorded command output; correct within the contract and rerun this check'})
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
        proofs = [item['evidence_id'] for item in evidence]
        for name in ('qa', 'reviewer'):
            self.gate()
            criteria = [c.id for c in task.criteria]
            prompt = canonical({'contract': asdict(task), 'version': version, 'diff': diff,
                                'evidence': evidence, 'integrated': integrated,
                                'plan_revision': self.plan.revision,
                                'objective': self.plan.objective, 'scope': self.plan.scope,
                                'exclusions': self.plan.exclusions, 'glossary': self.plan.glossary,
                                'environment': {'image': self.plan.image, 'network': 'none'},
                                'decisions': self.plan.decisions}) + '\n' + verdict_instruction(criteria)
            tools = Tools(self.executor, workspace, task, readonly=True, fixtures=self.plan.fixtures)
            result = verdict(await agent(self.client, self.config.role(name), prompt, tools), criteria)
            proofs.append(self.store.evidence(task.id, attempt, version, name, result['passed'], result))
            if result.get('needs_decision'):
                raise DecisionRequired(result['question'] + '\nRecomendação: ' + result['recommendation'])
            if not result['passed']:
                raise VerificationFailed({'task': task.id, 'version': version, 'reviewer': name,
                    'criteria': [asdict(c) for c in task.criteria],
                    'expected': 'Independent acceptance of every approved criterion',
                    'observed': result, 'evidence_id': proofs[-1],
                    'next_action': 'Resolve the recorded findings within the contract, then repeat checks and independent review'})
        return proofs

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
                                    'role': self.config.role(task.role).snapshot(),
                                    'environment': {'image': self.plan.image, 'network': 'none'},
                                    'objective': self.plan.objective, 'scope': self.plan.scope,
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
                    require(not self.store.pending_publications(self.plan.revision), 'Transferência anterior precisa de recuperação')
                    self.store.task(task.id, 'integrating', candidate=commit)
                    candidate, current = await self.repo.candidate(task, attempt, workspace, commit, base)
                    # Re-evaluate both fast checks and reviewers against the integrated candidate.
                    proofs = await self.evaluate(candidate, task, attempt, current, integrated=True)
                    self.gate()
                    publication_started = True
                    integrated = await self.repo.publish_local(candidate, current, task=task.id,
                                                               attempt=attempt, proofs=proofs)
                    self.store.task(task.id, 'done', 'Integrada e validada', integrated)
        except asyncio.CancelledError:
            self.store.task(task.id, 'blocked', 'Cancelada; reconciliar efeitos antes de repetir')
            raise
        except (KrakenError, TimeoutError, OSError) as exc:
            detail = str(exc) or 'Tempo de tentativa esgotado'
            # A conflicted/incomplete integration is preserved; do not retry uncertain effects.
            blocked = (isinstance(exc, DecisionRequired) or attempt >= self.plan.limits.attempts or publication_started
                       or self.store.get('control') != 'running')
            if publication_started:
                self.store.set('control', 'recovery_required')
            self.store.task(task.id, 'blocked' if blocked else 'ready', detail)
            self.store.emit('correction_feedback', task=task.id, attempt=attempt,
                            feedback=exc.feedback if isinstance(exc, VerificationFailed) else detail,
                            harness_suggestion='Revisar controles se esta falha se repetir')
