"""End-to-end test with unique project/ports/volumes, never production settings."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
import urllib.parse

SOURCE = Path(__file__).parents[1]
spec = importlib.util.spec_from_file_location('manage', SOURCE / 'manage.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def free_port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return str(s.getsockname()[1])


def request(url, body=None, token=None):
    headers = {'Content-Type': 'application/json'}
    parsed = urllib.parse.urlsplit(url)
    headers['Origin'] = parsed.scheme + '://' + parsed.netloc
    if token:
        headers['Authorization'] = 'Bearer ' + token
    if sys.platform == 'darwin':
        # Docker Desktop's host namespace is inside its Linux VM. Run the HTTP
        # probe there; do not change production networking to accommodate macOS.
        command = ['docker', 'compose', '--env-file', str(m.ENV), '-f', str(m.ROOT / 'compose.yaml'),
                   'exec', '-T', 'web', 'node', '-e',
                   "let s='';process.stdin.on('data',c=>s+=c);process.stdin.on('end',async()=>{try{const q=JSON.parse(s);const r=await fetch(q.url,q.options);console.log(JSON.stringify({status:r.status,body:await r.text()}));}catch(e){console.error(e.message);process.exit(1);}});"]
        options = {'headers': headers}
        if body is not None:
            options.update(method='POST', body=json.dumps(body))
        result = json.loads(subprocess.check_output(command, input=json.dumps({'url':url, 'options':options}).encode()))
        assert result['status'] == 200, result
        return result['body']
    req = urllib.request.Request(url, json.dumps(body).encode() if body is not None else None, headers)
    with urllib.request.urlopen(req, timeout=20) as result:
        assert result.status == 200
        return result.read()


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--build', action='store_true')
    p.add_argument('--api-image', default='mtv2-api:compose-v1')
    p.add_argument('--web-image', default='mtv2-web:compose-v1')
    args = p.parse_args()
    if args.build:
        for image, folder in ((args.api_image, 'backend'), (args.web_image, 'frontend')):
            subprocess.run(['docker', 'build', '-t', image, str(SOURCE.parent / folder)], check=True)
    project = 'mtv2-smoke-' + secrets.token_hex(4)
    with tempfile.TemporaryDirectory(prefix=project + '-') as temp:
        m.ROOT = Path(temp)
        m.ENV = m.ROOT / '.env'
        for src, target in (('compose.yaml', 'compose.yaml'), ('config.yml.example', 'config.yml'), ('nginx.conf.template', 'nginx.conf.template'), ('.env.example', '.env.example')):
            shutil.copyfile(SOURCE / src, m.ROOT / target)
        (m.ROOT / 'certs').mkdir()
        values = m.read_env(SOURCE / '.env.example')
        values.update(COMPOSE_PROJECT_NAME=project, COMPOSE_PROFILES='mongo,redis,es', MONGO_MODE='docker', REDIS_MODE='docker', ES_MODE='docker', API_IMAGE=args.api_image, WEB_IMAGE=args.web_image, GATEWAY_BIND='127.0.0.1')
        used = set()
        for key in ('GATEWAY_PORT', 'API_PORT', 'WEB_PORT', 'MONGO_PORT', 'REDIS_PORT', 'ES_PORT'):
            port = free_port()
            while port in used:
                port = free_port()
            used.add(port)
            values[key] = port
        for key in ('MONGO_ROOT_PASSWORD', 'REDIS_PASSWORD', 'ELASTIC_PASSWORD', 'MTV2_JWT_SECRET'):
            values[key] = secrets.token_hex(32)
        values.update(MTV2_MONGODB_URI='mongodb://mtv2_root:' + values['MONGO_ROOT_PASSWORD'] + '@127.0.0.1:' + values['MONGO_PORT'] + '/?authSource=admin', MTV2_REDIS_PORT=values['REDIS_PORT'], MTV2_REDIS_PASSWORD=values['REDIS_PASSWORD'], MTV2_ES_HOSTS='http://127.0.0.1:' + values['ES_PORT'], MTV2_ES_PASSWORD=values['ELASTIC_PASSWORD'])
        m.write_env(m.ENV, values)
        try:
            m.apply(local=True, initialize=True)
            # Initialization must be repeatable without clearing anything.
            m.initialize_managed(values)
            base = 'http://127.0.0.1:' + values['GATEWAY_PORT']
            assert json.loads(request(base + '/health'))['code'] == 0
            request(base + '/login')
            for path in ('/asset-manifest.json', '/asset-worker.js', '/brand/navbar-logo.png'):
                request(base + path)
            env = dict(MTV2_ADMIN_USERNAME='deployment-smoke', MTV2_ADMIN_EMAIL='smoke@example.invalid', MTV2_ADMIN_PASSWORD=secrets.token_hex(16))
            m.compose('run', '--rm', '--no-deps', '-e', 'MTV2_ADMIN_USERNAME', '-e', 'MTV2_ADMIN_EMAIL', '-e', 'MTV2_ADMIN_PASSWORD', 'api', '/app/mtv2ctl', 'admin', env=env)
            body = dict(username=env['MTV2_ADMIN_USERNAME'], password=env['MTV2_ADMIN_PASSWORD'])
            login = json.loads(request(base + '/api/login', body))
            assert login['code'] == 0, login
            token = login['data']['token']
            for path in ('/api/problem/quota', '/api/problem/recent'):
                result = json.loads(request(base + path, token=token))
                assert result['code'] == 0, result
            m.settings(argparse.Namespace(key='MTV2_ALLOWED_ORIGINS', value='https://smoke.invalid'))
            assert json.loads(request(base + '/health'))['code'] == 0
            print('PASS: isolated full stack, initialization, admin login, authenticated API, static assets and settings')
        except Exception:
            m.compose('ps', check=False)
            # Do not print environment or unredacted logs to CI.
            raise
        finally:
            # Only the unique test project is removed; no production project is loaded.
            assert project.startswith('mtv2-smoke-')
            m.compose('down', '--volumes', '--remove-orphans', check=False)


if __name__ == '__main__':
    main()
