import os
from dataclasses import replace
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from helpers import FakeModel, approve, plan, project
from kraken.domain import KrakenError
from kraken.git import git
from kraken.harness import normalize
from kraken.orchestrator import Orchestrator
from kraken.session import Session


@unittest.skipUnless(os.environ.get('KRAKEN_TEST_DOCKER') == '1', 'Requires local Docker opt-in')
class RecoveryTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        await project(self.root)
        self.session = Session(self.root)
        p = approve(self.session)
        self.fake = FakeModel(self.session.client)
        self.orch = Orchestrator(self.root, self.session.store, self.session.config, p, self.fake)

    async def asyncTearDown(self):
        self.session.close()
        self.tmp.cleanup()

    async def assert_recovered_without_model_retry(self):
        calls = len(self.fake.calls)
        # Even an exhausted model budget must not prevent finishing a validated ref transfer.
        self.session.store.set('calls', self.orch.plan.limits.calls)
        await self.orch.run()
        self.assertEqual(self.session.store.tasks()[0]['status'], 'done')
        self.assertEqual(self.session.store.get('control'), 'done')
        self.assertEqual(len(self.fake.calls), calls)
        self.assertEqual(await git(self.root, 'show', self.orch.repo.branch + ':result.txt'), 'hello')
        self.assertFalse((self.root / 'result.txt').exists())

    async def test_resume_after_durable_intent_before_git_mutation(self):
        with patch.object(self.orch.repo, 'finish_publication', side_effect=KrakenError('injected interruption')):
            await self.orch.run()
        self.assertTrue(self.session.store.pending_publications(self.orch.plan.revision))
        await self.assert_recovered_without_model_retry()

    async def test_resume_after_integration_advanced_before_export(self):
        import kraken.git as module
        original = module.git
        async def failing(root, *args, **kwargs):
            if Path(root) == self.root and args[0] == 'update-ref':
                raise KrakenError('injected export interruption')
            return await original(root, *args, **kwargs)
        with patch.object(module, 'git', failing):
            await self.orch.run()
        self.assertEqual(next(iter(self.session.store.pending_publications(self.orch.plan.revision).values()))['status'],
                         'integration_updated')
        await self.assert_recovered_without_model_retry()

    async def test_resume_after_export_before_completion_marker(self):
        original = self.session.store.set
        def failing(key, value):
            if key.startswith('publication:') and isinstance(value, dict) and value.get('status') == 'completed':
                raise KrakenError('injected marker interruption')
            return original(key, value)
        with patch.object(self.session.store, 'set', failing):
            await self.orch.run()
        await self.assert_recovered_without_model_retry()

    async def test_resume_after_completion_marker_before_task_state(self):
        original = self.session.store.task
        def failing(task, status, *args, **kwargs):
            if status == 'done':
                raise KrakenError('injected task state interruption')
            return original(task, status, *args, **kwargs)
        with patch.object(self.session.store, 'task', failing):
            await self.orch.run()
        self.assertFalse(self.session.store.pending_publications(self.orch.plan.revision))
        await self.assert_recovered_without_model_retry()

    async def test_divergent_destination_is_never_overwritten(self):
        with patch.object(self.orch.repo, 'finish_publication', side_effect=KrakenError('stop')):
            await self.orch.run()
        original = await git(self.root, 'rev-parse', 'HEAD')
        await git(self.root, 'update-ref', 'refs/heads/' + self.orch.repo.branch, original)
        with self.assertRaisesRegex(KrakenError, 'divergiu'):
            await self.orch.run()
        self.assertEqual(await git(self.root, 'rev-parse', self.orch.repo.branch), original)

    async def test_invalidated_evidence_prevents_recovery(self):
        with patch.object(self.orch.repo, 'finish_publication', side_effect=KrakenError('stop')):
            await self.orch.run()
        with self.session.store.db:
            self.session.store.db.execute('UPDATE evidence SET passed=0')
        with self.assertRaisesRegex(KrakenError, 'Evidência'):
            await self.orch.run()
        self.assertIsNone(await self.orch.repo.ref(self.root, self.orch.repo.branch))

    async def test_new_revision_revalidates_and_reuses_unchanged_completed_task(self):
        await self.orch.run()
        old_revision = self.orch.plan.revision
        second = replace(plan('TASK-2', 'second.txt').tasks[0], depends_on=('TASK-1',))
        new_plan = replace(self.orch.plan, scope='Both approved output files', tasks=(*self.orch.plan.tasks, second))
        approve(self.session, new_plan)
        self.assertEqual(self.session.store.tasks()[0]['status'], 'reuse_pending')
        fake = FakeModel(self.session.client)
        orch = Orchestrator(self.root, self.session.store, self.session.config, new_plan, fake)
        await orch.run()
        rows = {r['id']: r for r in self.session.store.tasks()}
        self.assertEqual(rows['TASK-1']['status'], 'done')
        self.assertEqual(rows['TASK-1']['attempt'], 0)
        self.assertEqual(rows['TASK-2']['status'], 'done')
        implementations = [messages[1]['content'] for role, messages in fake.calls if role == 'backend']
        self.assertTrue(all(__import__('json').loads(text.split('\nGuidance')[0])['contract']['id'] != 'TASK-1'
                            for text in implementations))
        events = [e for e in self.session.store.events(limit=2000) if e['kind'] == 'task_reused']
        self.assertEqual(events[-1]['payload']['source_revision'], old_revision)
        self.assertEqual(await git(self.root, 'show', orch.repo.branch + ':result.txt'), 'hello')

    async def test_junit_is_collected_from_this_container_before_removal(self):
        await self.orch.repo.prepare()
        check = replace(self.orch.plan.tasks[0].checks[0], runner='junit', report='/tmp/result.xml')
        result = await self.orch.executor.run(self.orch.repo.integration,
                     ['sh', '-c', 'printf \'<testsuite tests="1"><testcase name="fixture"/></testsuite>\' > /tmp/result.xml'],
                     readonly=True, capture_paths=(check.report,))
        self.assertTrue(normalize(check, result).ok)
        missing = await self.orch.executor.run(self.orch.repo.integration, ['true'], readonly=True,
                                              capture_paths=(check.report,))
        self.assertEqual(normalize(check, missing).status, 'unverified')

    async def test_changed_review_configuration_requires_fresh_candidate_evidence(self):
        from kraken.config import Config
        with patch.object(self.orch.repo, 'finish_publication', side_effect=KrakenError('stop')):
            await self.orch.run()
        folder = self.root / '.kraken/skills/qa'
        folder.mkdir(parents=True)
        (folder / 'SKILL.md').write_text('Review the same candidate with updated standards.')
        self.session.config = Config(self.root)
        approve(self.session, self.orch.plan)
        fake = FakeModel(self.session.client)
        resumed = Orchestrator(self.root, self.session.store, self.session.config, self.orch.plan, fake)
        await resumed.run()
        self.assertEqual(self.session.store.tasks()[0]['status'], 'done')
        self.assertTrue(fake.calls)
        self.assertTrue(all(name in {'qa', 'reviewer'} for name, _ in fake.calls))
        key = f'publication:{self.orch.plan.revision}:TASK-1:1'
        self.assertEqual(self.session.store.get(key)['config'], self.session.config.revision)

    async def test_skill_executable_requirement_checked_in_approved_image(self):
        from kraken.config import Config
        folder = self.root / '.kraken/skills/implementation'
        folder.mkdir(parents=True)
        (folder / 'SKILL.md').write_text('Implement the approved task.')
        (folder / 'skill.toml').write_text('required_commands=["kraken_missing_fixture_executable"]\n')
        self.session.config = Config(self.root)
        approve(self.session, self.orch.plan)
        fake = FakeModel(self.session.client)
        orch = Orchestrator(self.root, self.session.store, self.session.config, self.orch.plan, fake)
        with self.assertRaisesRegex(KrakenError, 'Executável exigido'):
            await orch.run()
        self.assertFalse(fake.calls)
        self.assertFalse((self.root / 'result.txt').exists())
