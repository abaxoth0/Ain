import re
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]


def load_config(path):
    return yaml.load((ROOT / path).read_text(), Loader=yaml.BaseLoader)


class WorkflowTests(unittest.TestCase):
    def setUp(self):
        self.ci = load_config('.github/workflows/ci.yml')
        self.release = load_config('.github/workflows/release.yml')

    def test_development_pushes_and_pull_requests_run_ci(self):
        events = self.ci['on']
        self.assertTrue({'main', 'dev'} <= set(events['push']['branches']))
        self.assertIn('pull_request', events)
        self.assertFalse((events['pull_request'] or {}).get('branches-ignore'))

    def test_release_runs_all_ci_checks_before_publishing(self):
        self.assertIn('workflow_call', self.ci['on'])
        publish = self.release['jobs']['release']
        needs = publish['needs']
        if isinstance(needs, str):
            needs = [needs]
        self.assertTrue(any(
            self.release['jobs'][job].get('uses') == './.github/workflows/ci.yml'
            for job in needs
        ))
        self.assertTrue({'test', 'lint', 'build', 'security'} <= self.ci['jobs'].keys())
        for job in self.ci['jobs'].values():
            self.assertNotEqual(job.get('continue-on-error'), 'true')
            for step in job.get('steps', []):
                self.assertNotEqual(step.get('continue-on-error'), 'true')

    def test_release_can_publish_with_full_history(self):
        publish = self.release['jobs']['release']
        permissions = publish.get('permissions', self.release.get('permissions', {}))
        self.assertEqual(permissions.get('contents'), 'write')
        checkout = next(step for step in publish['steps']
                        if step.get('uses', '').startswith('actions/checkout@'))
        self.assertEqual(checkout['with']['fetch-depth'], '0')

    def test_library_release_does_not_require_a_binary(self):
        config = load_config('.goreleaser.yml')
        self.assertTrue(config['builds'])
        for build in config['builds']:
            self.assertEqual(build.get('skip'), 'true')
        for job in self.ci['jobs'].values():
            for step in job.get('steps', []):
                for target in re.findall(r'go build[^\n]*?(\./[\w/-]+)', step.get('run', '')):
                    self.assertTrue((ROOT / target).is_dir(), f'Missing build target: {target}')

    def test_lint_action_supports_the_pinned_linter(self):
        step = next(step for step in self.ci['jobs']['lint']['steps']
                    if step.get('uses', '').startswith('golangci/golangci-lint-action@'))
        action_major = int(step['uses'].rsplit('@v', 1)[1].split('.')[0])
        self.assertRegex(step['with']['version'], r'^v2\.\d+\.\d+$')
        self.assertGreaterEqual(action_major, 7)

    def test_slsa_builders_have_the_required_configuration(self):
        for path in (ROOT / '.github/workflows').glob('*.yml'):
            workflow = load_config(path)
            for job in workflow['jobs'].values():
                if 'builder_go_slsa3.yml@' in job.get('uses', ''):
                    self.assertTrue((ROOT / '.slsa-goreleaser.yml').is_file())


if __name__ == '__main__':
    unittest.main()
