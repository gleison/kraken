"""Transactional local state and bounded, sanitized operational events."""
from contextlib import contextmanager
import json
import fcntl
import os
from pathlib import Path
import sqlite3
import time
import uuid

from .config import Redactor
from .domain import PLAN_REVIEW_CRITERIA, KrakenError, Plan, canonical, require


class Store:
    def __init__(self, directory: Path, redactor=None):
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.directory = directory
        require(not (directory / '.lock').is_symlink(), 'Lock não pode ser symlink')
        self.lock = (directory / '.lock').open('a')
        try:
            fcntl.flock(self.lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            self.lock.close()
            raise KrakenError('Já existe uma sessão aberta neste projeto') from exc
        self.redactor = redactor or Redactor()
        self.db = sqlite3.connect(directory / 'state.sqlite3', timeout=10)
        self.db.row_factory = sqlite3.Row
        self.db.execute('PRAGMA foreign_keys=ON')
        self.db.executescript('''
            CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS events (
                seq INTEGER PRIMARY KEY, at REAL, kind TEXT, payload TEXT);
            CREATE TABLE IF NOT EXISTS reviews (
                revision TEXT, config TEXT, role TEXT, verdict TEXT,
                PRIMARY KEY(revision,config,role));
            CREATE TABLE IF NOT EXISTS tasks (
                id TEXT PRIMARY KEY, status TEXT NOT NULL, attempt INTEGER DEFAULT 0,
                detail TEXT DEFAULT '', candidate TEXT DEFAULT '');
            CREATE TABLE IF NOT EXISTS evidence (
                id TEXT PRIMARY KEY, task TEXT, attempt INTEGER, version TEXT,
                kind TEXT, passed INTEGER, data TEXT);
        ''')
        self.db.commit()
        if not self.get('session'):
            self.set('session', uuid.uuid4().hex[:12])
            self.set('control', 'idle')
            self.set('calls', 0)
            self.set('active_seconds', 0.0)

    def close(self):
        self.db.close()
        self.lock.close()

    @contextmanager
    def transaction(self):
        self.db.execute('BEGIN IMMEDIATE')
        try:
            yield
            self.db.commit()
        except BaseException:
            self.db.rollback()
            raise

    def get(self, key, default=None):
        row = self.db.execute('SELECT value FROM meta WHERE key=?', (key,)).fetchone()
        return json.loads(row[0]) if row else default

    def set(self, key, value):
        with self.db:
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', (key, canonical(value)))

    def emit(self, event_type, **payload):
        payload = self.redactor.data(payload)
        # Each event is bounded independently; never replace exit codes with truncation.
        encoded = canonical(payload)
        if len(encoded) > 100_000:
            payload = {'truncated': True, 'summary': encoded[:10_000]}
        with self.db:
            self.db.execute('INSERT INTO events(at,kind,payload) VALUES (?,?,?)',
                            (time.time(), event_type, canonical(payload)))
            self.db.execute('DELETE FROM events WHERE seq < (SELECT COALESCE(MAX(seq),0)-10000 FROM events)')

    def events(self, after=0, limit=200):
        return [dict(row) | {'payload': json.loads(row['payload'])} for row in
                self.db.execute('SELECT * FROM events WHERE seq>? ORDER BY seq LIMIT ?', (after, limit))]

    def save_plan(self, plan: Plan):
        require(self.get('control') not in {'running', 'pausing', 'cancelling'},
                'Pause e encerre o trabalho em voo antes de alterar o plano')
        previous = self.get('plan')
        if previous and Plan.from_dict(previous).revision == plan.revision:
            return
        with self.transaction():
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', ('plan', canonical(plan.to_dict())))
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', ('approved', 'null'))
            self.db.execute('DELETE FROM tasks')
            self.db.executemany('INSERT INTO tasks(id,status) VALUES (?,?)',
                                [(t.id, 'awaiting_approval') for t in plan.tasks])
        self.emit('plan_changed', revision=plan.revision)

    def review(self, plan, config, role, verdict):
        require(role in {'qa', 'reviewer'}, 'Revisor independente inválido')
        require(type(verdict.get('passed')) is bool and isinstance(verdict.get('feedback'), str)
                and bool(verdict['feedback'].strip())
                and isinstance(verdict.get('criteria'), (list, tuple))
                and all(isinstance(c, str) for c in verdict['criteria'])
                and set(verdict['criteria']) == set(PLAN_REVIEW_CRITERIA), 'Revisão de plano incompleta')
        with self.db:
            self.db.execute('INSERT OR REPLACE INTO reviews VALUES (?,?,?,?)',
                            (plan.revision, config, role, canonical(verdict)))
        self.emit('plan_review', role=role, verdict=verdict, revision=plan.revision)

    def approve(self, plan, config, *, explicit_user_action=False):
        require(explicit_user_action, 'Aprovação exige ação explícita do usuário')
        require(not plan.uncertainties, 'Resolva as incertezas antes de aprovar')
        require(plan.limits.counter_only, 'Confirme o modo por contadores; custo monetário indisponível')
        current = self.get('plan')
        require(current is not None and Plan.from_dict(current).revision == plan.revision,
                'O plano mudou durante a aprovação')
        rows = self.db.execute('SELECT role,verdict FROM reviews WHERE revision=? AND config=?',
                               (plan.revision, config)).fetchall()
        passed = {r['role'] for r in rows if json.loads(r['verdict']).get('passed') is True}
        require(passed == {'qa', 'reviewer'}, 'QA e revisão do plano precisam aprovar esta revisão')
        self.set('approved', {'revision': plan.revision, 'config': config, 'at': time.time(), 'actor': 'user'})
        with self.db:
            self.db.execute("UPDATE tasks SET status='ready' WHERE status='awaiting_approval'")
        self.emit('plan_approved', revision=plan.revision)

    def assert_approved(self, plan, config):
        approval = self.get('approved') or {}
        require(approval.get('revision') == plan.revision and approval.get('config') == config,
                'Plano, modelos ou skills mudaram; nova revisão/aprovação necessária')

    def claim(self, task_id):
        with self.transaction():
            row = self.db.execute('SELECT * FROM tasks WHERE id=?', (task_id,)).fetchone()
            require(row is not None and row['status'] == 'ready', 'Tarefa já despachada ou indisponível')
            self.db.execute("UPDATE tasks SET status='running',attempt=attempt+1 WHERE id=?", (task_id,))
        self.emit('task_started', task=task_id, attempt=row['attempt'] + 1)
        return row['attempt'] + 1

    def task(self, task_id, status, detail='', candidate=''):
        with self.db:
            self.db.execute('UPDATE tasks SET status=?,detail=?,candidate=? WHERE id=?',
                            (status, self.redactor.text(detail), candidate, task_id))
        self.emit('task_state', task=task_id, status=status, detail=detail)

    def tasks(self):
        return [dict(r) for r in self.db.execute('SELECT * FROM tasks ORDER BY id')]

    def evidence(self, task, attempt, version, kind, passed, data):
        evidence_id = uuid.uuid4().hex
        with self.db:
            self.db.execute('INSERT INTO evidence VALUES (?,?,?,?,?,?,?)',
                            (evidence_id, task, attempt, version, kind, int(passed),
                             canonical(self.redactor.data(data))))
        self.emit('evidence', id=evidence_id, task=task, version=version, kind=kind, passed=passed, data=data)
        return evidence_id

    def recover(self):
        # An interrupted attempt is not automatically retried: effects may be uncertain.
        with self.db:
            self.db.execute("UPDATE tasks SET status='blocked',detail='Interrompida: reconciliar antes de repetir' "
                            "WHERE status IN ('running','validating','integrating')")
        if self.get('control') in {'running', 'pausing', 'cancelling'}:
            self.set('control', 'paused')
            self.emit('recovery_required', reason='Tentativas interrompidas permanecem bloqueadas')
