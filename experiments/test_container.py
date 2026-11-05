"""K-01 Docker isolation experiment. No project code, models or credentials.

Requires the exact verified image below. Pull it explicitly before running.
"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

IMAGE = 'busybox@sha256:a7dbc319b1438115a207e11930dc4bb3e643dcb3ec0bdd694bb36cd8fad289ec'


class ContainerSpike(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='kraken-k01-')
        self.root = Path(self.tmp.name)
        self.workspace = self.root / 'workspace'
        self.workspace.mkdir()
        self.reference = self.root / 'reference'
        self.reference.mkdir()
        (self.reference / 'contract').write_text('approved fixture')
        self.env = {k: v for k, v in os.environ.items() if k not in {
            'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_TLS', 'DOCKER_TLS_VERIFY', 'DOCKER_CERT_PATH'}}
        self.docker = ['docker', '--host=unix:///var/run/docker.sock']

    def tearDown(self):
        self.tmp.cleanup()

    def run_command(self, script):
        return subprocess.run([
            *self.docker, 'run', '--rm', '--pull=never', '--network=none', '--read-only',
            '--cap-drop=ALL', '--security-opt=no-new-privileges', '--pids-limit=32',
            '--memory=64m', '--cpus=0.5', '--user', f'{os.getuid()}:{os.getgid()}',
            '--mount', f'type=bind,src={self.workspace},dst=/workspace',
            '--mount', f'type=bind,src={self.reference},dst=/reference,readonly',
            '--workdir=/workspace', IMAGE, 'sh', '-c', script,
        ], env=self.env, capture_output=True, text=True, timeout=30)

    def test_only_loopback_interface(self):
        result = self.run_command('ls /sys/class/net')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), 'lo')

    def test_workspace_write_persists(self):
        result = self.run_command('echo result > artifact')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.workspace / 'artifact').read_text(), 'result\n')

    def test_reference_is_readonly(self):
        result = self.run_command('echo changed > /reference/contract')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.reference / 'contract').read_text(), 'approved fixture')

    def test_root_filesystem_is_readonly(self):
        result = self.run_command('touch /etc/kraken-fixture')
        self.assertNotEqual(result.returncode, 0)

    def test_no_docker_socket(self):
        result = self.run_command('test ! -e /var/run/docker.sock')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_capabilities_dropped(self):
        result = self.run_command('cat /proc/self/status')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('CapEff:\t0000000000000000', result.stdout)

    def test_no_container_left_running(self):
        # Verify --rm for an isolated container without touching other workloads.
        name = f'kraken-k01-{os.getpid()}'
        result = subprocess.run([*self.docker, 'run', '--rm', '--pull=never',
                                 '--network=none', '--name', name, IMAGE, 'true'],
                                env=self.env, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        inspect = subprocess.run([*self.docker, 'inspect', name], env=self.env,
                                 capture_output=True, text=True, timeout=10)
        self.assertNotEqual(inspect.returncode, 0)


if __name__ == '__main__':
    unittest.main()
