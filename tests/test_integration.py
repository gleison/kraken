import asyncio
from dataclasses import replace
import os
from pathlib import Path
import tempfile
import unittest

from kraken.config import Config
from kraken.domain import KrakenError
from kraken.git import git
from kraken.orchestrator import Orchestrator
from kraken.session import Session
from helpers import FakeModel, approve, plan, project


@unittest.skipUnless(os.environ.get('KRAKEN_TEST_DOCKER') == '1', 'Requires explicit local Docker fixture opt-in')
class IntegrationTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        await project(self.root)
        self.original = await git(self.root, 'rev-parse', 'HEAD')
        self.session = Session(self.root)

    async def asyncTearDown(self):
        self.session.close()
        self.tmp.cleanup()

    def orchestrator(self, p=None, **kwargs):
        p = approve(self.session, p)
        fake = FakeModel(self.session.client, **kwargs)
        return Orchestrator(self.root, self.session.store, self.session.config, p, fake), fake

    async def test_real_container_and_git_roundtrip_preserves_original_checkout(self):
        orch, fake = self.orchestrator()
        await orch.run()
        self.assertEqual(self.session.store.tasks()[0]['status'], 'done')
        self.assertEqual(await git(self.root, 'rev-parse', 'HEAD'), self.original)
        self.assertFalse((self.root / 'result.txt').exists())
        self.assertEqual(await git(self.root, 'show', orch.repo.branch + ':result.txt'), 'hello')
        self.assertEqual(await git(self.root, 'remote'), '')
        kinds = [e['payload'].get('kind') for e in self.session.store.events() if e['kind'] == 'evidence']
        self.assertIn('qa', kinds)
        self.assertIn('reviewer', kinds)
        self.assertIn('computational', kinds)

    async def test_unapproved_plan_never_runs_or_probes(self):
        p = plan()
        self.session.save_plan(p)
        fake = FakeModel(self.session.client)
        orch = Orchestrator(self.root, self.session.store, self.session.config, p, fake)
        with self.assertRaises(KrakenError):
            await orch.run()
        self.assertFalse(fake.probed)
        self.assertFalse(orch.repo.integration.exists())

    async def test_changed_plan_file_blocks_execution(self):
        orch, fake = self.orchestrator()
        changed = replace(plan(), scope='unapproved change')
        (self.root / '.kraken/plan.json').write_text(__import__('json').dumps(changed.to_dict()))
        with self.assertRaises(KrakenError):
            await orch.run()
        self.assertFalse(fake.probed)

    async def test_independent_qa_failure_produces_bounded_correction(self):
        orch, fake = self.orchestrator(qa_failures=1)
        await orch.run()
        row = self.session.store.tasks()[0]
        self.assertEqual(row['status'], 'done')
        self.assertEqual(row['attempt'], 2)
        self.assertTrue(any(e['kind'] == 'correction_feedback' for e in self.session.store.events()))

    async def test_two_independent_tasks_then_dependent(self):
        p = plan()
        first = p.tasks[0]
        second = plan('TASK-2', 'second.txt').tasks[0]
        third = replace(plan('TASK-3', 'third.txt').tasks[0], depends_on=('TASK-1', 'TASK-2'))
        orch, fake = self.orchestrator(replace(p, tasks=(first, second, third)))
        await orch.run()
        self.assertTrue(all(r['status'] == 'done' for r in self.session.store.tasks()))
        events = self.session.store.events(limit=1000)
        started = {e['payload']['task']: e['seq'] for e in events if e['kind'] == 'task_started'}
        done = {e['payload']['task']: e['seq'] for e in events
                if e['kind'] == 'task_state' and e['payload']['status'] == 'done'}
        self.assertLess(started['TASK-2'], done['TASK-1'])
        self.assertGreater(started['TASK-3'], max(done['TASK-1'], done['TASK-2']))

    async def test_dirty_user_source_is_preserved_and_blocks_run(self):
        orch, fake = self.orchestrator()
        (self.root / 'user.txt').write_text('must survive')
        with self.assertRaises(KrakenError):
            await orch.run()
        self.assertEqual((self.root / 'user.txt').read_text(), 'must survive')
        self.assertFalse(fake.probed)

    async def test_cancel_blocks_uncertain_attempt_without_success(self):
        orch, fake = self.orchestrator(delay=0.3)
        task = asyncio.create_task(orch.run())
        for _ in range(200):
            if orch.active:
                break
            await asyncio.sleep(0.01)
        orch.cancel()
        await task
        self.assertEqual(self.session.store.get('control'), 'cancelled')
        self.assertEqual(self.session.store.tasks()[0]['status'], 'blocked')
        await self.session.retry('TASK-1')
        fake.delay = 0
        await orch.run()
        self.assertEqual(self.session.store.tasks()[0]['status'], 'done')
        self.assertEqual(self.session.store.tasks()[0]['attempt'], 2)

    async def test_zero_test_run_cannot_complete_task(self):
        p = plan()
        check = replace(p.tasks[0].checks[0], argv=('sh', '-c', 'echo "Ran 0 tests"'))
        task = replace(p.tasks[0], checks=(check,))
        orch, fake = self.orchestrator(replace(p, tasks=(task,), limits=replace(p.limits, attempts=1)))
        await orch.run()
        self.assertEqual(self.session.store.tasks()[0]['status'], 'blocked')
        self.assertEqual(await git(orch.repo.integration, 'rev-parse', 'HEAD'), self.original)

    async def test_changed_role_configuration_blocks_execution(self):
        orch, fake = self.orchestrator()
        (self.root / '.kraken/project.toml').write_text('[provider]\nmodel="changed"\napi_key_env=""\n')
        with self.assertRaises(KrakenError):
            await orch.run()
        self.assertFalse(fake.probed)

    async def test_pause_allows_inflight_to_finish_without_new_dispatch(self):
        p = plan()
        dependent = replace(plan('TASK-2', 'second.txt').tasks[0], depends_on=('TASK-1',))
        orch, fake = self.orchestrator(replace(p, tasks=(*p.tasks, dependent)), delay=0.03)
        task = asyncio.create_task(orch.run())
        for _ in range(200):
            if orch.active:
                break
            await asyncio.sleep(0.01)
        orch.pause()
        await task
        rows = {r['id']: r for r in self.session.store.tasks()}
        self.assertEqual(self.session.store.get('control'), 'paused')
        self.assertEqual(rows['TASK-1']['status'], 'done')
        self.assertEqual(rows['TASK-2']['attempt'], 0)
        await orch.run()
        self.assertEqual(self.session.store.get('control'), 'done')


    async def test_integration_cannot_regress_a_completed_task(self):
        from hashlib import sha256
        (self.root / 'alternate.txt').write_text('bye\n')
        await git(self.root, 'add', '--', 'alternate.txt')
        await git(self.root, 'commit', '-qm', 'alternate fixture')
        p = plan()
        second = replace(p.tasks[0], id='TASK-2', title='An intentionally conflicting request',
                         depends_on=('TASK-1',), checks=(replace(p.tasks[0].checks[0],
                         argv=('sh', '-c', 'cmp result.txt alternate.txt')),))
        fixtures = p.fixtures | {'alternate.txt': sha256(b'bye\n').hexdigest()}
        p = replace(p, tasks=(*p.tasks, second), fixtures=fixtures, limits=replace(p.limits, attempts=1))
        orch, fake = self.orchestrator(p, outputs={'TASK-2': 'bye'})
        await orch.run()
        rows = {r['id']: r for r in self.session.store.tasks()}
        self.assertEqual(rows['TASK-1']['status'], 'done')
        self.assertEqual(rows['TASK-2']['status'], 'blocked')
        self.assertEqual(await git(self.root, 'show', orch.repo.branch + ':result.txt'), 'hello')

    async def test_portable_write_tool_and_per_command_approval_gate(self):
        from kraken.tools import Tools
        orch, fake = self.orchestrator()
        await orch.executor.preflight()
        await orch.repo.prepare()
        workspace, _ = await orch.repo.workspace(orch.plan.tasks[0], 1)
        tools = Tools(orch.executor, workspace, orch.plan.tasks[0])
        result = await tools.call('write_file', {'path': 'result.txt', 'content': 'hello\n'})
        self.assertEqual(result['code'], 0)
        self.assertEqual((workspace / 'result.txt').read_text(), 'hello\n')
        changed = replace(plan(), scope='not approved')
        (self.root / '.kraken/plan.json').write_text(__import__('json').dumps(changed.to_dict()))
        with self.assertRaises(KrakenError):
            await tools.call('write_file', {'path': 'result.txt', 'content': 'unauthorized change'})
        self.assertEqual((workspace / 'result.txt').read_text(), 'hello\n')
