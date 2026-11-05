from kraken.domain import PLAN_REVIEW_CRITERIA
from pathlib import Path
import tempfile
import unittest

from kraken.config import Config
from kraken.session import Session
from kraken.ui import KrakenApp
from textual.widgets import Button, DataTable
from helpers import approve, plan, project


class UITests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        await project(self.root)
        self.session = Session(self.root)

    async def asyncTearDown(self):
        self.session.close()
        self.tmp.cleanup()

    async def test_startup_does_not_call_api_or_approve(self):
        self.session.save_plan(plan())
        app = KrakenApp(self.session)
        async with app.run_test(size=(80, 24)) as pilot:
            await pilot.pause()
            self.assertEqual(self.session.store.get('calls'), 0)
            self.assertIsNone(self.session.store.get('approved'))
            await pilot.resize_terminal(50, 20)
            await pilot.pause()
            self.assertEqual(app.size.width, 50)

    async def test_approval_requires_confirmation_modal(self):
        p = plan()
        self.session.save_plan(p)
        for role in ('qa', 'reviewer'):
            self.session.store.review(p, self.session.config.revision, role,
                                      {'passed': True, 'feedback': 'reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
        app = KrakenApp(self.session)
        async with app.run_test(size=(160, 45)) as pilot:
            await pilot.click('#approve')
            await pilot.pause()
            self.assertIsNone(self.session.store.get('approved'))
            # The long plan deliberately scrolls; focus the explicit confirmation control.
            confirm = app.screen.query_one('#confirm', Button)
            confirm.focus()
            await pilot.press('enter')
            await pilot.pause()
            self.assertEqual(self.session.store.get('approved')['revision'], p.revision)
            self.assertEqual(self.session.store.get('calls'), 0)

    async def test_dependencies_and_persisted_evidence_remain_inspectable_without_events(self):
        from dataclasses import replace
        from textual.widgets import RichLog, TabbedContent
        p = plan()
        second = replace(plan('TASK-2', 'second.txt').tasks[0], depends_on=('TASK-1',))
        self.session.save_plan(replace(p, tasks=(*p.tasks, second)))
        identity = self.session.store.evidence('TASK-2', 1, 'commit-fixture', 'qa', False,
                                              {'feedback': 'Actual persisted finding'})
        with self.session.store.db:
            self.session.store.db.execute('DELETE FROM events')
        app = KrakenApp(self.session)
        async with app.run_test(size=(100, 30)) as pilot:
            app.query_one(TabbedContent).active = 'tasks-tab'
            await pilot.pause()
            table = app.query_one('#tasks', DataTable)
            self.assertEqual(table.get_row('TASK-2')[1], 'TASK-1')
            table.move_cursor(row=1)
            await pilot.click('#inspect-evidence')
            await pilot.pause()
            self.assertEqual(app.screen.records[0]['id'], identity)
            self.assertTrue(app.screen.query_one('#evidence-records', RichLog).lines)
            await pilot.resize_terminal(50, 20)
            await pilot.pause()
            await pilot.click('#evidence-close')
            self.assertEqual(self.session.store.get('calls'), 0)

    async def test_project_manager_lists_sessions_and_returns_selection_without_running(self):
        from kraken.projects import Catalog
        from kraken.ui import ProjectLauncher
        catalog = Catalog(self.root / '.kraken/catalog')
        catalog.add(self.root)
        expected_id = self.session.selection_id
        self.session.close()
        launcher = ProjectLauncher(catalog)
        async with launcher.run_test(size=(140, 40)) as pilot:
            await pilot.pause()
            self.assertEqual(launcher.screen.query_one('#session-list', DataTable).row_count, 1)
            await pilot.click('#manager-resume')
            await pilot.pause()
        self.assertEqual(launcher.return_value, (str(self.root), expected_id))
        self.session = Session(self.root, session_id=expected_id)
        self.assertEqual(self.session.store.get('calls'), 0)

    async def test_main_tui_can_request_new_session_without_implicit_api(self):
        from kraken.projects import Catalog
        catalog = Catalog(self.root / '.kraken/catalog')
        catalog.add(self.root)
        app = KrakenApp(self.session, catalog=catalog)
        async with app.run_test(size=(160, 45)) as pilot:
            await pilot.click('#manage')
            await pilot.pause()
            await pilot.click('#manager-new')
            await pilot.pause()
        self.assertEqual(app.return_value, (str(self.root), 'new'))
        self.assertEqual(self.session.store.get('calls'), 0)
