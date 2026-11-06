#!/usr/bin/env bash
set -euo pipefail

# Preserve the deploy user's ownership of private staging files.
cd /srv/petrolida-staging
exec docker run --rm --network host \
  -e STAGING_OWNER_UID="$(id -u)" \
  -e STAGING_SOCKET_GID="$(stat -c '%g' /var/run/docker.sock)" \
  -e HOME=/tmp -e DOCKER_CONFIG=/tmp/.docker \
  -v /srv/petrolida-staging:/srv/petrolida-staging \
  -v /var/run/docker.sock:/var/run/docker.sock \
  docker:27-cli sh -ec '
    apk add --no-cache python3 su-exec >/dev/null
    exec su-exec "$STAGING_OWNER_UID:$STAGING_SOCKET_GID" \
      python3 /srv/petrolida-staging/configure-staging-petrorun.py "$@"
  ' staging-config "$@"
