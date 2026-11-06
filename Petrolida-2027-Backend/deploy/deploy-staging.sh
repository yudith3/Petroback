#!/usr/bin/env bash
set -euo pipefail

service="${1:-}"
tag="${2:-}"
case "$service" in
  frontend|backend) ;;
  *) echo 'Usage: deploy.sh {frontend|backend} FULL_GIT_SHA' >&2; exit 2 ;;
esac
if [[ ! "$tag" =~ ^[0-9a-f]{40}$ ]]; then
  echo 'Release tag must be a full Git commit SHA' >&2
  exit 2
fi

cd /srv/petrolida-staging
exec 9>.deploy.lock
flock -x 9

archive="incoming/${service}-${tag}.tar.gz"
if [[ ! -f "$archive" ]]; then
  echo "Missing release archive: $archive" >&2
  exit 1
fi

gzip -dc "$archive" | docker load >/dev/null
rm -f -- "$archive"
docker image inspect "petrolida/staging-${service}:${tag}" >/dev/null

key="${service^^}_TAG"
previous="$(awk -F= -v key="$key" '$1 == key { print $2; exit }' .env)"
if [[ -z "$previous" ]]; then
  echo "Missing $key in .env" >&2
  exit 1
fi

set_tag() {
  local value="$1" tmp
  tmp="$(mktemp .env.XXXXXX)"
  awk -F= -v key="$key" -v value="$value" \
    '$1 == key { print key "=" value; next } { print }' .env > "$tmp"
  chmod 600 "$tmp"
  mv -f -- "$tmp" .env
}

compose() {
  docker compose --env-file .env -f compose.staging.yaml "$@"
}

compose config --quiet

if [[ "$service" == backend ]]; then
  ./backup-staging.sh
fi

set_tag "$tag"
if ! compose up -d --remove-orphans --no-deps --force-recreate "$service"; then
  set_tag "$previous"
  if [[ "$previous" == pending ]]; then
    compose stop "$service"
  else
    compose up -d --no-deps --force-recreate "$service"
  fi
  exit 1
fi

if [[ "$service" == backend ]]; then
  health_url=http://127.0.0.1:28998/api/v1/health
else
  health_url=http://127.0.0.1:23000/
fi

# The backend performs database migrations and idempotent master-data seeding
# before it starts listening. A cold managed database connection can make this
# take longer than one minute, so allow three minutes before rolling back.
for attempt in {1..90}; do
  if curl --fail --silent --show-error --max-time 4 "$health_url" >/dev/null 2>&1; then
    if [[ "$service" == backend ]]; then
      : # Shared production Caddy is provisioned separately.
    fi
    printf '%s deployed at %s\n' "$service" "$tag"
    exit 0
  fi
  sleep 2
done

echo "Readiness failed for $service; restoring $previous" >&2
set_tag "$previous"
if [[ "$previous" == pending ]]; then
  compose stop "$service"
else
  compose up -d --no-deps --force-recreate "$service"
fi
exit 1
