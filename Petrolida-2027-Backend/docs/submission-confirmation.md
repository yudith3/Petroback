# Registration confirmation after a lost response

Registration accepts an optional random UUID v4 `submission_key` in the multipart `data` JSON. Existing clients without a key retain their existing behavior. The key and a SHA-256 fingerprint of the JSON answers and portfolio contents are stored on `oprec_staff` in the same transaction as its choices, documents, skills, and additional information. Startup migration adds two nullable columns and a unique key index; existing registrations remain valid.

A repeated request with the same key and fingerprint returns the original registration ID, including after the registration deadline. Different answers or portfolio contents receive `SUBMISSION_KEY_CONFLICT`. Concurrent requests are constrained by the unique database index; an upload belonging to a losing request is cleaned up before recovering the committed result.

`POST /api/v1/oprec-staff/submission-status` accepts `{ "submission_key": "<random UUID v4>" }`. A committed registration returns `state: "confirmed"` and its original acknowledgement. An absent key returns `state: "unconfirmed"`: the initial request may still be running. Database errors remain errors. The response has `Cache-Control: no-store`, contains no applicant details, and does not expose recruitment decisions. The key is a secret receipt and should remain in the request body rather than a URL or logs.

Deploy the backend before the frontend. Integration tests use an isolated PostgreSQL schema in CI to verify transaction rollback, complete related records, multiple legacy null keys, and concurrent key uniqueness. No participant data is used in these tests.
