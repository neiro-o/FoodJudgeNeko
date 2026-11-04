import argparse
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('manage', Path(__file__).parents[1] / 'manage.py')
manage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(manage)
TEMPLATE = Path(__file__).parents[1] / '.env.example'


class DeploymentSafetyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.patch = mock.patch.multiple(manage, ROOT=self.root, ENV=self.root / '.env')
        self.patch.start()
        self.addCleanup(self.patch.stop)
        self.addCleanup(self.temp.cleanup)
        self.values = manage.read_env(TEMPLATE)
        for key in ('MONGO_ROOT_PASSWORD', 'REDIS_PASSWORD', 'ELASTIC_PASSWORD', 'MTV2_JWT_SECRET'):
            self.values[key] = 'a' * 64
        self.values.update(COMPOSE_PROFILES='', MONGO_MODE='existing', REDIS_MODE='existing', ES_MODE='existing')
        manage.write_env(manage.ENV, self.values)
        (self.root / '.env.example').write_text(TEMPLATE.read_text())

    def test_dotenv_values_are_literal_and_private(self):
        self.values['MTV2_DEEPSEEK_KEY'] = "a$SECRET'$(touch /tmp/should-not-exist)#suffix"
        manage.write_env(manage.ENV, self.values)
        self.assertEqual(manage.read_env()['MTV2_DEEPSEEK_KEY'], self.values['MTV2_DEEPSEEK_KEY'])
        self.assertEqual(manage.ENV.stat().st_mode & 0o777, 0o600)
        self.values['MTV2_DEEPSEEK_KEY'] = 'bad\nINJECTED=1'
        with self.assertRaises(ValueError):
            manage.write_env(manage.ENV, self.values)

    @mock.patch.object(manage, 'compose')
    def test_existing_databases_are_never_initialized(self, compose):
        manage.initialize_managed(self.values)
        compose.assert_not_called()

    @mock.patch.object(manage, 'compose')
    def test_only_selected_new_databases_initialized_once(self, compose):
        self.values.update(COMPOSE_PROFILES='mongo', MONGO_MODE='docker')
        manage.initialize_managed(self.values)
        manage.initialize_managed(self.values)
        compose.assert_called_once_with('run', '--rm', '--no-deps', 'api', '/app/mtv2ctl', 'init-mongo')
        self.assertTrue((self.root / '.initialized-mongo').exists())

    @mock.patch.object(manage, 'require_docker')
    def test_setup_refuses_overwrite_before_touching_docker(self, require):
        with self.assertRaises(ValueError):
            manage.setup(argparse.Namespace())
        require.assert_not_called()

    @mock.patch.object(manage, 'compose')
    def test_failed_settings_restore_configuration_and_applications(self, compose):
        before = manage.ENV.read_text()
        compose.side_effect = [None, None, RuntimeError('unhealthy'), None]
        with self.assertRaises(RuntimeError):
            manage.settings(argparse.Namespace(key='GATEWAY_PORT', value='18001'))
        self.assertEqual(manage.ENV.read_text(), before)
        calls = compose.call_args_list
        self.assertTrue(all('mongo' not in call.args and 'es' not in call.args and 'redis' not in call.args for call in calls))
        self.assertIn('--no-deps', calls[-1].args)

    @mock.patch.object(manage, 'compose')
    def test_database_storage_settings_cannot_be_changed_by_settings(self, compose):
        with self.assertRaises(ValueError):
            manage.settings(argparse.Namespace(key='MONGO_PORT', value='27018'))
        compose.assert_not_called()

    def test_duplicate_ports_and_mode_mismatch_rejected(self):
        self.values['WEB_PORT'] = self.values['API_PORT']
        with self.assertRaises(ValueError):
            manage.validate(self.values)
        self.values['WEB_PORT'] = '13000'
        self.values['COMPOSE_PROFILES'] = 'mongo'
        with self.assertRaises(ValueError):
            manage.validate(self.values)


if __name__ == '__main__':
    unittest.main()
