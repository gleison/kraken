from pathlib import Path
import tempfile
import unittest

from helpers import approve, plan, project
from kraken.domain import KrakenError
from kraken.projects import Catalog, active_id, sessions
from kraken.session import Session


class ProjectTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        await project(self.root)

    async def asyncTearDown(self):
        self.tmp.cleanup()

    async def test_new_and_resume_preserve_plan_approval_budget_and_history(self):
        first = Session(self.root)
        identity = first.selection_id
        p = approve(first)
        first.store.set('calls', 7)
        first.store.emit('user_checkpoint', content='keep this')
        first.close()
        second = Session(self.root, session_id='new')
        self.assertNotEqual(second.selection_id, identity)
        self.assertIsNone(second.plan)
        self.assertEqual(second.store.get('calls'), 0)
        second.close()
        resumed = Session(self.root, session_id=identity)
        try:
            self.assertEqual(resumed.plan.revision, p.revision)
            resumed.store.assert_approved(p, resumed.config.revision)
            self.assertEqual(resumed.store.get('calls'), 7)
            self.assertTrue(any(e['kind'] == 'user_checkpoint' for e in resumed.store.events()))
            self.assertEqual(len(sessions(self.root)), 2)
        finally:
            resumed.close()

    async def test_session_switch_cannot_bypass_existing_project_owner(self):
        first = Session(self.root)
        try:
            p = approve(first)
            with self.assertRaises(KrakenError):
                Session(self.root, session_id='new')
            self.assertEqual(active_id(self.root), first.selection_id)
            self.assertEqual(first.plan.revision, p.revision)
        finally:
            first.close()

    async def test_catalog_is_local_and_does_not_start_session(self):
        catalog = Catalog(self.root / 'catalog')
        catalog.add(self.root)
        catalog.add(self.root)
        self.assertEqual(catalog.projects(), [self.root])
        self.assertEqual(sessions(self.root), [])

    async def test_unknown_session_does_not_destroy_active_plan(self):
        first = Session(self.root)
        p = approve(first)
        identity = first.selection_id
        first.close()
        with self.assertRaises(KrakenError):
            Session(self.root, session_id='aaaaaaaaaaaa')
        resumed = Session(self.root)
        try:
            self.assertEqual(resumed.selection_id, identity)
            self.assertEqual(resumed.plan.revision, p.revision)
        finally:
            resumed.close()


    async def test_catalog_canonicalizes_an_alias_without_duplicate_project(self):
        alias = self.root / 'project-alias'
        alias.symlink_to(self.root, target_is_directory=True)
        catalog = Catalog(self.root / '.kraken/catalog')
        catalog.add(alias)
        catalog.add(self.root)
        self.assertEqual(catalog.projects(), [self.root.resolve()])
