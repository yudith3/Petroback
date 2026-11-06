#!/usr/bin/env bash
set -euo pipefail

cd /srv/petrolida
mkdir -p backups
chmod 700 backups

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
output="backups/petrolida-${timestamp}.sql.gz"
tmp="${output}.part"
trap 'rm -f -- "$tmp"' EXIT

docker compose --env-file .env -f compose.prod.yaml --profile ops run --rm -T db-tools \
  pg_dump --no-owner --no-privileges | gzip -6 > "$tmp"
chmod 600 "$tmp"
mv -- "$tmp" "$output"
trap - EXIT
find backups -maxdepth 1 -type f -name 'petrolida-*.sql.gz' -mtime +14 -delete
printf 'Database backup saved: %s\n' "$output"
