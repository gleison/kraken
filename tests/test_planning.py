from kraken.domain import PLAN_REVIEW_CRITERIA
import ast
from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

from kraken.domain import KrakenError
from kraken.provider import Reply
from kraken.session import Session
from helpers import plan, project


class PlanningTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        await project(self.root)
        self.session = Session(self.root)

    async def asyncTearDown(self):
        self.session.close()
        self.tmp.cleanup()

    async def test_interview_plan_fresh_reviews_and_explicit_approval(self):
        calls = []
        async def complete(role, messages, tools=()):
            calls.append((role.name, messages))
            if len(calls) == 1:
                return Reply(json.dumps({'question': 'Must output exactly hello?', 'recommendation': 'Yes'}), [], None)
            if role.name == 'product':
                return Reply(json.dumps({'plan': plan().to_dict()}), [], None)
            return Reply(json.dumps({'passed': True, 'feedback': 'Fixture is self-contained', 'criteria': list(PLAN_REVIEW_CRITERIA)}), [], None)
        self.session.client.complete = complete
        question = await self.session.ask('Build the requested fixture output')
        self.assertIn('hello', question)
        await self.session.ask('Yes; counter-only budget is accepted for this fixture')
        self.assertIsNone(self.session.store.get('approved'))
        await self.session.review()
        self.assertIsNone(self.session.store.get('approved'))
        review_prompts = [messages[1]['content'] for name, messages in calls if name in {'qa', 'reviewer'}]
        self.assertEqual(len(review_prompts), 2)
        self.assertTrue(all('"history"' not in prompt for prompt in review_prompts))
        self.assertTrue(all('Must output exactly hello?' not in prompt for prompt in review_prompts))
        self.session.approve(self.session.plan.revision, self.session.config.revision)
        self.assertIsNotNone(self.session.store.get('approved'))

    async def test_plan_reviewer_finding_prevents_approval(self):
        self.session.save_plan(plan())
        async def complete(role, messages, tools=()):
            return Reply(json.dumps({'passed': role.name != 'qa', 'feedback': 'C1 is ambiguous', 'criteria': list(PLAN_REVIEW_CRITERIA)}), [], None)
        self.session.client.complete = complete
        await self.session.review()
        with self.assertRaises(KrakenError):
            self.session.approve(self.session.plan.revision, self.session.config.revision)

    async def test_manual_plan_revision_invalidates_old_review(self):
        p = plan()
        self.session.save_plan(p)
        for role in ('qa', 'reviewer'):
            self.session.store.review(p, self.session.config.revision, role, {'passed': True, 'feedback': 'Contract reviewed', 'criteria': list(PLAN_REVIEW_CRITERIA)})
        changed = replace(p, scope='New scope')
        self.session.save_plan(changed)
        with self.assertRaises(KrakenError):
            self.session.approve(changed.revision, self.session.config.revision)


class ArchitectureTests(unittest.TestCase):
    def test_core_does_not_import_tui_framework(self):
        import kraken
        root = Path(kraken.__file__).parent
        for path in root.glob('*.py'):
            if path.name in {'ui.py', '__main__.py'}:
                continue
            tree = ast.parse(path.read_text())
            for node in ast.walk(tree):
                if isinstance(node, ast.ImportFrom):
                    self.assertFalse((node.module or '').startswith(('textual', 'kraken.ui')), path.name)
                elif isinstance(node, ast.Import):
                    self.assertFalse(any(x.name.startswith('textual') for x in node.names), path.name)
