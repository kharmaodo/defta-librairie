#!/usr/bin/env python3
"""Verify release metadata, asset inventory and the Go compiler in each binary."""
import argparse
import datetime
import json
from pathlib import Path, PurePosixPath
import re
import subprocess
import tarfile
import tempfile
import zipfile


def safe_asset(name, root):
    path = PurePosixPath(name)
    if path.is_absolute() or '..' in path.parts or '\\' in name or not path.parts or path.parts[0] != root:
        raise ValueError('Unexpected archive path')
    parts = path.parts[1:]
    if any((p.startswith('.env') and p != '.env.example') or p in {'.git', 'data', 'backups'} or p.endswith(('.db', '.sqlite', '.sqlite3', '.db-wal', '.db-shm')) for p in parts):
        raise ValueError('Private runtime data in archive')
    if parts and parts[0] not in {'templates', 'static', '.env.example', 'README.md', 'BUILD-INFO.txt', 'defta-librairie', 'defta-librairie.exe'}:
        raise ValueError('Unexpected release asset')


def verify(archive, version, commit):
    windows = archive.name.endswith('-windows-amd64.zip')
    platform = 'windows-amd64' if windows else 'linux-amd64'
    root = f'defta-librairie-{version}-{platform}'
    expected = root + ('.zip' if windows else '.tar.gz')
    if archive.name != expected:
        raise ValueError('Unexpected archive name')
    required = {'static/fonts/manrope.ttf', 'static/fonts/manrope-OFL.txt', 'static/fonts/notokufiarabic.ttf', 'static/fonts/notokufiarabic-OFL.txt', 'BUILD-INFO.txt', 'static/fonts/notosansarabic.ttf', 'static/fonts/notosansarabic-OFL.txt', 'static/css/fonts.css', 'templates/base.html'}
    binary = 'defta-librairie.exe' if windows else 'defta-librairie'
    required.add(binary)
    contents = {}
    seen = set()
    container = zipfile.ZipFile(archive) if windows else tarfile.open(archive, 'r:gz')
    with container:
        entries = container.infolist() if windows else container.getmembers()
        for entry in entries:
            name = entry.filename if windows else entry.name
            safe_asset(name, root)
            if name in seen:
                raise ValueError('Duplicate archive path')
            seen.add(name)
            directory = entry.is_dir() if windows else entry.isdir()
            regular = ((entry.external_attr >> 16) & 0o170000) in {0, 0o100000} if windows else entry.isfile()
            if directory:
                continue
            if not regular:
                raise ValueError('Links or special files in archive')
            relative = str(PurePosixPath(name).relative_to(root))
            required.discard(relative)
            if relative not in {binary, 'BUILD-INFO.txt'}:
                continue
            size = entry.file_size if windows else entry.size
            limit = 4096 if relative == 'BUILD-INFO.txt' else 128 * 1024 * 1024
            if size > limit:
                raise ValueError('Oversized metadata or binary')
            stream = container.open(entry) if windows else container.extractfile(entry)
            with stream:
                contents[relative] = stream.read(limit + 1)
            if len(contents[relative]) > limit:
                raise ValueError('Oversized archive member')
    if required:
        raise ValueError('Missing release assets: ' + ', '.join(sorted(required)))
    info = dict(line.split('=', 1) for line in contents['BUILD-INFO.txt'].decode().splitlines())
    if info.get('VERSION') != version or info.get('SOURCE_COMMIT') != commit:
        raise ValueError('Release version or source commit mismatch')
    datetime.date.fromisoformat(info['BUILD_DATE'])
    match = re.fullmatch(r'go(\d+)\.(\d+)\.(\d+)', info.get('GO_VERSION', ''))
    if not match or tuple(map(int, match.groups())) < (1, 26, 7):
        raise ValueError('Go 1.26.7 or newer required')
    with tempfile.TemporaryDirectory(prefix='defta-package-check-') as directory:
        target = Path(directory) / binary
        target.write_bytes(contents[binary])
        result = subprocess.run(['go', 'version', '-m', str(target)], capture_output=True, text=True, check=True)
    actual = result.stdout.splitlines()[0].rsplit(': ', 1)[-1].strip()
    if actual != info['GO_VERSION']:
        raise ValueError('Compiled Go version differs from metadata')
    settings = dict(re.findall(r'\b(GOOS|GOARCH)=(\S+)', result.stdout))
    if settings.get('GOOS') != ('windows' if windows else 'linux') or settings.get('GOARCH') != 'amd64':
        raise ValueError('Compiled platform differs from archive name')
    return {'archive': archive.name, 'version': version, 'commit': commit, 'go': actual, 'status': 'verified'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--commit', required=True)
    parser.add_argument('archives', nargs='+', type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r'\d+\.\d+\.\d+', args.version) or not re.fullmatch(r'[0-9a-f]{40}', args.commit):
        parser.error('Version or full source SHA invalid')
    for archive in args.archives:
        print(json.dumps(verify(archive, args.version, args.commit)))


if __name__ == '__main__':
    main()
