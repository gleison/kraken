from pathlib import Path
import tempfile
import unittest
from unittest.mock import AsyncMock, patch

from kraken.domain import KrakenError
from kraken.store import Store
from kraken.tools import ProjectReader, DockerExecutor
from kraken.provider import Budget
from kraken.process import Result


class InspectionTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        self.store = Store(self.root / 'runtime')
        self.reader = ProjectReader(self.root, self.store)

    async def asyncTearDown(self):
        self.store.close()
        self.tmp.cleanup()

    async def test_secret_and_symlink_escape_are_rejected(self):
        (self.root / '.env').write_text('secret')
        (self.root / 'escape').symlink_to('/etc/passwd')
        for path in ('.env', '../outside', 'escape'):
            with self.assertRaises(KrakenError):
                await self.reader.call('read_file', {'path': path})

    async def test_listing_root_and_reading_are_bounded(self):
        (self.root / 'README.md').write_text('x' * 70_000)
        listing = await self.reader.call('list_files', {'path': '.'})
        self.assertIn('README.md', listing['stdout'])
        result = await self.reader.call('read_file', {'path': 'README.md'})
        self.assertTrue(result['truncated'])
        self.assertEqual(len(result['stdout']), 64_000)
        with self.assertRaises(KrakenError):
            await self.reader.call('run_command', {'argv': ['touch', 'should-not-exist']})
        self.assertFalse((self.root / 'should-not-exist').exists())

    async def test_missing_engine_or_remote_context_blocks_without_host_fallback(self):
        executor = DockerExecutor(self.store, Budget(self.store), 'fixture')
        with patch('kraken.tools.os.getuid', return_value=0), patch('kraken.tools.process', new=AsyncMock()) as run:
            with self.assertRaisesRegex(KrakenError, 'não root'):
                await executor.preflight()
            run.assert_not_awaited()
        for result in (Result(1, '', 'unavailable', False), Result(0, 'tcp://remote:2375', '', False)):
            with patch('kraken.tools.process', new=AsyncMock(return_value=result)) as run:
                with self.assertRaises(KrakenError):
                    await executor.preflight()
                self.assertEqual(run.await_count, 1)
                self.assertEqual(run.call_args.args[0][:3], ['docker', '--context', 'default'])
