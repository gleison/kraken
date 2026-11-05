"""Project catalog and immutable session locations; no UI or model dependencies."""
from contextlib import closing
import fcntl
import json
import os
from pathlib import Path
import re
import sqlite3
import uuid
from urllib.parse import quote

from .domain import KrakenError, canonical, require


class Catalog:
    def __init__(self, directory=None):
        self.directory = Path(directory or Path(os.environ.get('XDG_STATE_HOME', Path.home() / '.local/state')) / 'kraken')
        self.directory.mkdir(parents=True, exist_ok=True)
        self.path = self.directory / 'projects.json'

    def projects(self):
        if not self.path.exists():
            return []
        try:
            value = json.loads(self.path.read_text())
            require(isinstance(value, list) and all(isinstance(v, str) for v in value), 'Catálogo inválido')
            return [Path(v) for v in value]
        except (json.JSONDecodeError, OSError) as exc:
            raise KrakenError('Catálogo de projetos ilegível; preserve-o antes de corrigir') from exc

    def add(self, root):
        root = Path(root).resolve()
        require((root / '.kraken/project.toml').is_file(), 'Projeto ainda não inicializado')
        with (self.directory / '.catalog.lock').open('a') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            paths = sorted({str(p) for p in self.projects()} | {str(root)})
            temp = self.path.with_suffix('.tmp')
            temp.write_text(canonical(paths))
            temp.replace(self.path)


def active_id(root):
    runtime = Path(root) / '.kraken/runtime'
    marker = runtime / 'active.json'
    if marker.exists():
        try:
            result = json.loads(marker.read_text())
        except (OSError, json.JSONDecodeError) as exc:
            raise KrakenError('Seletor da sessão inválido') from exc
        require(isinstance(result, str) and (result == 'legacy' or re.fullmatch('[0-9a-f]{12}', result)),
                'ID da sessão inválido')
        return result
    return 'legacy' if (runtime / 'state.sqlite3').exists() else None


def session_directory(root, identity):
    runtime = Path(root) / '.kraken/runtime'
    require(identity == 'legacy' or re.fullmatch('[0-9a-f]{12}', identity), 'ID da sessão inválido')
    return runtime if identity == 'legacy' else runtime / 'sessions' / identity


def save_draft(root, identity):
    destination = session_directory(root, identity)
    destination.mkdir(parents=True, exist_ok=True, mode=0o700)
    for name in ('plan.json', 'plan.md'):
        source = Path(root) / '.kraken' / name
        require(not source.is_symlink(), 'Plano não pode ser symlink')
        if source.exists():
            require(source.stat().st_size <= 200_000, 'Plano excede limite de preservação')
            (destination / ('draft-' + name)).write_bytes(source.read_bytes())
        elif (destination / ('draft-' + name)).exists():
            (destination / ('draft-' + name)).unlink()


def select(root, requested=None):
    previous = active_id(root)
    identity = uuid.uuid4().hex[:12] if requested == 'new' or previous is None and requested is None else requested or previous
    directory = session_directory(root, identity)
    if requested not in {None, 'new'}:
        require((directory / 'state.sqlite3').is_file(), 'Sessão não encontrada')
    require(not directory.is_symlink() and directory.resolve().is_relative_to((Path(root) / '.kraken/runtime').resolve()),
            'Diretório da sessão não pode apontar para fora do runtime')
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    if previous and previous != identity:
        save_draft(root, previous)
        for name in ('plan.json', 'plan.md'):
            draft = directory / ('draft-' + name)
            target = Path(root) / '.kraken' / name
            require(not target.is_symlink(), 'Artefato não pode ser symlink')
            if draft.exists():
                temp = target.with_suffix(target.suffix + '.tmp')
                temp.write_bytes(draft.read_bytes())
                temp.replace(target)
            elif target.exists():
                target.unlink()  # Preserved in the previous session before changing selection.
    marker = Path(root) / '.kraken/runtime/active.json'
    temp = marker.with_suffix('.tmp')
    temp.write_text(canonical(identity))
    temp.replace(marker)
    return identity, directory


def sessions(root):
    runtime = Path(root) / '.kraken/runtime'
    entries = [('legacy', runtime)] if (runtime / 'state.sqlite3').exists() else []
    folder = runtime / 'sessions'
    if folder.is_dir():
        entries += [(p.name, p) for p in sorted(folder.iterdir())
                    if re.fullmatch('[0-9a-f]{12}', p.name) and not p.is_symlink()]
    result = []
    for identity, path in entries:
        try:
            with closing(sqlite3.connect('file:' + quote(str(path / 'state.sqlite3')) + '?mode=ro', uri=True)) as db:
                meta = {key: json.loads(value) for key, value in db.execute(
                    "SELECT key,value FROM meta WHERE key IN ('control','session','calls','plan')")}
                counts = dict(db.execute('SELECT status,COUNT(*) FROM tasks GROUP BY status'))
                result.append({'id': identity, 'session': meta.get('session'), 'status': meta.get('control'),
                               'calls': meta.get('calls', 0), 'objective': meta.get('plan', {}).get('objective', ''),
                               'tasks': counts})
        except (sqlite3.Error, json.JSONDecodeError):
            result.append({'id': identity, 'status': 'unreadable', 'calls': 0, 'objective': '', 'tasks': {}})
    return result


async def init_project(root):
    from .git import git
    from .session import initialize
    root = Path(root).resolve()
    top = await git(root, 'rev-parse', '--show-toplevel')
    require(Path(top).resolve() == root, 'Selecione a raiz do projeto Git')
    path = initialize(root)
    exclude = Path(await git(root, 'rev-parse', '--git-path', 'info/exclude'))
    if not exclude.is_absolute():
        exclude = root / exclude
    existing = exclude.read_text() if exclude.exists() else ''
    if '/.kraken/runtime/' not in existing.splitlines():
        exclude.parent.mkdir(parents=True, exist_ok=True)
        with exclude.open('a') as file:
            file.write(('\n' if existing and not existing.endswith('\n') else '') + '/.kraken/runtime/\n')
    return path
