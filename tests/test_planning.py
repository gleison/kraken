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
        with (self.root / '.kraken/project.toml').open('a') as file:
            file.write('\n[planning]\nmax_review_rounds=1\n')
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


    async def test_review_repairs_technical_findings_and_rechecks_with_fresh_context(self):
        original = plan()
        self.session.save_plan(original)
        fixed = replace(original, tasks=(replace(original.tasks[0], title='Clarified fixture contract'),))
        reviews = []
        async def complete(role, messages, tools=()):
            payload = json.loads(messages[1]['content'].split('\nReturn')[0])
            if role.name == 'product':
                self.assertIn('findings', payload)
                return Reply(json.dumps({'plan': fixed.to_dict()}), [], None)
            current = payload['plan']
            reviews.append((role.name, current))
            passed = current['tasks'][0]['title'] == fixed.tasks[0].title
            return Reply(json.dumps({'passed': passed, 'feedback': 'Title needs clarification' if not passed else 'Clear',
                                     'criteria': list(PLAN_REVIEW_CRITERIA)}), [], None)
        self.session.client.complete = complete
        result = await self.session.review()
        self.assertIn('2 rodada', result)
        self.assertEqual(len(reviews), 4)
        self.assertEqual(self.session.plan.revision, fixed.revision)
        self.assertIsNone(self.session.store.get('approved'))
        self.session.approve(fixed.revision, self.session.config.revision)

    async def test_review_requests_only_missing_human_decision(self):
        self.session.save_plan(plan())
        calls = []
        async def complete(role, messages, tools=()):
            calls.append(role.name)
            return Reply(json.dumps({'passed': False, 'feedback': 'Intent missing',
                 'criteria': list(PLAN_REVIEW_CRITERIA), 'needs_decision': True,
                 'question': 'Which behaviour should win?', 'recommendation': 'Keep approved fixture'}), [], None)
        self.session.client.complete = complete
        self.assertIn('Which behaviour', await self.session.review())
        self.assertNotIn('product', calls)
        self.assertIsNotNone(self.session.store.get('planning_decision'))

    async def test_automatic_repair_cannot_change_scope_or_budget(self):
        p = plan()
        self.session.save_plan(p)
        async def complete(role, messages, tools=()):
            if role.name == 'product':
                return Reply(json.dumps({'plan': replace(p, scope='expanded without consent').to_dict()}), [], None)
            return Reply(json.dumps({'passed': False, 'feedback': 'Fix it', 'criteria': list(PLAN_REVIEW_CRITERIA)}), [], None)
        self.session.client.complete = complete
        with self.assertRaisesRegex(KrakenError, 'escopo'):
            await self.session.review()
        self.assertEqual(self.session.plan.revision, p.revision)

    async def test_review_round_limit_keeps_plan_unapproved(self):
        p = plan()
        self.session.save_plan(p)
        fixes = 0
        async def complete(role, messages, tools=()):
            nonlocal fixes
            if role.name == 'product':
                fixes += 1
                return Reply(json.dumps({'plan': replace(p, tasks=(replace(p.tasks[0], title=f'Clarified {fixes}'),)).to_dict()}), [], None)
            return Reply(json.dumps({'passed': False, 'feedback': 'Still ambiguous', 'criteria': list(PLAN_REVIEW_CRITERIA)}), [], None)
        self.session.client.complete = complete
        self.assertIn('Limite', await self.session.review())
        self.assertEqual(fixes, 2)
        self.assertIsNone(self.session.store.get('approved'))

    async def test_unapproved_draft_cannot_raise_planning_api_budget(self):
        p = replace(plan(), limits=replace(plan().limits, calls=1000))
        self.session.save_plan(p)
        self.session.store.set('calls', 100)
        with self.assertRaisesRegex(KrakenError, 'Limite de chamadas'):
            await self.session.review()
        self.assertEqual(self.session.budget.limits.calls, 100)
        self.assertIsNone(self.session.store.get('approved'))


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
