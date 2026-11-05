import asyncio
import sys
import unittest
from unittest.mock import patch

from kraken.process import process


class ProcessTests(unittest.IsolatedAsyncioTestCase):
    async def test_cancel_during_spawn_keeps_ownership_and_reaps_child(self):
        original = asyncio.create_subprocess_exec
        created, children = asyncio.Event(), []

        async def delayed(*args, **kwargs):
            proc = await original(*args, **kwargs)
            children.append(proc)
            created.set()
            await asyncio.sleep(0.03)
            return proc

        with patch('kraken.process.asyncio.create_subprocess_exec', delayed):
            task = asyncio.create_task(process([sys.executable, '-c', 'import time; time.sleep(30)']))
            await created.wait()
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await asyncio.wait_for(task, 2)
        self.assertIsNotNone(children[0].returncode)

    async def test_truncation_preserves_real_failure_status(self):
        result = await process([sys.executable, '-c', 'import sys; print("x"*10000); sys.exit(7)'], limit=100)
        self.assertEqual(result.code, 7)
        self.assertTrue(result.truncated)
        self.assertEqual(len(result.stdout), 100)
