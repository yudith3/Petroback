# Division decisions, filters, and exports

## Final selection lock

The approved 221-person roster in `db/seeder/data/json/staff_results.json` is applied once at startup. Existing active `oprec_staff` rows matching its NRP become `accepted` with the approved subdivision; other active registrations become `rejected`. Registration answers and names are preserved, and missing applicants are not created.

Finalization and its persistent `oprec_staff_selections` lock commit in one transaction. Afterward, all registration and status/placement/rejection writes are blocked, including superadmin corrections. `PATCH /api/v1/oprec-staff/:id/status` returns `409 OPREC_SELECTION_FINALIZED`; `/oprec-staff/config` reports `selection_finalized=true` and `registration_open=false`. The dashboard disables Update Status. Restarts do not reapply the roster or unlock selection. Listing, details, documents, exports, and the announcement lookup remain available under their existing access rules.

The decision actions below describe behavior before finalization.

## Rejecting choices

Send exactly one action to `PATCH /api/v1/oprec-staff/:id/status`:

```json
{ "rejection_target": "primary" }
```

Allowed targets are `primary`, `secondary`, and `both`. Do not send `status`,
`accepted_division_id`, or `current_subdivision_id` with a rejection target.

- Primary rejection sets `primary_rejected=true`, moves the applicant to
  `screening_secondary`, and assigns `current_subdivision_id` to their second
  choice. The remaining choice must be active. The secondary division receives
  the applicant in its scoped list; the primary division loses screening access.
- Secondary rejection sets `secondary_rejected=true`. If primary is still
  available, the applicant moves to `screening_primary`; if primary was already
  rejected, the final status becomes `rejected`.
- Rejecting both sets both flags and the final status to `rejected`. Rejection
  clears accepted placement; final rejection also clears the screening assignment.
- A division admin can reject only choices belonging to their parent division.
  Reject Both across different parent divisions requires the superadmin. An
  admin whose division owns both choices can reject both together.

List, detail, and mutation responses expose `primary_rejected` and
`secondary_rejected`. Mutation responses use the actual resulting status and
assignment. Decisions and their audit entry are stored in one transaction with
a registrant row lock. Audit records include the target and before/after flags.

Existing status updates remain available for explicit corrections. `pending`
reopens both choices. `screening_primary` reopens primary;
`screening_secondary` rejects primary and reopens secondary. An interview or
acceptance clears rejection of its assigned choice while preserving the other
choice's decision. A legacy `status=rejected` is final rejection of both;
division admins must own both choices to use it.

The startup migration adds the flags and audit columns, backfills older
`screening_secondary`/`rejected` records, and is safe to rerun.

## Filtering by the original choice

List and both export endpoints accept the same filters:

```text
?division_id=<parent UUID>&choice_priority=1
?subdivision_id=<subdivision UUID>&choice_priority=2
```

`division_id` matches every subdivision in that parent division.
`subdivision_id` matches a single subdivision. `choice_priority=1` means
primary only, `2` means secondary only, and `all`/omitted means either. Priority
1/2 requires a division or subdivision filter. Matching division and priority
apply to the same choice; a secondary match cannot satisfy a primary filter.

These filters use the original choices, including their rejection history.
Use `status=screening_secondary` to view the active secondary queue. All filters
remain intersected with the authenticated admin's scope.

## Export workbook

`GET /api/v1/oprec-staff/export/excel` creates one `.xlsx` file opening on a
**Navigasi** tab with per-division counts and links
to three tabs for each parent division represented by the matching choices:

- **Data - Division**: one row per applicant, with primary and secondary parent
  divisions and subdivisions near the start, the priority belonging to this
  division, biodata, contacts, document links, placement, status, and rejection flags.
- **Jawaban - Division**: one row per applicant, with each question in its own column.
  Includes general responses, both choices and their complete answers, all ten
  skill levels, design applications, equipment, sponsor/vendor relations, and
  connected companies. Use the name or NRP filter to read one applicant's answers.
  A missing optional record produces blank answers, rather than invented values.
- **Scoring - Division**: one row per applicant with identity, original choices,
  and document links filled. General and choice questions have their own blank
  score columns. The ten skill levels share one **Skor Skills Level** column.
  The six yes/no additional-information answers share one **Skor Additional
  Information** column. Design applications and connected company retain their
  own score columns. All individual answers remain in Jawaban.
  This is followed by Portfolio, Kelengkapan Berkas, KTM, CV, Twibbon, and LinkedIn,
  Instagram, and TikTok follow-proof scores. Penilai and Catatan also stay blank.
  Assessment cells use General format so entered scores remain numeric. Required
  documents share their submitted folder link; individual file URLs are not stored.
  **Akumulasi Skor**, immediately before Catatan, automatically sums all question,
  Skills Level, Additional Information, design applications, connected company,
  and document scores on that applicant's row. It stays blank
  until at least one numeric score is entered (an entered zero displays 0),
  excludes Penilai and Catatan, and recalculates when scores change. No weighting
  or scoring scale is assumed, and all assessment inputs start blank.

Every table starts with its header in row 1 and applicants in row 2, without
banner, instruction, or summary rows. Headers are light gray; body cells stay
white with thin borders. Tabs use neutral defaults. Data and Scoring rows are
compact; answer rows wrap text at a fixed height without discarding cell contents.
Document URLs display as short hyperlinks. Filters and frozen headers plus the
first three identity columns keep wide tables navigable. Navigation is the active
first tab and links directly to each section.

Different subdivisions in the same division share the three tabs. An applicant
appears once in all three sections per parent division; an applicant who chose two
different divisions appears in both divisions. Explicit division/subdivision/
priority filters restrict the groups. Division admins receive only their own
division's three tabs, while both original choices remain visible in the row.
Rows with no available choice mapping use a fallback group. An empty superadmin
export has navigation with an empty-results message; an empty division-admin
export keeps that division's three header-only templates.

CSV keeps one row per matching applicant and now includes all skills and
additional-information responses after the existing columns. Both formats export
all matching rows, regardless of the current page. Workbook tab names
are sanitized, limited to 31 characters, and made unique without regard to case.
Text identifiers, formula safety, and authenticated portfolio links are preserved.
