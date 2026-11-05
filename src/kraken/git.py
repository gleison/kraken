"""Local, isolated Git operations; never stage the user's checkout."""
from hashlib import sha256
from pathlib import Path
import os

from .domain import Plan, require, safe_path
from .process import process


async def git(root, *args, check=True):
    env = os.environ.copy()
    # Do not inherit repository selectors or execute hooks/external diffs.
    for key in ('GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_OBJECT_DIRECTORY',
                'GIT_ALTERNATE_OBJECT_DIRECTORIES'):
        env.pop(key, None)
    result = await process(['git', '-c', 'core.hooksPath=/dev/null', '-c', 'diff.external=',
                            '-c', 'core.fsmonitor=false', '-C', str(root), *args],
                           timeout=60, env=env, limit=200_000)
    if check:
        require(result.code == 0, f'Git falhou ({args[0]}): {result.stderr[:1000]}')
        require(not result.truncated, 'Saída Git truncada; operação bloqueada')
    return result.stdout.rstrip('\n') if check else result


class Repository:
    def __init__(self, root, store):
        self.root, self.store = Path(root).resolve(), store
        raw = store.get('plan')
        revision = Plan.from_dict(raw).revision[:12] if raw else 'unplanned'
        self.work = store.directory / 'workspaces' / revision
        self.branch = 'kraken/' + store.get('session') + '/' + revision
        self.integration = self.work / 'integration'

    async def inspect(self):
        top = await git(self.root, 'rev-parse', '--show-toplevel')
        require(Path(top).resolve() == self.root, 'Selecione a raiz do repositório Git')
        return await git(self.root, 'rev-parse', '--verify', 'HEAD^{commit}')

    async def clean_source(self):
        await self.inspect()
        # Ignore only Kraken's own project metadata; source changes are never stashed/reset.
        changed = await git(self.root, 'status', '--porcelain=v1', '-z', '--untracked-files=all',
                            '--', '.', ':(exclude).kraken')
        require(not changed, 'Há alterações no projeto. Preserve/commit essas alterações antes de executar')

    async def clone(self, source, destination, commit=None):
        require(not destination.exists(), 'Workspace já existe; reconciliar em vez de sobrescrever')
        destination.parent.mkdir(parents=True, exist_ok=True)
        await git(destination.parent, 'clone', '--no-hardlinks', '--no-local', '--', str(source), str(destination))
        if commit:
            await git(destination, 'checkout', '--detach', commit)
        await git(destination, 'config', 'user.name', 'Kraken')
        await git(destination, 'config', 'user.email', 'kraken@localhost')
        return destination

    async def prepare(self):
        await self.clean_source()
        head = await self.inspect()
        if self.integration.exists():
            require(self.store.get('base') == head, 'HEAD original mudou; reconciliação necessária')
            status = await git(self.integration, 'status', '--porcelain=v1')
            require(not status, 'Workspace de integração precisa de reconciliação')
        else:
            await self.clone(self.root, self.integration, head)
            await git(self.integration, 'checkout', '-b', self.branch)
            self.store.set('base', head)
        return await git(self.integration, 'rev-parse', 'HEAD')

    async def workspace(self, task, attempt):
        head = await git(self.integration, 'rev-parse', 'HEAD')
        path = self.work / f'{task.id}-{attempt}'
        await self.clone(self.integration, path, head)
        self.store.emit('workspace_created', task=task.id, attempt=attempt, base=head)
        return path, head

    async def changes(self, path):
        tracked = await git(path, 'diff', '--no-ext-diff', '--name-only', '-z', 'HEAD')
        untracked = await git(path, 'ls-files', '--others', '--exclude-standard', '-z')
        return sorted(set(n for n in (tracked + untracked).split('\0') if n))

    async def commit_task(self, path, task, plan):
        changed = await self.changes(path)
        for name in changed:
            require(task.permits(name), f'Alteração fora do contrato: {name}')
            require(not (path / name).is_symlink(), 'Novos links simbólicos não são permitidos')
        self.fixtures(path, plan)
        if changed:
            await git(path, 'add', '--', *changed)
            await git(path, 'commit', '-m', f'{task.id}: {task.title}')
        return await git(path, 'rev-parse', 'HEAD')

    def fixtures(self, path, plan):
        for name, expected in plan.fixtures.items():
            full = path / safe_path(name)
            require(full.is_file() and not full.is_symlink() and full.resolve().is_relative_to(path.resolve()),
                    f'Fixture ausente ou insegura: {name}')
            require(sha256(full.read_bytes()).hexdigest() == expected, f'Fixture aprovada mudou: {name}')

    async def candidate(self, task, attempt, task_workspace, commit, base):
        head = await git(self.integration, 'rev-parse', 'HEAD')
        candidate = self.work / f'integrate-{task.id}-{attempt}'
        await self.clone(self.integration, candidate, head)
        if commit != base:
            await git(candidate, 'fetch', '--no-tags', str(task_workspace), commit)
            result = await git(candidate, 'cherry-pick', commit, check=False)
            require(result.code == 0, 'Conflito na integração; candidato preservado para reconciliação')
        return candidate, head

    async def ref(self, repository, name):
        result = await git(repository, 'rev-parse', '--verify', name, check=False)
        return result.stdout.strip() if result.code == 0 else None

    async def publish_local(self, candidate, expected_base, *, task=None, attempt=None, proofs=()):
        await self.clean_source()
        require(await git(self.integration, 'rev-parse', 'HEAD') == expected_base,
                'Base de integração mudou durante a avaliação')
        require(await self.inspect() == self.store.get('base'), 'HEAD original mudou; revisar antes de integrar')
        commit = await git(candidate, 'rev-parse', 'HEAD')
        require(not await git(candidate, 'status', '--porcelain=v1'), 'Candidato contém alterações não avaliadas')
        current_branch = await git(self.root, 'symbolic-ref', '--quiet', 'HEAD', check=False)
        require(current_branch.stdout.strip() != 'refs/heads/' + self.branch,
                'A branch de destino está em edição pelo usuário')
        key = f'publication:{Plan.from_dict(self.store.get("plan")).revision}:{task}:{attempt}'
        journal = {'status': 'started', 'commit': commit, 'base': expected_base,
                   'candidate': str(candidate), 'branch': self.branch, 'task': task,
                   'attempt': attempt, 'proofs': list(proofs), 'origin_head': await self.inspect(), 'config': (self.store.get('approved') or {}).get('config'),
                   'previous_ref': await self.ref(self.root, 'refs/heads/' + self.branch)}
        require(task is not None and attempt is not None, 'Publicação local exige tarefa e tentativa')
        self.store.set(key, journal)
        return await self.finish_publication(key, journal)

    async def finish_publication(self, key, journal):
        await self.clean_source()
        require(journal['branch'] == self.branch, 'Journal aponta para outra branch')
        require(await self.inspect() == journal.get('origin_head'), 'HEAD do usuário mudou durante a execução')
        candidate = Path(journal['candidate'])
        require(candidate.resolve().is_relative_to(self.work.resolve()) and not candidate.is_symlink(),
                'Candidato fora do workspace aprovado')
        require(await self.ref(candidate, 'HEAD') == journal['commit'], 'Candidato mudou após validação')
        require(not await git(candidate, 'status', '--porcelain=v1'), 'Candidato alterado após validação')
        current_branch = await git(self.root, 'symbolic-ref', '--quiet', 'HEAD', check=False)
        require(current_branch.stdout.strip() != 'refs/heads/' + self.branch, 'Destino está em edição pelo usuário')
        head = await self.ref(self.integration, 'HEAD')
        require(head in {journal['base'], journal['commit']}, 'Integração divergiu; não sobrescrever')
        exported = await self.ref(self.root, 'refs/heads/' + self.branch)
        require(exported in {journal.get('previous_ref'), journal['commit']}, 'Ref de destino divergiu; não sobrescrever')
        if head != journal['commit']:
            await git(self.integration, 'fetch', '--no-tags', str(candidate), journal['commit'])
            await git(self.integration, 'merge', '--ff-only', journal['commit'])
        journal = dict(journal, status='integration_updated')
        self.store.set(key, journal)
        if exported != journal['commit']:
            await git(self.root, 'fetch', '--no-tags', str(self.integration), journal['commit'])
            await git(self.root, 'update-ref', 'refs/heads/' + self.branch, journal['commit'],
                      journal.get('previous_ref') or '0' * 40)
        journal['status'] = 'completed'
        self.store.set(key, journal)
        self.store.emit('integrated', branch=self.branch, commit=journal['commit'])
        return journal['commit']

    async def reconcile(self, plan, revalidate=None):
        tasks = {task.id: task for task in plan.tasks}
        async def refresh(key, journal, task):
            if journal.get('config') != (self.store.get('approved') or {}).get('config'):
                require(revalidate is not None, 'Configuração mudou; execute a retomada para revalidar o candidato')
                journal['proofs'] = await revalidate(Path(journal['candidate']), task, journal['attempt'],
                                                     journal['base'], integrated=True)
                journal['config'] = self.store.get('approved')['config']
                self.store.set(key, journal)
            self.store.verify_proofs(journal, task)
        for key, journal in self.store.pending_publications(plan.revision).items():
            require(journal.get('task') in tasks and 'commit' in journal,
                    'Journal incompleto/legado exige inspeção antes de repetir')
            task = tasks[journal['task']]
            await refresh(key, journal, task)
            self.fixtures(Path(journal['candidate']), plan)
            commit = await self.finish_publication(key, journal)
            self.store.task(task.id, 'done', 'Transferência recuperada a partir de evidências verificadas', commit)
        # Crash after the durable completion marker but before the task row update.
        for row in self.store.tasks():
            journal = self.store.get(f'publication:{plan.revision}:{row["id"]}:{row["attempt"]}', {})
            if row['status'] == 'blocked' and journal.get('status') == 'completed':
                await refresh(f'publication:{plan.revision}:{row["id"]}:{row["attempt"]}', journal, tasks[row['id']])
                require(await self.ref(self.root, 'refs/heads/' + self.branch) == journal['commit'],
                        'Ref não corresponde ao resultado concluído')
                self.store.task(row['id'], 'done', 'Conclusão recuperada sem repetir comandos', journal['commit'])
