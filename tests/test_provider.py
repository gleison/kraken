import asyncio
from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

import httpx
from kraken.config import Profile, Role
from kraken.domain import KrakenError, Limits
from kraken.provider import Budget, ModelClient
from kraken.store import Store


class ProviderTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.store = Store(Path(self.tmp.name))
        self.role = Role('backend', Profile('http://localhost/v1', 'fixture', ''), (), 'fixture', '', 'hash')

    async def asyncTearDown(self):
        self.store.close()
        self.tmp.cleanup()

    def stream(self, events, done=True):
        text = ': comment\n\n' + ''.join('data: ' + json.dumps(e) + '\n\n' for e in events)
        return text + ('data: [DONE]\n\n' if done else '')

    async def run_stream(self, events, *, done=True):
        payload = self.stream(events, done)
        async def handle(request):
            self.assertEqual(request.url.path, '/v1/chat/completions')
            self.assertNotIn('authorization', request.headers)
            return httpx.Response(200, content=payload, headers={'Content-Type': 'text/event-stream'})
        async with httpx.AsyncClient(transport=httpx.MockTransport(handle)) as client:
            adapter = ModelClient(self.store, Budget(self.store), client)
            return await adapter.complete(self.role, [{'role': 'user', 'content': 'fixture'}])

    async def test_interleaved_calls_fragmented_arguments_and_usage(self):
        reply = await self.run_stream([
            {'choices': [{'delta': {'tool_calls': [
                {'index': 0, 'id': 'a', 'function': {'name': 'read_file', 'arguments': '{"path":'}},
                {'index': 1, 'id': 'b', 'function': {'name': 'list_files', 'arguments': '{"path":'}},
            ]}}]},
            {'choices': [{'delta': {'tool_calls': [
                {'index': 1, 'function': {'arguments': '"src"}'}},
                {'index': 0, 'function': {'arguments': '"README.md"}'}},
            ]}, 'finish_reason': 'tool_calls'}]},
            {'choices': [], 'usage': {'prompt_tokens': 9, 'completion_tokens': 7}},
        ])
        self.assertEqual([c['id'] for c in reply.tool_calls], ['a', 'b'])
        self.assertEqual(json.loads(reply.tool_calls[0]['function']['arguments']), {'path': 'README.md'})
        self.assertEqual(self.store.get('usage')['completion_tokens'], 7)

    async def test_truncated_stream_rejected(self):
        with self.assertRaises(KrakenError):
            await self.run_stream([{'choices': [{'delta': {'content': 'looks good'}}]}], done=False)

    async def test_length_stop_rejected(self):
        with self.assertRaises(KrakenError):
            await self.run_stream([{'choices': [{'delta': {}, 'finish_reason': 'length'}]}])

    async def test_bad_tool_json_rejected_before_execution(self):
        with self.assertRaises(KrakenError):
            await self.run_stream([{'choices': [{'delta': {'tool_calls': [
                {'index': 0, 'id': 'a', 'function': {'name': 'run_command', 'arguments': 'broken'}}
            ]}, 'finish_reason': 'tool_calls'}]}])

    async def test_unknown_usage_is_not_reported_as_zero_cost(self):
        reply = await self.run_stream([{'choices': [{'delta': {'content': 'ok'}, 'finish_reason': 'stop'}]}])
        self.assertIsNone(reply.usage)
        self.assertEqual(self.store.get('unknown_usage_calls'), 1)

    async def test_cancellation_is_propagated_without_retry(self):
        calls = 0
        async def handle(request):
            nonlocal calls
            calls += 1
            await asyncio.sleep(60)
        async with httpx.AsyncClient(transport=httpx.MockTransport(handle)) as client:
            adapter = ModelClient(self.store, Budget(self.store), client)
            task = asyncio.create_task(adapter.complete(self.role, [{'role': 'user', 'content': 'fixture'}]))
            await asyncio.sleep(0.01)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task
        self.assertEqual(calls, 1)
        self.assertTrue(any(e['kind'] == 'model_uncertain' for e in self.store.events()))


    async def test_configured_prices_produce_labeled_estimate(self):
        self.role = replace(self.role, profile=replace(self.role.profile,
                            input_usd_per_million=1.0, output_usd_per_million=2.0))
        await self.run_stream([
            {'choices': [{'delta': {'content': 'ok'}, 'finish_reason': 'stop'}]},
            {'choices': [], 'usage': {'prompt_tokens': 9, 'completion_tokens': 7}},
        ])
        self.assertAlmostEqual(self.store.get('estimated_cost_usd'), 23 / 1_000_000)
        self.assertAlmostEqual(self.store.get('reserved_cost_usd'), 0)
        self.assertFalse(self.store.get('cost_unavailable'))
