import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from kraken.config import Config
from kraken.domain import KrakenError


class SkillTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name).resolve()
        skill = self.root / '.kraken/skills/qa'
        skill.mkdir(parents=True)
        (skill / 'SKILL.md').write_text('Inspect independent evidence without modifying code.')
        self.path = skill / 'skill.toml'
        (self.root / '.kraken/project.toml').write_text('[provider]\nmodel="fixture"\napi_key_env=""\n')

    def tearDown(self):
        self.tmp.cleanup()

    def test_origin_version_and_requirements_are_in_approval_snapshot(self):
        self.path.write_text('source="https://github.com/example/skills"\nversion="commit-123"\nlicense="MIT"\nrequired_tools=["read_file"]\n')
        first = Config(self.root)
        meta = first.role('qa').snapshot()['requirements']
        self.assertEqual(meta['version'], 'commit-123')
        self.assertTrue(meta['declared'])
        self.path.write_text(self.path.read_text().replace('commit-123', 'commit-456'))
        self.assertNotEqual(Config(self.root).revision, first.revision)

    def test_unsupported_runtime_network_or_tool_never_grants_permission(self):
        for content in ('runtime="other-agent"', 'requires_network=true', 'required_tools=["write_file"]', 'required_unknown=true'):
            self.path.write_text(content)
            with self.assertRaises(KrakenError):
                Config(self.root)

    def test_required_variable_checked_by_name_without_logging_value(self):
        self.path.write_text('required_variables=["KRAKEN_FIXTURE_SETTING"]\n')
        config = Config(self.root)
        with patch.dict(os.environ, {'KRAKEN_FIXTURE_SETTING': ''}):
            with self.assertRaisesRegex(KrakenError, 'KRAKEN_FIXTURE_SETTING'):
                config.validate_environment({'qa'})
        with patch.dict(os.environ, {'KRAKEN_FIXTURE_SETTING': 'value-do-not-persist'}):
            config.validate_environment({'qa'})
            self.assertNotIn('value-do-not-persist', str(config.snapshot()))

    def test_manifest_is_optional_but_unknown_origin_is_explicit(self):
        meta = Config(self.root).role('qa').requirements
        self.assertEqual(meta['source'], 'local:qa')
        self.assertFalse(meta['declared'])
        self.assertTrue(meta['version'].startswith('sha256:'))
