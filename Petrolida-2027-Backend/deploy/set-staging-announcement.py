"""Set a staging announcement release without exposing other environment values."""

import argparse
from datetime import datetime
import os
from pathlib import Path
import re
import sys
import tempfile


KEY = "OPREC_INTERVIEW_ANNOUNCEMENT_AT"
STAGE_KEYS = {"interview": KEY, "staff": "OPREC_ANNOUNCEMENT_AT"}
STAGING_DIRECTORY = Path("/srv/petrolida-staging")


def set_release(environment_file: Path, release: str, stage: str = "interview") -> None:
    if stage not in STAGE_KEYS:
        raise ValueError("stage must be interview or staff")
    key = STAGE_KEYS[stage]
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})", release):
        raise ValueError("release must be an RFC3339 timestamp with a timezone")
    datetime.fromisoformat(release.replace("Z", "+00:00"))
    with environment_file.open(encoding="utf-8", newline="") as current:
        lines = current.read().splitlines(keepends=True)
    line_ending = "\r\n" if any(line.endswith("\r\n") for line in lines) else "\n"
    updated = []
    found = False
    for line in lines:
        if line.startswith(key + "="):
            if not found:
                updated.append(f"{key}={release}{line_ending}")
            found = True
        else:
            updated.append(line)
    if not found:
        if updated and not updated[-1].endswith("\n"):
            updated[-1] += line_ending
        updated.append(f"{key}={release}{line_ending}")
    temporary_name = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", newline="", dir=environment_file.parent, prefix=".env.announcement.", delete=False) as temporary:
            temporary_name = temporary.name
            temporary.write("".join(updated))
            temporary.flush()
            os.fsync(temporary.fileno())
        os.chmod(temporary_name, 0o600)
        os.replace(temporary_name, environment_file)
        temporary_name = None
    finally:
        if temporary_name is not None:
            os.unlink(temporary_name)


def main() -> None:
    import fcntl

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stage", choices=STAGE_KEYS, default="interview")
    args = parser.parse_args()
    release = sys.stdin.read().strip()
    with (STAGING_DIRECTORY / ".deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        set_release(STAGING_DIRECTORY / ".env", release, args.stage)
    print(f"Staging {args.stage} release set to {release}")


if __name__ == "__main__":
    main()
