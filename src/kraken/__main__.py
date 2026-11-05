"""No API call or project command is implicit in CLI startup."""
import argparse
import asyncio
import json
from pathlib import Path
import sys

from .domain import KrakenError, Plan
from .git import git
from .session import Session, initialize


async def init_project(root):
    # Read-only Git validation before creating metadata.
    top = await git(root, 'rev-parse', '--show-toplevel')
    if Path(top).resolve() != root.resolve():
        raise KrakenError('Selecione a raiz do projeto Git')
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


def main(argv=None):
    parser = argparse.ArgumentParser(description='Kraken — planejamento e orquestração pelo terminal')
    subs = parser.add_subparsers(dest='command', required=True)
    for command in ('init', 'tui', 'status', 'health'):
        p = subs.add_parser(command)
        p.add_argument('project', type=Path)
    p = subs.add_parser('import-plan')
    p.add_argument('project', type=Path)
    p.add_argument('file', type=Path)
    args = parser.parse_args(argv)
    session = None
    try:
        if args.command == 'init':
            print('Configuração criada/preservada:', asyncio.run(init_project(args.project)))
            return 0
        session = Session(args.project)
        if args.command == 'import-plan':
            session.save_plan(Plan.from_dict(json.loads(args.file.read_text())))
            print('Plano importado sem aprovação. Revise pela TUI.')
        elif args.command == 'health':
            print(asyncio.run(session.health()))
        elif args.command == 'status':
            print(json.dumps({'control': session.store.get('control'),
                              'tasks': session.store.tasks(), 'approved': session.store.get('approved'),
                              'calls': session.store.get('calls', 0)}, ensure_ascii=False, indent=2))
        else:
            from .ui import KrakenApp
            KrakenApp(session).run()
        return 0
    except (KrakenError, OSError, ValueError) as exc:
        print('kraken:', session.store.redactor.text(exc) if session else str(exc), file=sys.stderr)
        return 1
    finally:
        if session:
            session.close()


if __name__ == '__main__':
    sys.exit(main())
