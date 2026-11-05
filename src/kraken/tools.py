"""Containers are the only executor for agent tools and project checks."""
from dataclasses import asdict
import asyncio
import json
import os
from pathlib import Path
import time
import uuid

from .domain import KrakenError, require, safe_path
from .process import process

TOOL_SCHEMAS = {
    'read_file': {'path': {'type': 'string'}},
    'list_files': {'path': {'type': 'string'}},
    'write_file': {'path': {'type': 'string'}, 'content': {'type': 'string'}},
    'run_command': {'argv': {'type': 'array', 'items': {'type': 'string'}, 'minItems': 1}},
}


def schemas(names):
    return [{'type': 'function', 'function': {'name': name, 'description': {
        'read_file': 'Read a project file (bounded output).',
        'list_files': 'List files below a project directory, excluding metadata and secrets.',
        'write_file': 'Write UTF-8 content within the approved task paths.',
        'run_command': 'Run argv in the isolated container; never on the host.',
    }[name], 'parameters': {'type': 'object', 'properties': TOOL_SCHEMAS[name],
                           'required': list(TOOL_SCHEMAS[name]), 'additionalProperties': False}}}
            for name in names]


class DockerExecutor:
    def __init__(self, store, budget, image, *, docker=('docker', '--context', 'default'), memory='512m', policy=None):
        self.store, self.budget, self.image = store, budget, image
        self.docker, self.memory = tuple(docker), memory
        self.policy = policy

    async def preflight(self):
        endpoint = await process([*self.docker, 'context', 'inspect', self.docker[-1],
                                  '--format', '{{.Endpoints.docker.Host}}'], timeout=20)
        require(endpoint.code == 0 and endpoint.stdout.strip().startswith('unix://'),
                'Use um contexto Docker local com socket Unix; daemon remoto não está autorizado')
        for args in (('info', '--format', '{{.ServerVersion}}'), ('image', 'inspect', self.image)):
            result = await process([*self.docker, *args], timeout=20)
            require(result.code == 0, 'Docker/imagem indisponível; prepare a imagem por digest antes de executar')
        self.store.emit('container_ready', image=self.image, network='none')

    async def cleanup_orphans(self):
        listing = await process([*self.docker, 'ps', '-aq', '--filter',
                                 'label=kraken.session=' + self.store.get('session')], timeout=20)
        require(listing.code == 0, 'Não foi possível reconciliar containers da sessão')
        import re
        for identity in listing.stdout.split():
            require(re.fullmatch(r'[0-9a-f]{12,64}', identity), 'Identidade de container inválida')
            stopped = await process([*self.docker, 'rm', '-f', identity], timeout=20)
            require(stopped.code == 0, 'Container da sessão não pôde ser encerrado')
            self.store.emit('orphan_container_removed', container=identity)

    async def run(self, workspace: Path, argv, *, readonly=False, fixtures=(), input=None, capture_paths=()):
        require(self.policy is not None, 'Executor exige política de aprovação explícita')
        self.policy()
        require(isinstance(argv, (list, tuple)) and argv and
                all(isinstance(a, str) and '\x00' not in a for a in argv), 'argv inválido')
        self.budget.check()
        name = 'kraken-' + uuid.uuid4().hex
        mount = f'type=bind,src={workspace.resolve()},dst=/workspace' + (',readonly' if readonly else '')
        args = [*self.docker, 'run', '--rm', '--pull=never', '--name', name,
                '--label', 'kraken.session=' + self.store.get('session'),
                '--network=none', '--read-only', '--cap-drop=ALL',
                '--security-opt=no-new-privileges', '--pids-limit=128',
                f'--memory={self.memory}', '--cpus=1', '--user', f'{os.getuid()}:{os.getgid()}',
                '--tmpfs', '/tmp:rw,nosuid,nodev,size=128m,mode=1777',
                '--mount', mount, '--workdir=/workspace',
                '--env', 'PYTHONDONTWRITEBYTECODE=1', '--env', 'HOME=/tmp',
                '--env', 'HTTP_PROXY=', '--env', 'HTTPS_PROXY=', '--env', 'ALL_PROXY=',
                '--env', 'http_proxy=', '--env', 'https_proxy=', '--env', 'all_proxy=']
        # Prevent editing Git metadata/approved contracts even from arbitrary commands.
        protected = ['.git', '.kraken'] + list(fixtures)
        for path in protected:
            full = workspace / path
            if full.exists():
                require(not full.is_symlink() and full.resolve().is_relative_to(workspace.resolve()),
                        'Mount protegido não pode apontar para fora do workspace')
                args += ['--mount', f'type=bind,src={full.resolve()},dst=/workspace/{path},readonly']
        reports = {}
        for index, report in enumerate(capture_paths):
            require(report.startswith('/tmp/') and '..' not in Path(report).parts,
                    'Artefato fora de /tmp')
            directory = self.store.directory / 'artifacts' / name
            directory.mkdir(parents=True, exist_ok=True, mode=0o700)
            destination = directory / str(index)
            destination.touch(mode=0o600, exist_ok=False)
            reports[report] = destination
            # A dedicated writable output file survives the container's tmpfs teardown.
            args += ['--mount', f'type=bind,src={destination.resolve()},dst={report}']
        if input is not None:
            args += ['--interactive']
        args += [self.image, *argv]
        self.store.emit('tool_started', container=name, argv=argv, readonly=readonly)
        started = time.monotonic()
        try:
            result = await process(args, timeout=self.budget.limits.command_seconds, input=input)
            for report, destination in reports.items():
                require(not destination.is_symlink() and destination.is_file(), 'Artefato inválido')
                require(destination.stat().st_size <= 1_000_000, 'Relatório excede limite')
                if destination.stat().st_size:
                    result.artifacts[report] = destination.read_text()
            self.store.emit('tool_finished', container=name, **asdict(result))
            return result
        except TimeoutError as exc:
            raise KrakenError('Comando excedeu o tempo aprovado') from exc
        finally:
            self.budget.elapsed(time.monotonic() - started)
            # Stop only our uniquely named container, including after timeout/cancellation.
            cleanup = await asyncio.shield(process([*self.docker, 'rm', '-f', name], timeout=15))
            if cleanup.code != 0 and 'No such container' not in cleanup.stderr:
                self.store.emit('container_cleanup_uncertain', container=name)


class Tools:
    def __init__(self, executor, workspace, task, *, readonly=False, fixtures=()):
        self.executor, self.workspace, self.task = executor, workspace, task
        self.readonly, self.fixtures = readonly, fixtures

    async def call(self, name, arguments):
        require(name in TOOL_SCHEMAS and set(arguments) == set(TOOL_SCHEMAS[name]),
                'Ferramenta ou argumentos desconhecidos')
        input_data = None
        if name == 'run_command':
            argv = arguments['argv']
        else:
            path = ('.' if name == 'list_files' and arguments['path'] == '.'
                    else safe_path(arguments['path']))
            parts = Path(path).parts
            require(not any(p.startswith('.env') or p.endswith(('.pem', '.key')) for p in parts),
                    'Arquivo potencialmente secreto não pode ser lido')
            if name == 'read_file':
                argv = ['cat', '--', path]
            elif name == 'list_files':
                argv = ['find', path, '-name', '.git', '-prune', '-o', '-name', '.kraken', '-prune',
                        '-o', '-name', '.env*', '-prune', '-o', '-type', 'f', '-print']
            else:
                require(not self.readonly and self.task.permits(path), 'Escrita fora do contrato')
                require(isinstance(arguments['content'], str) and len(arguments['content']) <= 100_000,
                        'Conteúdo inválido ou excessivo')
                # Portable POSIX tools from the project's image; no Python-in-image requirement.
                argv = ['sh', '-c', 'mkdir -p -- "$1" && cat > "$2"',
                        'kraken-write', str(Path(path).parent), path]
                input_data = arguments['content'].encode()
        result = await self.executor.run(self.workspace, argv, readonly=self.readonly, fixtures=self.fixtures, input=input_data)
        return self.executor.store.redactor.data(asdict(result))


class ProjectReader:
    """Bounded read-only inspection before a container/image has been planned.

    This exposes file reads, never a host command or a write operation.
    """
    names = ('read_file', 'list_files')
    task = None

    def __init__(self, root, store):
        self.root, self.store = root.resolve(), store

    async def call(self, name, arguments):
        require(name in self.names and set(arguments) == {'path'}, 'Inspeção permite apenas leitura/listagem')
        raw = arguments['path']
        relative = '.' if raw == '.' and name == 'list_files' else safe_path(raw)
        full = (self.root / relative).resolve()
        require(full.is_relative_to(self.root), 'Referência fora do projeto')
        parts = full.relative_to(self.root).parts
        require(not any(p in {'.git', '.kraken'} or p.startswith('.env') or p.endswith(('.key', '.pem'))
                        for p in parts), 'Referência protegida ou potencialmente secreta')
        if name == 'read_file':
            require(full.is_file(), 'Referência não é arquivo')
            with full.open('rb') as file:
                content = file.read(64_001)
            result = {'code': 0, 'stdout': content[:64_000].decode(errors='replace'),
                      'truncated': len(content) > 64_000}
        else:
            require(full.is_dir(), 'Referência não é diretório')
            names = []
            for path in full.iterdir():
                if path.name in {'.git', '.kraken'} or path.name.startswith('.env') or path.suffix in {'.pem', '.key'}:
                    continue
                names.append(path.name + ('/' if path.is_dir() and not path.is_symlink() else ''))
                require(len(names) <= 2000, 'Diretório muito grande; selecione um subdiretório')
            result = {'code': 0, 'stdout': '\n'.join(sorted(names)), 'truncated': False}
        self.store.emit('project_inspected', path=relative, tool=name, truncated=result['truncated'])
        return self.store.redactor.data(result)
