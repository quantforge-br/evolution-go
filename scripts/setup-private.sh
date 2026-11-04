#!/bin/sh
# Generate local deployment secrets without printing them or overwriting a setup.
set -eu
cd "$(dirname "$0")/.."

if [ -e .env.private ]; then
    printf '%s\n' '.env.private already exists; leaving its credentials unchanged.'
else
    command -v openssl >/dev/null 2>&1 || {
        printf '%s\n' 'openssl is required to generate secure credentials.' >&2
        exit 1
    }
    api_key=$(openssl rand -hex 32)
    database_password=$(openssl rand -hex 32)
    (
        umask 077
        set -C
        printf 'SERVER_PORT=8080\nGLOBAL_API_KEY=%s\nPOSTGRES_PASSWORD=%s\n' \
            "$api_key" "$database_password" > .env.private
    )
    printf '%s\n' 'Created .env.private with unique credentials (mode 600).'
fi

printf '%s\n' \
    'Start with:' \
    '  docker compose --env-file .env.private -f compose.private.yml up -d --build' \
    'Manager: http://127.0.0.1:8080/manager/login (use an SSH tunnel on a remote VPS).'
