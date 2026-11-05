"""No API call or project command is implicit in CLI startup."""
import argparse
import asyncio
import json
from pathlib import Path
import sys

from .domain import KrakenError, Plan
from .git import git
from .session import Session, initialize
from .projects import Catalog, init_project, sessions


def main(argv=None):
    parser = argparse.ArgumentParser(description='Kraken — planejamento e orquestração pelo terminal')
    subs = parser.add_subparsers(dest='command', required=True)
    for command in ('init', 'tui', 'status', 'health', 'sessions', 'new-session'):
        p = subs.add_parser(command)
        p.add_argument('project', type=Path, nargs='?' if command == 'tui' else None)
        if command == 'tui':
            p.add_argument('--state-dir', type=Path)
            p.add_argument('--session')
    p = subs.add_parser('import-plan')
    p.add_argument('project', type=Path)
    p.add_argument('file', type=Path)
    args = parser.parse_args(argv)
    session = None
    try:
        if args.command == 'init':
            print('Configuração criada/preservada:', asyncio.run(init_project(args.project)))
            return 0
        if args.command == 'sessions':
            print(json.dumps(sessions(args.project), ensure_ascii=False, indent=2))
            return 0
        if args.command == 'tui':
            from .ui import KrakenApp, ProjectLauncher
            catalog = Catalog(args.state_dir)
            choice = (str(args.project), args.session) if args.project else ProjectLauncher(catalog).run()
            while choice:
                root, selection = choice
                session = Session(Path(root), session_id=selection)
                catalog.add(session.root)
                try:
                    choice = KrakenApp(session, catalog=catalog).run()
                finally:
                    session.close()
                    session = None
            return 0
        session = Session(args.project, session_id='new' if args.command == 'new-session' else None)
        if args.command == 'new-session':
            print('Nova sessão:', session.selection_id)
            return 0
        if args.command == 'import-plan':
            session.save_plan(Plan.from_dict(json.loads(args.file.read_text())))
            print('Plano importado sem aprovação. Revise pela TUI.')
        elif args.command == 'health':
            print(asyncio.run(session.health()))
        elif args.command == 'status':
            print(json.dumps({'control': session.store.get('control'),
                              'tasks': session.store.tasks(), 'approved': session.store.get('approved'),
                              'calls': session.store.get('calls', 0)}, ensure_ascii=False, indent=2))

        return 0
    except (KrakenError, OSError, ValueError) as exc:
        print('kraken:', session.store.redactor.text(exc) if session else str(exc), file=sys.stderr)
        return 1
    finally:
        if session:
            session.close()


if __name__ == '__main__':
    sys.exit(main())
