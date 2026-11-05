"""K-01: compare HTTP streaming/cancellation without an API key or model call.

Run with Python >=3.12 and httpx==0.28.1. All requests target a local fixture.
This is a technical experiment, not the production adapter.
"""
import asyncio
import json
import threading
import time
import unittest
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import httpx


class Fixture(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        data = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        if self.path == '/error':
            self.send_error(429)
            return
        if self.path == '/slow':
            time.sleep(0.2)
        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream')
        self.end_headers()
        chunks = [
            {'choices': [{'delta': {'tool_calls': [
                {'index': 0, 'id': 'call_1', 'type': 'function',
                 'function': {'name': 'read_file', 'arguments': '{"pa'}},
            ]}}]},
            {'choices': [{'delta': {'tool_calls': [
                {'index': 0, 'function': {'arguments': 'th":"README.md"}'}},
            ]}}]},
            {'choices': [{'delta': {}, 'finish_reason': 'tool_calls'}]},
            {'choices': [], 'usage': {'prompt_tokens': 12, 'completion_tokens': 8}},
        ]
        if not data.get('stream'):
            chunks = []
        try:
            for chunk in chunks:
                # SSE comments and empty lines cannot be treated as model payloads.
                self.wfile.write(b': heartbeat\n\n')
                self.wfile.write(('data: ' + json.dumps(chunk) + '\n\n').encode())
                self.wfile.flush()
            self.wfile.write(b'data: [DONE]\n\n')
        except (BrokenPipeError, ConnectionResetError):
            pass


def collect(lines):
    """Small fixture parser, deliberately not the production implementation."""
    arguments, usage, finished = '', None, False
    for line in lines:
        if not line.startswith('data: '):
            continue
        if line == 'data: [DONE]':
            finished = True
            continue
        chunk = json.loads(line[6:])
        usage = chunk.get('usage', usage)
        for choice in chunk['choices']:
            for call in choice.get('delta', {}).get('tool_calls', []):
                arguments += call.get('function', {}).get('arguments', '')
    if not finished:
        raise ValueError('stream incompleto')
    return json.loads(arguments), usage


class TransportSpike(unittest.IsolatedAsyncioTestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(('127.0.0.1', 0), Fixture)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.url = f'http://127.0.0.1:{cls.server.server_port}'

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    async def test_httpx_stream_tool_arguments_and_usage(self):
        async with httpx.AsyncClient() as client:
            async with client.stream('POST', self.url, json={'stream': True}) as response:
                response.raise_for_status()
                args, usage = collect([line async for line in response.aiter_lines()])
        self.assertEqual(args, {'path': 'README.md'})
        self.assertEqual(usage, {'prompt_tokens': 12, 'completion_tokens': 8})

    async def test_standard_library_stream(self):
        def blocking():
            request = urllib.request.Request(self.url, data=b'{"stream": true}',
                                             headers={'Content-Type': 'application/json'})
            with urllib.request.urlopen(request, timeout=2) as response:
                return collect(line.decode().strip() for line in response)
        args, _ = await asyncio.to_thread(blocking)
        self.assertEqual(args, {'path': 'README.md'})

    async def test_httpx_read_timeout(self):
        async with httpx.AsyncClient(timeout=0.03) as client:
            with self.assertRaises(httpx.ReadTimeout):
                await client.post(self.url + '/slow', json={'stream': True})

    async def test_httpx_cancel_during_headers(self):
        async with httpx.AsyncClient() as client:
            task = asyncio.create_task(client.post(self.url + '/slow', json={'stream': True}))
            await asyncio.sleep(0.02)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await asyncio.wait_for(task, timeout=0.1)

    async def test_http_errors_are_not_success(self):
        async with httpx.AsyncClient() as client:
            response = await client.post(self.url + '/error', json={'stream': True})
            with self.assertRaises(httpx.HTTPStatusError):
                response.raise_for_status()

    async def test_incomplete_stream_is_not_success(self):
        with self.assertRaisesRegex(ValueError, 'incompleto'):
            collect(['data: {"choices": []}'])


if __name__ == '__main__':
    unittest.main()
