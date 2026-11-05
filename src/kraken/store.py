"""Transactional local state and bounded, sanitized operational events."""
from contextlib import contextmanager
from dataclasses import asdict
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
    def __init__(self, directory: Path, redactor=None, session_id=None):
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
            CREATE TABLE IF NOT EXISTS task_history (revision TEXT, id TEXT, document TEXT, PRIMARY KEY(revision,id));
            CREATE TABLE IF NOT EXISTS plan_revisions (revision TEXT PRIMARY KEY, document TEXT NOT NULL);
            CREATE TABLE IF NOT EXISTS tasks (
                id TEXT PRIMARY KEY, status TEXT NOT NULL, attempt INTEGER DEFAULT 0,
                detail TEXT DEFAULT '', candidate TEXT DEFAULT '');
            CREATE TABLE IF NOT EXISTS evidence (
                id TEXT PRIMARY KEY, task TEXT, attempt INTEGER, version TEXT,
                kind TEXT, passed INTEGER, data TEXT);
        ''')
        self.db.commit()
        if not self.get('session'):
            self.set('session', session_id or uuid.uuid4().hex[:12])
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
        require(not self.pending_publications(), 'Reconcilie transferências pendentes antes de alterar o plano')
        old_plan = Plan.from_dict(previous) if previous else None
        old_rows = {row['id']: row for row in self.tasks()}
        carry = {}
        guards = ('decisions', 'exclusions', 'image', 'fixtures', 'glossary')
        if old_plan and all(canonical(old_plan.to_dict()[k]) == canonical(plan.to_dict()[k]) for k in guards):
            old_tasks = {t.id: t for t in old_plan.tasks}
            pending_carry = self.get('reuse', {})
            for task in plan.tasks:
                row = old_rows.get(task.id)
                if not row or task.id not in old_tasks or canonical(asdict(task)) != canonical(asdict(old_tasks[task.id])):
                    continue
                if row['status'] == 'reuse_pending' and task.id in pending_carry:
                    carry[task.id] = pending_carry[task.id]
                elif row['status'] == 'done':
                    publication = self.get(f'publication:{old_plan.revision}:{task.id}:{row["attempt"]}', {})
                    if publication.get('status') == 'completed' and publication.get('commit') and publication.get('base'):
                        carry[task.id] = {'commit': publication['commit'], 'base': publication['base'],
                                         'source': str(self.directory / 'workspaces' / old_plan.revision[:12] / 'integration'),
                                         'attempt': row['attempt'], 'revision': old_plan.revision}
            changed = True
            while changed:
                changed = False
                for task in plan.tasks:
                    if task.id in carry and any(dep not in carry for dep in task.depends_on):
                        del carry[task.id]
                        changed = True
        with self.transaction():
            if old_plan:
                self.db.execute('INSERT OR IGNORE INTO plan_revisions VALUES (?,?)',
                                (old_plan.revision, canonical(old_plan.to_dict())))
                self.db.executemany('INSERT OR REPLACE INTO task_history VALUES (?,?,?)',
                                    [(old_plan.revision, key, canonical(row)) for key, row in old_rows.items()])
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', ('plan', canonical(plan.to_dict())))
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', ('approved', 'null'))
            self.db.execute('INSERT OR REPLACE INTO meta VALUES (?,?)', ('reuse', canonical(carry)))
            self.db.execute('DELETE FROM tasks')
            self.db.executemany('INSERT INTO tasks(id,status,attempt,candidate) VALUES (?,?,?,?)',
                                [(t.id, 'reuse_pending' if t.id in carry else 'awaiting_approval',
                                  0, carry.get(t.id, {}).get('commit', '')) for t in plan.tasks])
            self.db.execute('INSERT OR IGNORE INTO plan_revisions VALUES (?,?)', (plan.revision, canonical(plan.to_dict())))
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

    def plan_reviews(self, plan, config):
        return {row['role']: json.loads(row['verdict']) for row in self.db.execute(
            'SELECT role,verdict FROM reviews WHERE revision=? AND config=?', (plan.revision, config))}

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

    def pending_publications(self, revision=None):
        prefix = f'publication:{revision}:' if revision else 'publication:'
        rows = self.db.execute('SELECT key,value FROM meta WHERE key LIKE ?', (prefix + '%',))
        return {row['key']: json.loads(row['value']) for row in rows
                if json.loads(row['value']).get('status') != 'completed'}

    def verify_proofs(self, journal, task):
        require(journal.get('config') == (self.get('approved') or {}).get('config'),
                'Configuração da evidência mudou; revalidar candidato antes de recuperar')
        ids = journal.get('proofs', [])
        require(bool(ids), 'Journal legado/sem evidências: reconciliação explícita necessária')
        records = []
        for identity in ids:
            row = self.db.execute('SELECT * FROM evidence WHERE id=?', (identity,)).fetchone()
            require(row and row['passed'] == 1 and row['version'] == journal['commit'],
                    'Evidência ausente, reprovada ou de outra versão')
            records.append(dict(row))
        current = [r for r in records if r['task'] == task.id]
        require({'qa', 'reviewer', 'computational'} <= {r['kind'] for r in current}, 'Avaliações independentes ausentes')
        checks = {json.loads(r['data']).get('check', {}).get('id') for r in current if r['kind'] == 'computational'}
        require({c.id for c in task.checks if c.timing != 'health'} <= checks, 'Checks obrigatórios ausentes')

    def recover(self):
        interrupted = self.get('interrupted', [])
        rows = self.db.execute("SELECT id FROM tasks WHERE status IN ('running','validating','integrating')").fetchall()
        self.set('interrupted', sorted(set(interrupted) | {row['id'] for row in rows}))
        with self.db:
            self.db.execute("UPDATE tasks SET status='blocked',detail='Interrompida: reconciliar antes de repetir' "
                            "WHERE status IN ('running','validating','integrating')")
        if self.get('control') in {'running', 'pausing', 'cancelling', 'recovery_required'}:
            self.set('control', 'paused')
            self.emit('recovery_required', reason='Tentativas interrompidas aguardam reconciliação')
