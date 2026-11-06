# Production deployment

The `prod` branch builds the backend image in GitHub Actions and sends it to the VPS over SSH. The frontend repository has a matching workflow. Both use `/srv/petrolida/compose.prod.yaml` and `/srv/petrolida/deploy.sh` on the server. Images are tagged with the full Git commit SHA. Deployments are serialized on the server and checked through localhost before the tag is kept. A failed release restores the previous image.

The stack includes the Go API, the Next.js frontend, Caddy, an internal Nginx master-data cache, a persistent private portfolio volume for local fallback, and an on-demand PostgreSQL client used for backups. Production portfolios can be stored in Google Drive by selecting the `gdrive` storage driver. The application database is the managed Supabase PostgreSQL instance and stores only portfolio metadata and the Drive file ID, not the PDF content. Only Caddy publishes public ports 80 and 443. The API and frontend also have localhost-only ports for readiness checks.

## Initial VPS setup

Install Docker Engine and the Compose plugin from Docker's RHEL repository, then create a dedicated `deploy` user with Docker access and an SSH key from each repository's `VPS_SSH_KEY` secret. Give `/srv/petrolida` to this user and copy `compose.prod.yaml`, `Caddyfile`, `nginx-cache.conf`, `deploy.sh`, and `backup.sh` there. Create `/srv/petrolida/incoming` and an owner-only `/srv/petrolida/.env`:

```dotenv
DB_HOST=<supabase-pooler-host>
DB_PORT=5432
DB_USER=<supabase-pooler-user>
DB_PASS=<database-password>
DB_NAME=postgres
DB_SSL_MODE=require
JWT_SECRET=<random-value-at-least-32-characters>
ADMIN_NAME=<initial-admin-name>
ADMIN_EMAIL=<initial-admin-email>
ADMIN_PASSWORD=<initial-random-admin-password>
OPREC_REGISTRATION_START_AT=2026-09-23T18:00:00+07:00
OPREC_REGISTRATION_END_AT=2026-10-06T18:00:00+07:00
OPREC_INTERVIEW_ANNOUNCEMENT_AT=2026-10-08T10:00:00+07:00
OPREC_ALLOWED_BATCHES=2024,2025,2026
PORTFOLIO_STORAGE_DRIVER=gdrive
GDRIVE_CLIENT_ID=<google-oauth-client-id>
GDRIVE_CLIENT_SECRET=<google-oauth-client-secret>
GDRIVE_REFRESH_TOKEN=<google-oauth-refresh-token>
GDRIVE_FOLDER_ID=<destination-drive-folder-id>
FRONTEND_TAG=pending
BACKEND_TAG=pending
SITE_ADDRESS=http://103.247.11.184
SITE_ORIGIN=http://103.247.11.184
```

Generate `JWT_SECRET` and the initial admin password with a cryptographically secure password generator; do not commit or print any secret. `ADMIN_*` bootstraps or deliberately rotates the configured admin account at startup, so store it only in the owner-only server `.env`. Set `.env` mode to `0600`. The GitHub repositories need a `VPS_SSH_KEY` secret and `VPS_HOST` and `VPS_KNOWN_HOSTS` variables. The host key must be pinned to the server's verified ED25519 host key.

Push or manually run the `prod` backend workflow first, then the frontend workflow. The backend workflow uploads the Compose and Caddy configuration. Check `docker compose --env-file .env -f compose.prod.yaml ps`, `curl -f http://127.0.0.1:18998/api/v1/health`, and `curl -f http://127.0.0.1:13000/` on the VPS.

Registration stays closed when either schedule value is missing. Both values must use RFC3339 with an explicit offset, and the end must be later than the start. After changing either value in `/srv/petrolida/.env`, recreate the backend container so it receives the new environment:

```sh
docker compose --env-file .env -f compose.prod.yaml up -d --no-deps --force-recreate backend
curl -f https://petrolida.com/api/v1/oprec-staff/config
```

For daily database backups, install the provided systemd service and timer in `/etc/systemd/system`, run `systemctl daemon-reload`, and enable `petrolida-backup.timer`. The backup job connects to Supabase through the configured pooler and never embeds credentials in the archive command. Backend deployment also makes a database backup before replacing its image. Backups are retained for 14 days in `/srv/petrolida/backups`. Copy database backups and, when the local storage driver is used, the `portfolio_data` Docker volume off the VPS for disaster recovery. Google Drive portfolio files follow the retention and recovery policy of the configured Drive account.

## Enable the domain and HTTPS

The authoritative nameservers for `petrolida.com` are Hostinger. In Hostinger DNS, keep the A record for `@` pointing to `103.247.11.184` and the `www` CNAME pointing to `petrolida.com`. Preserve unrelated DNS records, including Mailgun and existing subdomains. Allow TCP 80 and 443 and UDP 443 to the VPS. After the A record resolves to the VPS, update the two server values:

```dotenv
SITE_ADDRESS=petrolida.com
SITE_ORIGIN=https://petrolida.com
```

Then run `docker compose --env-file .env -f compose.prod.yaml up -d --remove-orphans --no-deps --force-recreate caddy backend` from `/srv/petrolida`. Verify `https://petrolida.com/`, `https://petrolida.com/api/v1/health`, a configured `https://petrolida.com/go/:slug`, and the redirect from `https://www.petrolida.com/`. Caddy obtains and renews the certificates automatically. The frontend calls `/api/v1` on the same origin, so the existing `api.petrolida.com` record can continue to serve its separate destination.

## Operations

### Master-data cache

Caddy forwards `/api/v1/master/*` to the internal `nginx-cache:8080` service.
Anonymous GET/HEAD requests to the eight existing public master-data list routes
are cached for 15 minutes. Detail routes and unknown paths bypass caching.
All other API and frontend routes retain their direct Caddy routing.
Requests with any Cookie or Authorization header bypass both cache reads and writes.
Only upstream `200` responses are stored; `Cache-Control`, `Set-Cookie` and `Vary`
remain authoritative. Cache keys include the host, query string and Origin.
The cache has a 100 MB disk limit and 10 MB metadata zone and uses a dedicated volume.
Expired responses are not served stale. Changes to master data can therefore take
up to 15 minutes to become visible to anonymous visitors.

Backend deployment uploads and validates the Nginx configuration, waits for its
health check and then activates Caddy. For an existing server, copy the updated
Compose, Caddy and Nginx files to `/srv/petrolida` and run:

```sh
cd /srv/petrolida
docker compose --env-file .env -f compose.prod.yaml config --quiet
docker compose --env-file .env -f compose.prod.yaml run --rm --no-deps nginx-cache nginx -t
docker compose --env-file .env -f compose.prod.yaml run --rm --no-deps caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose --env-file .env -f compose.prod.yaml up -d --no-deps --force-recreate --wait --wait-timeout 45 nginx-cache
docker compose --env-file .env -f compose.prod.yaml exec -T nginx-cache wget -q -O /dev/null http://127.0.0.1:8080/api/v1/master/faculties
docker compose --env-file .env -f compose.prod.yaml up -d --no-deps --force-recreate caddy
```

Request the same endpoint twice without cookies or Authorization. Inspect
`X-Cache-Status`: a cold entry returns `MISS`, then `HIT`; sessions return `BYPASS`.
The entry may already be warm from another visitor. Logs report cache status,
total request time and upstream time without logging query strings or credentials.

```sh
curl -sS -D - -o /dev/null https://petrolida.com/api/v1/master/faculties
curl -sS -D - -o /dev/null https://petrolida.com/api/v1/master/faculties
docker compose --env-file .env -f compose.prod.yaml logs --tail=100 nginx-cache
```

To immediately invalidate master data after a database update, delete only cache
files (the next anonymous request repopulates the entry):

```sh
docker compose --env-file .env -f compose.prod.yaml exec -T nginx-cache sh -c 'find /var/cache/nginx/master-data -type f -delete'
```

Run the proxy integration checks locally or in CI with a running Docker engine:
`python3 deploy/tests/verify_cache.py`. They use synthetic responses and need no
Supabase connection, credentials or live participant data.

Production activation on 2026-10-04 used a Caddy configuration reload. All 17 live
checks passed, including the eight list routes, credential bypass, CORS isolation,
and production/staging health. Seven VPS-local samples on the faculties endpoint
had medians of 18.85 ms through the backend and 1.28 ms on a cache HIT; these timings
exclude the visitor's internet connection. See the dated
[verification report](verification/nginx-cache-2026-10-04.json).
The previous server configuration is retained at
`/srv/petrolida/cache-backups/nginx-cache-setup-20261004-b41c7e`.
Keep the repository's cache configuration and production workflow in the next
production release so its file upload retains the cache routing.

### Release operations

- `docker compose --env-file .env -f compose.prod.yaml logs --tail=100 backend frontend caddy nginx-cache` shows recent logs.
- Re-run a repository's production workflow to retry a release.
- Restore an earlier image by placing its SHA in the relevant `.env` tag and running `docker compose --env-file .env -f compose.prod.yaml up -d --no-deps --force-recreate backend` or `frontend`.
- Database migrations run when the backend starts. Image rollback does not reverse schema changes, so review migrations for backward compatibility before merging to `prod`.
