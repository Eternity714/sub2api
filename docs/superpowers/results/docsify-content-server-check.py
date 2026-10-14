"""Read-only checks on the production host; run gray-status.sh first."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.request

root = Path('/opt/sub2api')
release = 'sha-411025133ebf4728eba0700bc4af959c128c1df9'
container = f'sub2api-docsify-green-{release}'
canonical = root / 'content'
previous = root / f'gray/docsify/releases/blue-{release}/content'


def run(*args):
    return subprocess.check_output(args, text=True, stderr=subprocess.STDOUT)


for name, expected in {
    'state.env': 'e46ed023f47403d054ea38267995d178f4d2862c1f6114d1477fcb7af563f819',
    'slots.env': '5fb7225682a691a6f6fc83538160bcf59a5528bb6c5316d5afe474a932f0ed26',
}.items():
    assert hashlib.sha256((root / 'gray' / name).read_bytes()).hexdigest() == expected, name
print('PASS main application state and slots match baseline')

assert canonical.is_dir() and not canonical.is_symlink()
assert canonical.resolve() == canonical
excluded = os.environ.get('DOCSIFY_MIGRATION_FIXTURE', '')


def manifest(path):
    result = {}
    for entry in sorted(path.rglob('*')):
        assert not entry.is_symlink(), str(entry)
        if entry.is_file():
            name = entry.relative_to(path).as_posix()
            if name != excluded:
                result[name] = hashlib.sha256(entry.read_bytes()).hexdigest()
    return result


files = manifest(canonical)
assert files == manifest(previous), 'Migrated content differs from maintained stable content'
assert all(name in files for name in ('index.md', '_sidebar.md', '_navbar.md', '_404.md'))
print('PASS fixed real directory retains all stable content:', len(files), 'files, inode', canonical.stat().st_ino)

inspection = json.loads(run('podman', 'inspect', container))[0]
assert inspection['State']['Health']['Status'] == 'healthy'
assert inspection['Config']['Image'] == f'ghcr.io/eternity714/sub2api-docsify:{release}'
mounts = [m for m in inspection['Mounts'] if m['Destination'] == '/usr/share/nginx/html/docs']
assert len(mounts) == 1 and mounts[0]['Source'] == str(canonical)
assert mounts[0]['Type'] == 'bind' and mounts[0]['RW'] is False
print('PASS immutable candidate is healthy and binds canonical content read-only')
print('candidate_started_at=' + inspection['State']['StartedAt'])

run('sudo', '-n', 'nginx', '-t')
run('podman', 'exec', container, 'nginx', '-t')
print('PASS host and container Nginx configuration')
with urllib.request.urlopen('http://127.0.0.1:18084/health', timeout=5) as response:
    assert response.status == 200 and response.read().strip() == b'ok'
for name in files:
    with urllib.request.urlopen('http://127.0.0.1:18084/docs/' + name, timeout=5) as response:
        assert response.status == 200, name
        assert hashlib.sha256(response.read()).hexdigest() == files[name], name
print('PASS every public file is readable through the real container:', len(files))
logs = run('podman', 'logs', '--since', '5m', container)
errors = re.findall(r'"\s+(5\d\d)\s+', logs)
assert not errors, errors
error_lines = [line for line in logs.splitlines() if re.search(r'\[(?:error|crit|alert|emerg)\]', line)]
# The HTTP acceptance suite deliberately checks this one nonexistent runtime.
# Nginx logs alias misses as errors even though the expected response is 404.
expected_404 = re.compile(
    r'\[error\].*open\(\) "/opt/docsify/runtime/assets/no-such-runtime\.js" '
    r'failed \(2: No such file or directory\).*request: '
    r'"GET /docs/_assets/no-such-runtime\.js HTTP/1\.[01]"')
unexpected_errors = [line for line in error_lines if not expected_404.search(line)]
assert not unexpected_errors, unexpected_errors
print('PASS candidate recent logs have no 5xx or unexpected Nginx errors;',
      len(error_lines), 'known acceptance 404 log entries')
