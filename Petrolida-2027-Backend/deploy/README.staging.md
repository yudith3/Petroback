# Staging on the production VPS

Target: https://staging.petrolida.com on 103.247.11.184.
Source: staging branch in each repository. Separate /srv/petrolida-staging,
image namespaces, ports 28998/23000, JWT secret, admin session cookie,
portfolio volume, and Supabase staging project.

PR #60 review: its deployment step writes /srv/petrolida and uses
compose.prod.yaml, replacing production when VPS_HOST is this shared VPS.
Its documentation explicitly requires another server. CI passed, but that
configuration is unsuitable for the requested shared VPS. It also does not
provide the frontend workflow or provision staging GitHub Environments.
These changes supply the isolated backend and frontend workflows.

## Provision

Create /srv/petrolida-staging/{incoming,backups}, owned by deploy.
Copy compose.staging.yaml, deploy-staging.sh and backup-staging.sh there.
Copy .env.staging.example to .env, fill separate database credentials and
independent random JWT_SECRET/admin password, chmod 600 .env and chmod 700 scripts.
Use Supabase Session Pooler port 5432 if the direct endpoint is IPv6-only.
Keep ENVIRONMENT=production for secure cookies and strong JWT validation.
The database is external and does not reuse production data or credentials.

Create docker network petrolida-staging-edge. Both staging services join this
network; only the shared production Caddy also joins it. Keep this distinct
from petrolida-prod_default to prevent backend/frontend DNS alias collisions.

Create staging GitHub Environments in both repositories, restrict branches to
staging, and set VPS_HOST, VPS_KNOWN_HOSTS and deploy-account VPS_SSH_KEY.
Deploy backend first, then frontend. The backend workflow transfers its image,
staging Compose file, and staging deployment/backup scripts together, so new
environment mappings reach the running backend. The server `.env` is retained.
The frontend workflow transfers its image only. Each service checks
localhost readiness and rolls back on failure. Schema migrations are not reversed.

## Test the administration announcement

Staging defaults to an open administration release at
`2026-10-08T00:00:00+07:00`; production retains its independent 10:00 WIB release.
The backend imports the approved roster automatically at startup. Test the real
form at `https://staging.petrolida.com/announcement/interview` and verify
`/api/v1/announcements/config` reports `interview.open=true`.

If staging's server `.env` already overrides this date, dispatch the backend
staging workflow on the `staging` branch with `interview_announcement_at` set to
the desired RFC3339 timestamp. The helper changes only that key in
`/srv/petrolida-staging/.env`, preserves the other settings, and uses the existing
staging deployment lock. The backend is recreated by the normal release flow.
Changing this input does not affect the production environment.

## Test the final staff announcement

Open `https://staging.petrolida.com/announcement/staff`. Staging defaults to an
open staff release at `2026-10-15T00:00:00+07:00` when its server setting is empty.
To override an existing staging release date, dispatch the backend staging workflow
on `staging` with `staff_announcement_at` set to a past RFC3339 timestamp, for
example `2026-10-15T00:00:00+07:00`. The helper changes only
`OPREC_ANNOUNCEMENT_AT` in `/srv/petrolida-staging/.env` and the deployment
recreates the staging backend. Verify `/api/v1/announcements/config` reports
`staff.open=true`. The production schedule and the existing finalized
`oprec_staff` decisions remain intact.

## Caddy

Preserve production routes. Add the optional import from deploy/Caddyfile,
the staging directory mount and the staging-edge network from compose.prod.yaml.
Place Caddyfile.staging at /srv/petrolida/staging/staging.caddy.
Validate before recreating Caddy; recreation briefly interrupts routing.
Keep production deployment source updated with these additions so later prod
releases retain staging routing. Never copy staging Compose over production Compose.

```sh
cd /srv/petrolida
docker compose --env-file .env -f compose.prod.yaml run --rm --no-deps caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose --env-file .env -f compose.prod.yaml up -d --no-deps --force-recreate caddy
curl -f https://petrolida.com/api/v1/health
curl -f https://staging.petrolida.com/api/v1/health
curl -f https://staging.petrolida.com/
```

Check staging login and test registration separately. X-Robots-Tag discourages
indexing; it does not restrict access. Use synthetic staging records.
Backend deployments back up the staging database; retention is 14 days.
Store portfolio-volume backups separately when needed.

## PetroRun activation

Both Compose files forward every `PETRORUN_*` key from their own server `.env`.
Leave all keys blank until the receiving bank and admin/jersey fees are final;
the API then remains running and `/api/v1/petrorun/config` returns 503. Do not
fill a subset: incomplete event settings fail backend startup validation.

The organizer's default schedule is 24 Oct–22 Nov 2026 WIB, racepack 25 Nov.
Normal price is configured as 150000; early bird defaults to 125000 for 25
participants reserved for 24 hours, and late price 139000 begins 19 Nov WIB.
See `docs/petrorun.md` for the complete environment keys, quota/review rules,
exclusive closing timestamp and activation checklist. Redeploy backend after
updating staging's server `.env`; GitHub environment variables alone do not
change the container's event configuration. Production has independent values.

## Enable PetroRun tests before payment details are final

Run **Configure PetroRun staging tests** on branch `staging`, mode `enable`,
after deploying the current backend. It opens a synthetic registration window,
uses the approved 125k/150k/139k price tiers, sets admin/jersey fees to zero, and
uses `STAGING TEST BANK - JANGAN TRANSFER` / `0000000000`. The frontend displays
a persistent test notice. No real transfer is required: use a synthetic receipt
and review it from `/admin/petrorun`. The public config exposes `test_mode=true`.
The backend rejects this flag unless APP_NAME is `petrolida-staging-api`;
production Compose does not forward the test flag.

The helper preserves existing admin credentials and order access secrets;
missing bootstrap credentials are randomly generated and provisioned on restart.
An optional RSA public key PEM lets the requester download admin credentials as
an RSA-OAEP-SHA256 encrypted artifact with one-day retention. Plaintext is never
logged or uploaded, and the private key stays on the requester's machine.

The helper runs in an ephemeral Docker CLI/Python container, so Python is not
required on the VPS. It preserves the deploy user's ownership and private mode
600, holds the staging deployment lock, preserves other `.env` settings, and
restores runtime settings if readiness fails. The first original PetroRun
settings are saved privately in `.petrorun-before-test.json`. Mode `restore`
reinstates those settings; test orders remain in the separate staging database.
Never use this workflow as production activation or send real money to the
dummy bank. Final bank/admin/jersey configuration remains a separate launch step.
