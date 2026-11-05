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
        self.root = Path(self.tmp.name)
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
