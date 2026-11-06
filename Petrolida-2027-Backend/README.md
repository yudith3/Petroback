# Petrolida 2027 Backend

REST API foundation for Petrolida 2027. It uses Go, Gin, GORM, and PostgreSQL with a layered
`router -> handler -> service -> repository` structure ready for future business modules.

## Requirements

- Go 1.25 or newer
- Docker Desktop with Docker Compose
- Air, optionally, for live reload

## Local setup

PowerShell:

```powershell
Copy-Item .env.example .env
docker compose up -d postgres
go mod download
go run .
```

The development database is exposed on host port `54327` to avoid collisions with an existing
PostgreSQL installation. Containers communicate with it internally on port `5432`.

The API starts at `http://localhost:8998`. Check readiness at:

```text
GET http://localhost:8998/api/v1/health
```

To run PostgreSQL and the containerized API together:

```bash
docker compose --profile full up --build
```

## Commands

```bash
make run       # Start the API
make dev       # Start with Air live reload
make db-up     # Start PostgreSQL
make db-down   # Stop local containers
make check     # Test, vet, and build
```

## API response contract

Successful responses:

```json
{
  "status": true,
  "message": "service is ready",
  "data": {},
  "request_id": "..."
}
```

Failed responses:

```json
{
  "status": false,
  "message": "request failed",
  "error": {
    "code": "ERROR_CODE",
    "message": "safe public message"
  },
  "request_id": "..."
}
```

## Oprec Staff flow

- `GET /api/v1/oprec-staff/config` returns the registration window, scheduled announcement state, allowed batches, portfolio policy, and `selection_finalized`. Finalized selection closes registration and makes the dashboard read-only.
- `POST /api/v1/oprec-staff` accepts multipart form data: `data` contains registration JSON and `portfolio` is the optional/conditionally required PDF.
- Required evidence (KTM, CV, twibbon, and follow proof) is represented by one public Google Drive folder URL.
- `POST /api/v1/auth/login` creates an HttpOnly admin session. `admin@petrolida.com` is provisioned as the `superadmin`; the pre-provisioned division accounts use the `division_admin` role and are bound to their existing division.
- The superadmin can inspect all registrations. A division admin can only list, count, inspect, and open documents for applicants associated with their division, while both the applicant's primary and secondary choices remain visible in the API response.
- Before finalization, `PATCH /api/v1/oprec-staff/:id/status` supports selection statuses and rejection of primary, secondary, or both choices. After finalization, every admin (including the superadmin) receives `409 OPREC_SELECTION_FINALIZED`. See [the selection contract](docs/oprec-selection.md) for role restrictions and correction rules.
- `GET /api/v1/oprec-staff/activity-logs` is restricted to the superadmin and returns the status-change history across all division accounts.
- `GET /api/v1/oprec-staff/export/csv` and `GET /api/v1/oprec-staff/export/excel` download every registrant matching the list filters (`search`, `status`, `faculty_id`, `department_id`, `subdivision_id`, `batch`, `sort_by`, `sort`); `page` and `page_size` are ignored. Division admins only export applicants within their scope. Columns: registration time (WIB), biodata, faculty/department/batch, choice 1 and 2 with answers, required-documents link, portfolio link, status, interview subdivision, and accepted division. Values that spreadsheets would treat as formulas are prefixed with `'`.
  - Excel (`.xlsx`) opens on **Navigasi**, followed by three tabs per parent division: **Data** (biodata, both division/subdivision choices, contacts, documents, status), **Jawaban** (each question is a column), and **Scoring** (blank question/document scores, one **Skor Skills Level**, one **Skor Additional Information**, separate design-applications/connected-company scores, and automatic **Akumulasi Skor** immediately before Catatan). All sections use one row per applicant, headers in row 1, light gray headers, white cells, and thin borders. Data and Scoring rows stay compact; document links have short labels. Subdivisions share their parent's tabs. Filters and frozen headers plus identity columns support wide tables. NRP and phone remain text. `division_id` and `choice_priority=1|2|all` also apply to the list and both exports; see [filter and workbook semantics](docs/oprec-selection.md).
  - CSV is UTF-8 with BOM and comma-separated, and writes NRP/phone as `="..."` so leading zeros survive. Import it into Google Sheets or via Excel's Data > From Text/CSV, since Excel with Indonesian regional settings expects `;`.
- Portfolio links in exports point to `APP_PUBLIC_URL` + the authenticated document endpoint, never to the stored file key. Set `APP_PUBLIC_URL` to the site origin (for example `https://petrolida.com`, no path); production reuses `SITE_ORIGIN`.
- Portfolio files are stored outside the public web root and can only be streamed through the authenticated document endpoint.
- All authenticated admins can manage branded short links through `/api/v1/short-links`; public requests to `/go/:slug` receive a temporary redirect and increment the link's click count.

## Production deployment

The `prod` branch deploys a tested Docker image to the VPS. See [deploy/README.md](deploy/README.md)
for the shared stack, DNS setup, backups, health checks, and rollback procedure.

## Public staff and interview announcements

- `GET /api/v1/announcements/config` returns independent `staff` and `interview` release times/open flags and `server_time`.
- `POST /api/v1/announcements/staff` returns final staff selection.
- `POST /api/v1/announcements/interview` returns interview selection.

Both lookup endpoints accept JSON `{"nrp":"5070261234"}` (10 digits, surrounding whitespace trimmed). The explicitly retained Student ID `351518190408001` is also supported; other nonstandard IDs are invalid. These endpoints are public.

Successful `data` contains `name`, `nrp`, `status` (`accepted` or `rejected`), and, for successful applicants, `subdivision: {id, name}` plus optional `action_url`. The retained 15-digit ID currently has no name/subdivision in the supplied roster and returns its NRP and acceptance without fabricating those fields. Email, phone, documents, and registration answers are never returned. Responses use `Cache-Control: no-store`. NRP is sent in the request body to keep it out of URL access logs.

The approved administration announcement opens on **8 October 2026 at 10:00 WIB** (`OPREC_INTERVIEW_ANNOUNCEMENT_AT=2026-10-08T10:00:00+07:00`, or 03:00 UTC). The local environment example and all Compose environments use this release time; a nonempty environment value can override it. When running the backend directly, an empty interview release date keeps that stage closed. Set `OPREC_ANNOUNCEMENT_AT` separately for final staff release using RFC3339 with an explicit offset; an empty value keeps final staff results closed. Configure `OPREC_INTERVIEW_SCHEDULE_URL` and `OPREC_STAFF_GROUP_URL` with official absolute HTTPS URLs; empty links omit `action_url`.

Staff lookup reads **finalized decisions in `oprec_staff`**. The approved source `db/seeder/data/json/staff_results.json` contains 221 unique staff across 29 subdivisions, with labels mapped to existing master IDs (including Front End, Back End, UI/UX, PBM, Sub LO, and Medpro). Repeated name-only entries after `TOTAL STAFF` are not additional records. On the first startup with this roster, a single transaction matches active registrations by NRP: roster members become `accepted` with `accepted_divisi_id` set to their final subdivision; all other active registrants become `rejected` with no accepted placement. Screening assignments are cleared. Existing names, answers, contacts, and interview results are preserved. Missing registrations are never fabricated, so an NRP absent from `oprec_staff` returns `404 REGISTRANT_NOT_FOUND`, even if it appears in the source roster. Successful lookups show the registered name and final subdivision. Final staff release still follows `OPREC_ANNOUNCEMENT_AT`, and the group button uses `OPREC_STAFF_GROUP_URL`.

The same transaction writes the permanent `oprec_staff_selections` lock (`id=staff-2027`), recording the finalization time, roster hash, accepted/rejected counts, and the count of roster NRPs without an active registration. Row locks coordinate finalization with registration/status writes. Subsequent startups skip finalization and preserve the stored decisions. Both new registrations and all admin status/placement/rejection changes are blocked after finalization; there is no dashboard/API unlock. Master data migrations and the finalization lock have row level security enabled. Finalization is independent of the public announcement release time.

Interview lookup uses the approved administration roster in `oprec_interview_results`, independently of later admin decisions. Startup migrates this table, enables its row level security, and imports `db/seeder/data/json/interview_results.json` in a transaction. The supplied roster's names and subdivisions are preserved with subdivision labels mapped to the existing master IDs; no fake registration records are created. It contains 385 accepted IDs and 5 unique IDs on hold, including the additional retained 15-digit ID. The two corrected IDs are `5011251134` (formerly `501251134`) and `5056251049` (formerly `505625049`).

Repeated roster IDs (`5019261031`, `5046251070`, `5061261003`, `5033251113`, `5026261032`) are imported once with `status=on_hold` and no assigned subdivision. They return `409 ANNOUNCEMENT_ON_HOLD` until confirmed. Resolve a held record by setting its approved name, subdivision ID, and `status=accepted` in the snapshot table; the bootstrap seed only inserts missing IDs and preserves database corrections across restarts. After the stage opens, a registrant absent from this complete roster returns `rejected`, regardless of live selection status; an ID absent from both the roster and registration records returns `404 REGISTRANT_NOT_FOUND`. Registration and final staff decisions are not modified by this import.

Errors: `400 VALIDATION_ERROR` for malformed JSON/NRP, `403 ANNOUNCEMENT_CLOSED` before release (without querying applicant data), `404 REGISTRANT_NOT_FOUND` for unknown NRP after release, `409 ANNOUNCEMENT_PENDING` for undecided/incomplete results, and `409 ANNOUNCEMENT_ON_HOLD` for duplicate roster IDs awaiting confirmation. The frontend displays validation, availability, not-found, pending, and on-hold messages respectively. Preview routes remain separate from the real lookup.
