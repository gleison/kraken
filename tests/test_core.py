from kraken.domain import PLAN_REVIEW_CRITERIA
import asyncio
from dataclasses import replace
from pathlib import Path
import tempfile
import unittest

from kraken.config import Config, Redactor
from kraken.domain import Check, KrakenError, Limits, Plan
from kraken.provider import Budget
from kraken.store import Store
from helpers import plan


class CoreTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        self.store = Store(self.root / 'runtime')

    def tearDown(self):
        self.store.close()
        self.tmp.cleanup()

    def test_plan_roundtrip_stable_revision(self):
        p = plan()
        self.assertEqual(p.revision, Plan.from_dict(p.to_dict()).revision)
        self.assertEqual(set(p.team), {'product', 'orchestrator', 'backend', 'qa', 'reviewer'})
        self.assertIn('## Time', p.render())

    def test_dependency_cycle_and_unknown_dependency_rejected(self):
        p = plan()
        for dependencies in [('MISSING',), ('TASK-1',)]:
            with self.assertRaises(KrakenError):
                replace(p, tasks=(replace(p.tasks[0], depends_on=dependencies),))

    def test_arbitrary_paths_and_uncovered_criteria_rejected(self):
        p = plan()
        for path in ('../outside', '/etc/passwd', '.git/config', '*.py', '.kraken/plan.json'):
            with self.assertRaises(KrakenError):
                replace(p.tasks[0], allowed_paths=(path,))
        with self.assertRaises(KrakenError):
            replace(p.tasks[0], checks=())
        directory = replace(p.tasks[0], allowed_paths=('src/api',))
        self.assertTrue(directory.overlaps(replace(p.tasks[0], allowed_paths=('src/api/handler.py',))))
        self.assertFalse(directory.overlaps(replace(p.tasks[0], allowed_paths=('src/api2',))))

    def test_fixture_cannot_be_edited(self):
        p = plan()
        with self.assertRaises(KrakenError):
            replace(p, tasks=(replace(p.tasks[0], allowed_paths=('expected.txt',)),))

    def test_unpinned_image_rejected(self):
        with self.assertRaises(KrakenError):
            replace(plan(), image='python:latest')

    def test_explicit_approval_requires_fresh_independent_reviews(self):
        p = plan()
        self.store.save_plan(p)
        with self.assertRaises(KrakenError):
            self.store.approve(p, 'config')
        for role in ('qa', 'reviewer'):
            self.store.review(p, 'config', role, {'passed': True, 'feedback': 'Contract reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
        with self.assertRaises(KrakenError):
            self.store.approve(p, 'changed-config', explicit_user_action=True)
        self.store.approve(p, 'config', explicit_user_action=True)
        self.store.assert_approved(p, 'config')
        with self.assertRaises(KrakenError):
            self.store.assert_approved(replace(p, scope='changed'), 'config')

    def test_unknown_cost_requires_explicit_counter_mode(self):
        p = replace(plan(), limits=Limits())
        self.store.save_plan(p)
        for role in ('qa', 'reviewer'):
            self.store.review(p, 'cfg', role, {'passed': True, 'feedback': 'Contract reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
        with self.assertRaises(KrakenError):
            self.store.approve(p, 'cfg', explicit_user_action=True)

    def test_same_plan_does_not_reset_state(self):
        p = plan()
        self.store.save_plan(p)
        for role in ('qa', 'reviewer'):
            self.store.review(p, 'cfg', role, {'passed': True, 'feedback': 'Contract reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
        self.store.approve(p, 'cfg', explicit_user_action=True)
        self.store.claim('TASK-1')
        self.store.save_plan(Plan.from_dict(self.store.get('plan')))
        self.assertEqual(self.store.tasks()[0]['status'], 'running')

    def test_no_duplicate_claim_and_recovery_is_not_success(self):
        p = plan()
        self.store.save_plan(p)
        self.store.task('TASK-1', 'ready')
        self.store.claim('TASK-1')
        with self.assertRaises(KrakenError):
            self.store.claim('TASK-1')
        self.store.set('control', 'running')
        self.store.recover()
        self.assertEqual(self.store.tasks()[0]['status'], 'blocked')
        self.assertEqual(self.store.get('control'), 'paused')

    def test_second_process_owner_is_rejected(self):
        with self.assertRaises(KrakenError):
            Store(self.root / 'runtime')

    def test_atomic_budget_reservation(self):
        budget = Budget(self.store, Limits(calls=1))
        budget.reserve()
        with self.assertRaises(KrakenError):
            budget.reserve()
        self.assertEqual(self.store.get('calls'), 1)

    def test_redaction_preserves_structure(self):
        redactor = Redactor(['private-token-with-"quote'])
        data = redactor.data({'x': 'private-token-with-"quote', 'nested': ['Bearer unknown']})
        self.assertEqual(data['x'], '[REDACTED]')
        self.assertNotIn('unknown', data['nested'][0])

    def test_default_configuration_does_not_need_credentials(self):
        config = Config(self.root)
        self.assertIn('roles', config.snapshot())
        with self.assertRaises(KrakenError):
            config.role('backend')

    def test_role_override_and_skill_change_invalidate_snapshot(self):
        directory = self.root / '.kraken'
        directory.mkdir()
        (directory / 'project.toml').write_text('[provider]\nmodel="default"\n'
             '[roles.qa.provider]\nmodel="judge"\n')
        original = Config(self.root)
        self.assertEqual(original.role('qa').profile.model, 'judge')
        self.assertEqual(original.role('backend').profile.model, 'default')
        skill = directory / 'skills/qa'
        skill.mkdir(parents=True)
        (skill / 'SKILL.md').write_text('A custom independent QA procedure')
        self.assertNotEqual(Config(self.root).revision, original.revision)

    def test_external_runtime_skill_rejected(self):
        directory = self.root / '.kraken/skills/planning'
        directory.mkdir(parents=True)
        (directory / 'SKILL.md').write_text('Call the Skill tool with grilling')
        with self.assertRaises(KrakenError):
            Config(self.root)

    def test_concurrent_estimated_cost_reservations(self):
        budget = Budget(self.store, Limits(max_estimated_cost_usd=0.10))
        first = budget.reserve(0.06)
        with self.assertRaises(KrakenError):
            budget.reserve(0.06)
        budget.settle(first, 0.02)
        budget.reserve(0.06)
        self.assertAlmostEqual(self.store.get('estimated_cost_usd'), 0.02)
        self.assertAlmostEqual(self.store.get('reserved_cost_usd'), 0.06)

    def test_malformed_config_produces_actionable_error(self):
        directory = self.root / '.kraken'
        directory.mkdir()
        (directory / 'project.toml').write_text('provider="not a table"\n')
        with self.assertRaises(KrakenError):
            Config(self.root)

    def test_replan_invalidates_changed_task_and_transitive_dependents_only(self):
        from dataclasses import asdict
        first = plan().tasks[0]
        second = replace(plan('TASK-2', 'second.txt').tasks[0], depends_on=(first.id,))
        third = plan('TASK-3', 'third.txt').tasks[0]
        old = replace(plan(), tasks=(first, second, third))
        self.store.save_plan(old)
        for task in old.tasks:
            self.store.task(task.id, 'done', candidate='1' * 40)
            self.store.set(f'publication:{old.revision}:{task.id}:0',
                           {'status': 'completed', 'commit': '1' * 40, 'base': '2' * 40})
        revised = replace(old, tasks=(replace(first, goal='Different intent'), second, third))
        self.store.save_plan(revised)
        states = {r['id']: r['status'] for r in self.store.tasks()}
        self.assertEqual(states['TASK-1'], 'awaiting_approval')
        self.assertEqual(states['TASK-2'], 'awaiting_approval')
        self.assertEqual(states['TASK-3'], 'reuse_pending')
        self.assertIsNone(self.store.get('approved'))
        self.assertEqual(self.store.db.execute('SELECT COUNT(*) FROM task_history').fetchone()[0], 3)

    def test_pending_publication_cannot_be_hidden_by_replanning(self):
        p = plan()
        self.store.save_plan(p)
        self.store.set(f'publication:{p.revision}:TASK-1:1', {'status': 'started'})
        with self.assertRaisesRegex(KrakenError, 'transferências pendentes'):
            self.store.save_plan(replace(p, scope='New scope'))
        self.assertEqual(self.store.get('plan')['scope'], p.scope)
