"""Application operations shared by the TUI and CLI, without UI imports."""
from dataclasses import asdict
from hashlib import sha256
import json
import fcntl
from pathlib import Path

from .agent import agent, json_object, verdict, verdict_instruction
from .config import Config, Redactor
from .domain import PLAN_REVIEW_CRITERIA, KrakenError, Limits, Plan, canonical, require
from .git import Repository, git
from .orchestrator import Orchestrator
from .provider import Budget, ModelClient
from .store import Store
from .tools import ProjectReader
from .harness import normalize
from .projects import select, save_draft

PLAN_FORMAT = '''Return only JSON, either {"question":"one decision", "recommendation":"recommended answer"}
or {"plan": {...}}. Questions may include "memory": {"decisions": {"stable-key":"settled decision"},
"glossary": {"term":"definition"}, "open_questions": ["remaining decision"]}.
Update durable memory to retain settled decisions; it is not approval. A plan has objective, scope, exclusions (strings), decisions (list of strings),
glossary (object), uncertainties (list), fixtures (object path->sha256), image (registry image@sha256:digest),
limits (concurrency,calls,active_seconds,command_seconds,attempt_seconds,attempts,turns,output_tokens,
counter_only boolean, max_estimated_cost_usd optional numeric threshold), tasks (list). Each task: id,title,goal,role (one of the configured available_roles), allowed_paths
(list of explicit relative files/directories, no globs), depends_on (list of task ids), references (list of paths),
criteria (list of {id,description}), checks (list of {id,argv:list,criteria:list of ids,
category:behaviour/maintainability/architecture,timing:before_integration/after_integration/health,
runner:auto/command/unittest/pytest/junit,report:optional /tmp/file.xml,min_passed:positive integer,
allow_skipped:false unless explicitly approved,allow_expected_failures:false unless explicitly approved}).
Every criterion needs an executable check and every task a before_integration behaviour check.
Ask for missing decisions, approved examples, actual image digest and actual commands; do not invent them.
Do not set counter_only true unless the user explicitly accepted operation without a monetary ceiling.
Only propose a plan when blocking uncertainty is resolved. Do not approve or execute it.
Propose the implementation team through task roles, using their actual skills, models and authorized tools.
Product, orchestrator, QA and reviewer always participate; optional specialist roles must be justified by the tasks.
Return a whole consolidated revision when the user changes an existing plan. No Markdown fences.'''


def initialize(root):
    root = Path(root).resolve()
    require(',' not in str(root), 'Caminho do projeto não pode conter vírgula (mount Docker)')
    meta = root / '.kraken'
    require(not meta.is_symlink(), '.kraken não pode ser link simbólico')
    meta.mkdir(exist_ok=True)
    config = meta / 'project.toml'
    if not config.exists():
        config.write_text('# Credentials are environment variable references, never values.\n'
                          '[provider]\nbase_url = "https://api.openai.com/v1"\n'
                          '# model = "your-model"  # or set OPENAI_MODEL\n'
                          'api_key_env = "OPENAI_API_KEY"\n')
    # Runtime data must not become an untracked source change or enter a commit.
    # Resolve Git's own exclude location; linked repos are not assumed to use .git/.
    return config


class Session:
    def __init__(self, root, *, client=None, session_id=None):
        self.root = Path(root).resolve()
        require(',' not in str(self.root), 'Caminho do projeto não pode conter vírgula')
        require((self.root / '.kraken/project.toml').is_file(), 'Execute kraken init <projeto> primeiro')
        require(not (self.root / '.kraken').is_symlink() and
                not (self.root / '.kraken/runtime').is_symlink(), 'Metadados não podem ser symlinks')
        self.config = Config(self.root)
        runtime = self.root / '.kraken/runtime'
        runtime.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.project_lock = (runtime / '.project.lock').open('a')
        try:
            fcntl.flock(self.project_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self.selection_id, directory = select(self.root, session_id)
            self.store = Store(directory, Redactor(self.config.secrets()),
                               session_id=self.selection_id if self.selection_id != 'legacy' else None)
        except BlockingIOError as exc:
            self.project_lock.close()
            raise KrakenError('Já existe uma sessão aberta neste projeto') from exc
        except BaseException:
            self.project_lock.close()
            raise
        self.store.recover()
        self.budget = Budget(self.store)
        self.client = ModelClient(self.store, self.budget, client=client)
        self.orchestrator = None

    def close(self):
        try:
            save_draft(self.root, self.selection_id)
        finally:
            self.store.close()
            self.project_lock.close()

    @property
    def plan(self):
        path = self.root / '.kraken/plan.json'
        if not path.exists():
            return None
        try:
            require(path.stat().st_size <= 100_000, 'Plano excede tamanho máximo')
            return Plan.from_dict(json.loads(path.read_text()))
        except (json.JSONDecodeError, OSError) as exc:
            raise KrakenError('Não foi possível carregar plan.json') from exc

    def save_plan(self, plan):
        require(len(canonical(plan.to_dict())) <= 100_000, 'Plano excede tamanho máximo')
        self.store.save_plan(plan)
        directory = self.root / '.kraken'
        # Write complete versions atomically so interruptions cannot approve half a file.
        for name, text in [('plan.json', json.dumps(plan.to_dict(), ensure_ascii=False, indent=2) + '\n'),
                           ('plan.md', plan.render())]:
            temp = directory / (name + '.tmp')
            require(not temp.is_symlink() and not (directory / name).is_symlink(), 'Artefato é symlink')
            temp.write_text(text)
            temp.replace(directory / name)

    async def context(self):
        await Repository(self.root, self.store).inspect()
        names = (await git(self.root, 'ls-files', '-z')).split('\0')
        context, total = {}, 0
        for name in names:
            if not name or name.startswith(('.git/', '.kraken/')):
                continue
            path = self.root / name
            if (not path.is_file() or path.is_symlink() or
                    any(p.startswith('.env') or p.endswith(('.pem', '.key')) for p in path.parts)):
                continue
            if total >= 40_000:
                break
            try:
                with path.open() as source:
                    text = source.read(4000)
            except (UnicodeError, OSError):
                continue
            context[name] = text
            total += len(text)
        return self.store.redactor.data({'tracked_paths': names[:300], 'samples': context,
                    'bounded_snapshot': True, 'note': 'Samples may be incomplete; absence is not proof.'})

    async def ask(self, text):
        require(bool(text.strip()), 'Escreva a decisão ou intenção')
        self.config = Config(self.root)
        self.store.redactor = Redactor(self.config.secrets())
        self.budget.limits = self.config.planning_limits
        self.config.validate_environment({'product'})
        self.store.emit('planning_started')
        prior = self.store.get('interview', [])
        clean = self.store.redactor.text(text)
        prompt = canonical({'project': await self.context(), 'history': prior[-20:], 'durable_memory': self.store.get('planning_memory', {}),
                            'current_plan': self.plan.to_dict() if self.plan else None,
                            'review_findings': self.store.plan_reviews(self.plan, self.config.revision) if self.plan else {},
                            'pending_decision': self.store.get('planning_decision'),
                            'user_message': clean, 'available_roles': self.config.snapshot()['roles'],
                            'default_limits': asdict(Limits()), 'planning_budget': asdict(self.config.planning_limits)}) + '\n' + PLAN_FORMAT
        response = json_object(await agent(self.client, self.config.role('product'), prompt,
                                          ProjectReader(self.root, self.store)))
        memory = response.pop('memory', None)
        if memory is not None:
            require(isinstance(memory, dict) and set(memory) == {'decisions', 'glossary', 'open_questions'}
                    and isinstance(memory['decisions'], dict) and isinstance(memory['glossary'], dict)
                    and isinstance(memory['open_questions'], list)
                    and all(isinstance(k, str) and isinstance(v, str) for section in ('decisions', 'glossary')
                            for k, v in memory[section].items())
                    and all(isinstance(q, str) for q in memory['open_questions']), 'Memória de planejamento inválida')
            previous = self.store.get('planning_memory', {'decisions': {}, 'glossary': {}})
            memory['decisions'] = previous['decisions'] | memory['decisions']
            memory['glossary'] = previous['glossary'] | memory['glossary']
            require(len(canonical(memory)) <= 20_000, 'Memória excede limite; consolide decisões explicitamente')
            self.store.set('planning_memory', self.store.redactor.data(memory))
        require(set(response) in ({'question', 'recommendation'}, {'plan'}), 'Resposta de planejamento inválida')
        if 'plan' in response:
            plan = Plan.from_dict(self.store.redactor.data(response['plan']))
            for task in plan.tasks:
                self.config.role(task.role)
            self.save_plan(plan)
            message = f'Plano {plan.revision[:12]} gerado. Confira os contratos e solicite revisão.'
        else:
            require(all(isinstance(v, str) and v.strip() for v in response.values()), 'Pergunta inválida')
            message = response['question'] + '\nRecomendação: ' + response['recommendation']
        prior += [{'user': clean, 'response': self.store.redactor.text(message)}]
        self.store.set('interview', prior)
        self.store.emit('planning_message', text=message)
        return message

    async def review(self):
        plan = self.plan
        require(plan is not None, 'Gere ou importe um plano primeiro')
        self.config = Config(self.root)
        self.budget.limits = self.config.planning_limits
        self.store.save_plan(plan)
        self.store.redactor = Redactor(self.config.secrets())
        self.config.validate_environment({'product', 'qa', 'reviewer'})
        context = await self.context()
        self.store.set('planning_decision', None)
        for round_number in range(1, self.config.planning_rounds + 1):
            for task in plan.tasks:
                self.config.role(task.role)
            self.store.emit('plan_review_round', number=round_number, revision=plan.revision)
            findings = {}
            for name in ('qa', 'reviewer'):
                prompt = canonical({'plan': plan.to_dict(), 'project_references': context,
                                    'configuration': self.config.snapshot()}) + '\n' + verdict_instruction(PLAN_REVIEW_CRITERIA)
                prompt += (' If intent is missing, include needs_decision=true, question and recommendation. '
                           'Do not ask the human for facts you can inspect in the project.')
                result = verdict(await agent(self.client, self.config.role(name), prompt,
                                            ProjectReader(self.root, self.store)), PLAN_REVIEW_CRITERIA)
                require(self.plan.revision == plan.revision and Config(self.root).revision == self.config.revision,
                        'Plano/configuração mudou durante a revisão; nada será sobrescrito')
                self.store.review(plan, self.config.revision, name, result)
                findings[name] = result
            if all(v['passed'] for v in findings.values()):
                return f'Plano {plan.revision[:12]} revisado em {round_number} rodada(s). A aprovação humana continua obrigatória.'
            decisions = [v for v in findings.values() if v.get('needs_decision') is True]
            if decisions:
                decision = decisions[0]
                require(isinstance(decision.get('question'), str) and decision['question'].strip()
                        and isinstance(decision.get('recommendation'), str), 'Decisão sem pergunta/recomendação')
                self.store.set('planning_decision', decision)
                self.store.emit('planning_decision_required', decision=decision)
                return decision['question'] + '\nRecomendação: ' + decision['recommendation']
            if round_number == self.config.planning_rounds:
                break
            locked = ('objective', 'scope', 'exclusions', 'decisions', 'image', 'fixtures', 'limits')
            prompt = canonical({'plan': plan.to_dict(), 'findings': findings, 'references': context,
                                'locked_fields': locked}) + '\n' + PLAN_FORMAT
            prompt += (' Correct technical deficiencies only. Preserve locked fields exactly. '
                       'Return a question if a correction requires new intent, scope, permissions or limits.')
            response = json_object(await agent(self.client, self.config.role('product'), prompt,
                                              ProjectReader(self.root, self.store)))
            require(self.plan.revision == plan.revision, 'O plano foi editado durante a correção')
            if set(response) == {'question', 'recommendation'}:
                require(all(isinstance(v, str) and v.strip() for v in response.values()), 'Decisão inválida')
                self.store.set('planning_decision', response)
                self.store.emit('planning_decision_required', decision=response)
                return response['question'] + '\nRecomendação: ' + response['recommendation']
            require(set(response) == {'plan'}, 'Correção precisa retornar plano ou uma decisão')
            repaired = Plan.from_dict(response['plan'])
            require(all(canonical(plan.to_dict()[key]) == canonical(repaired.to_dict()[key]) for key in locked),
                    'Correção alteraria decisões/escopo/limites; exige decisão do usuário')
            require(repaired.revision != plan.revision, 'Correção não modificou o plano reprovado')
            self.save_plan(repaired)
            self.store.emit('plan_corrected', previous=plan.revision, revision=repaired.revision, findings=findings)
            plan = repaired
        self.store.emit('plan_review_exhausted', revision=plan.revision, rounds=self.config.planning_rounds)
        return 'Limite de revisão/correção atingido. Plano permanece sem aprovação; consulte os achados.'

    def approve(self, revision, config_revision):
        plan = self.plan
        require(plan is not None and plan.revision == revision, 'O plano mudou; confira a nova revisão')
        config = Config(self.root)
        require(config.revision == config_revision, 'Configuração ou skills mudaram durante a aprovação')
        if plan.limits.max_estimated_cost_usd is not None:
            for role in {t.role for t in plan.tasks} | {'product', 'orchestrator', 'qa', 'reviewer'}:
                profile = config.role(role).profile
                require(profile.input_usd_per_million is not None and profile.output_usd_per_million is not None,
                        'Configure preços em todos os papéis antes de aprovar o limite estimado')
        Repository(self.root, self.store).fixtures(self.root, plan)
        self.store.approve(plan, config.revision, explicit_user_action=True)

    async def run(self):
        require(self.orchestrator is None, 'Execução já ativa')
        plan = self.plan
        require(plan is not None, 'Não há plano')
        self.config = Config(self.root)
        self.store.redactor = Redactor(self.config.secrets())
        self.orchestrator = Orchestrator(self.root, self.store, self.config, plan, self.client)
        try:
            await self.orchestrator.run()
        finally:
            self.orchestrator = None

    def pause(self):
        if self.orchestrator:
            self.orchestrator.pause()

    def cancel(self):
        if self.orchestrator:
            self.orchestrator.cancel()

    async def health(self):
        require(self.orchestrator is None, 'Encerre a execução antes de verificar saúde')
        plan = self.plan
        require(plan is not None, 'Não há plano')
        config = Config(self.root)
        self.store.assert_approved(plan, config.revision)
        self.budget.limits = plan.limits
        from .tools import DockerExecutor
        def gate():
            self.store.assert_approved(self.plan, Config(self.root).revision)
            self.budget.check()
        executor = DockerExecutor(self.store, self.budget, plan.image,
                                  docker=('docker', '--context', config.docker_context), policy=gate)
        await executor.preflight()
        repo = Repository(self.root, self.store)
        await repo.clean_source()
        workspace = repo.integration if repo.integration.exists() else self.root
        checks = [(task, check) for task in plan.tasks for check in task.checks if check.timing == 'health']
        require(bool(checks), 'Plano não contém sensores de saúde; nenhuma verificação executada')
        version = await git(workspace, 'rev-parse', 'HEAD')
        all_passed = True
        for task, check in checks:
            result = await executor.run(workspace, check.argv, readonly=True, fixtures=plan.fixtures,
                                        capture_paths=(check.report,) if check.report else ())
            outcome = normalize(check, result)
            self.store.evidence(task.id, 0, version, 'health', outcome.ok,
                                {'check': asdict(check), 'result': asdict(result), 'outcome': asdict(outcome)})
            all_passed &= outcome.ok
        require(all_passed, 'Há sensores de saúde reprovados; consulte as evidências')
        return f'{len(checks)} sensores de saúde executados e aprovados.'

    async def retry(self, task_id):
        require(self.orchestrator is None, 'Encerre a execução antes de reconciliar')
        plan = self.plan
        require(plan is not None, 'Não há plano')
        self.config = Config(self.root)
        self.store.assert_approved(plan, self.config.revision)
        row = next((r for r in self.store.tasks() if r['id'] == task_id), None)
        require(row and row['status'] == 'blocked', 'Tarefa não está bloqueada')
        repo = Repository(self.root, self.store)
        publication = self.store.get(f'publication:{plan.revision}:{task_id}:{row["attempt"]}')
        if publication:
            await self.run()
            return 'Transferência reconciliada com evidências; consulte o estado das tarefas.'
        require(row['attempt'] < plan.limits.attempts, 'Limite de tentativas atingido')
        await repo.prepare()
        from .process import process
        running = await process(['docker', '--context', self.config.docker_context, 'ps', '-a', '--filter',
                                 'label=kraken.session=' + self.store.get('session'), '--format', '{{.Names}}'])
        require(running.code == 0 and not running.stdout.strip(),
                'Há container pendente ou runtime indisponível; reconciliar antes de repetir')
        self.store.task(task_id, 'ready', 'Usuário confirmou reconciliação; nova tentativa autorizada')
