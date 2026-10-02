#!/usr/bin/env python3
"""Launch the isolated Linux/WSL recipe without loading an existing .env."""
import argparse
from datetime import datetime, timezone
import os
from pathlib import Path
import secrets
import stat
import sys
from urllib.parse import urlsplit

SERVICE_KEYS = {
    'MINIO_ACCESS_KEY', 'MINIO_SECRET_KEY', 'NATS_USER', 'NATS_PASSWORD',
    'MINIO_BUCKET_COVERS', 'MINIO_API_BIND', 'MINIO_CONSOLE_BIND',
    'NATS_CLIENT_BIND', 'NATS_MONITOR_BIND',
}


def origin(value):
    if not value:
        return ''
    parsed = urlsplit(value)
    if (parsed.scheme != 'https' or not parsed.hostname or parsed.username
            or parsed.password or parsed.path or parsed.query or parsed.fragment
            or any(c.isspace() for c in value)):
        raise ValueError('L’origine doit être https://hote, sans chemin ni identifiants.')
    _ = parsed.port
    return value


def prepare(root, source, public_origin):
    root, source = root.resolve(), source.resolve()
    public_origin = origin(public_origin)
    database = root / 'defta-restored.db'
    services = root / 'services.env'
    if database.is_symlink() or not database.is_file():
        raise ValueError('La copie de recette defta-restored.db est absente ou est un lien.')
    if services.is_symlink() or not services.is_file():
        raise ValueError('services.env absent ou lien symbolique.')
    if stat.S_IMODE(services.stat().st_mode) & 0o077:
        raise ValueError('services.env doit être privé : chmod 600 sur ce fichier.')
    if os.path.lexists(source / '.env'):
        raise ValueError('Utiliser un checkout séparé sans .env ; le fichier existant reste intact.')
    if not (source / 'go.mod').is_file() or not (source / 'cmd').is_dir():
        raise ValueError('Le checkout source est invalide.')
    values = {}
    for line in services.read_text().splitlines():
        if not line or line.startswith('#'):
            continue
        key, separator, value = line.partition('=')
        if not separator or key not in SERVICE_KEYS or key in values or not value:
            raise ValueError('Format ou clé invalide dans services.env (contenu non affiché).')
        values[key] = value
    required = {'MINIO_ACCESS_KEY', 'MINIO_SECRET_KEY', 'NATS_USER', 'NATS_PASSWORD',
                'MINIO_BUCKET_COVERS', 'MINIO_API_BIND', 'NATS_CLIENT_BIND'}
    if not required <= values.keys():
        raise ValueError('services.env incomplet.')
    for key in ('MINIO_API_BIND', 'NATS_CLIENT_BIND'):
        host, separator, port = values[key].rpartition(':')
        if host != '127.0.0.1' or not separator or not port.isdigit() or not 1 <= int(port) <= 65535:
            raise ValueError('Les services de recette doivent écouter sur 127.0.0.1:port.')
    # Never inherit application credentials/configuration from the active shell.
    env = {k: v for k, v in os.environ.items() if k in {
        'PATH', 'HOME', 'USER', 'TMPDIR', 'LANG', 'LC_ALL', 'GOPATH',
        'GOCACHE', 'GOMODCACHE', 'GOTOOLCHAIN', 'GOFLAGS', 'CC', 'CXX',
    }}
    env.update(values)
    env.update({
        'PORT': '8080', 'DB_PATH': str(database), 'JWT_SECRET': secrets.token_hex(32),
        'PUBLIC_ORIGIN': public_origin, 'AUTH_COOKIE_SECURE': str(bool(public_origin)).lower(),
        'VERSION': '1.7.2', 'BUILD_DATE': datetime.now(timezone.utc).date().isoformat(),
        'COVERS_ENABLED': 'true', 'MINIO_ENDPOINT': values['MINIO_API_BIND'],
        'MINIO_USE_SSL': 'false', 'NATS_URL': 'nats://' + values['NATS_CLIENT_BIND'],
        'NSFW_MODERATION_ENDPOINT': 'http://127.0.0.1:8090',
        'OCR_ENGINE': 'tesseract', 'OCR_LANGUAGE': 'ara', 'OCR_EXPERIMENTAL_ENABLED': 'false',
    })
    return source, env


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--recette-dir', required=True, type=Path)
    parser.add_argument('--source', type=Path, help='Défaut : RECETTE_DIR/source')
    parser.add_argument('--public-origin', default='', help='Vide : HTTP local ; origine HTTPS : tunnel')
    parser.add_argument('--check', action='store_true', help='Valider sans démarrer ni modifier de fichier')
    args = parser.parse_args()
    try:
        source, env = prepare(args.recette_dir, args.source or args.recette_dir / 'source', args.public_origin)
        print('Configuration de recette validée ; mode ' + ('HTTPS via proxy local.' if env['PUBLIC_ORIGIN'] else 'HTTP local.'), flush=True)
        if not args.check:
            os.chdir(source)
            os.execvpe('go', ['go', 'run', '-tags', 'fts5', './cmd'], env)
    except (ValueError, OSError):
        # Do not expose configuration values through exception messages.
        print('Lancement refusé : vérifier origine HTTPS, checkout sans .env, copie SQLite et services.env privé conforme au guide.', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
