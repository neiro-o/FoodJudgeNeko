"""Validate every independent Docker/existing DB selection using real Compose."""
import importlib.util
import itertools
from pathlib import Path
import shutil
import tempfile

SOURCE = Path(__file__).parents[1]
spec = importlib.util.spec_from_file_location('manage', SOURCE / 'manage.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
with tempfile.TemporaryDirectory(prefix='mtv2-compose-') as temp:
    m.ROOT = Path(temp)
    m.ENV = m.ROOT / '.env'
    shutil.copyfile(SOURCE / 'compose.yaml', m.ROOT / 'compose.yaml')
    shutil.copyfile(SOURCE / 'config.yml.example', m.ROOT / 'config.yml')
    shutil.copyfile(SOURCE / 'nginx.conf.template', m.ROOT / 'nginx.conf.template')
    (m.ROOT / 'certs').mkdir()
    for modes in itertools.product(('docker', 'existing'), repeat=3):
        values = m.read_env(SOURCE / '.env.example')
        for key in ('MONGO_ROOT_PASSWORD', 'REDIS_PASSWORD', 'ELASTIC_PASSWORD'):
            values[key] = 'test-only-not-a-secret-12345'
        for name, mode in zip(('mongo', 'redis', 'es'), modes):
            values[name.upper() + '_MODE'] = mode
        values['COMPOSE_PROFILES'] = ','.join(m.selected_services(values))
        m.validate(values)
        m.write_env(m.ENV, values)
        m.compose('config', '--quiet')
        print('Validated', dict(zip(('mongo', 'redis', 'es'), modes)))
