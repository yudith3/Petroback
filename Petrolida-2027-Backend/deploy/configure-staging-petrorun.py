"""Manage synthetic PetroRun settings on the isolated staging deployment."""

from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import time
import urllib.request


STAGING_DIRECTORY = Path("/srv/petrolida-staging")
TEST_BANK = "STAGING TEST BANK - JANGAN TRANSFER"
WIB = timezone(timedelta(hours=7))


def replace_environment(path: Path, values: dict[str, str | None]) -> None:
    original = path.read_bytes().decode("utf-8")
    ending = "\r\n" if "\r\n" in original else "\n"
    lines, seen = [], set()
    for line in original.splitlines(keepends=True):
        key = line.partition("=")[0]
        if key not in values:
            lines.append(line)
        elif key not in seen:
            if values[key] is not None:
                lines.append(f"{key}={values[key]}{ending}")
            seen.add(key)
    if lines and not lines[-1].endswith("\n"):
        lines[-1] += ending
    for key, value in values.items():
        if key not in seen and value is not None:
            lines.append(f"{key}={value}{ending}")
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", newline="", dir=path.parent, prefix=".env.petrorun.", delete=False) as output:
            temporary = output.name
            output.write("".join(lines))
            output.flush()
            os.fsync(output.fileno())
        os.chmod(temporary, 0o600)
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            os.unlink(temporary)


def test_values(now: datetime) -> dict[str, str]:
    # Open staging now, including outside the official sales period. Preserve
    # the official late/closing/racepack dates whenever they are still ahead.
    day = now.astimezone(WIB).replace(hour=0, minute=0, second=0, microsecond=0)
    late = max(datetime.fromisoformat("2026-11-19T00:00:00+07:00"), day + timedelta(days=27))
    end = max(datetime.fromisoformat("2026-11-23T00:00:00+07:00"), late + timedelta(days=4))
    racepack = max(datetime.fromisoformat("2026-11-25T00:00:00+07:00"), end + timedelta(days=2))
    return {
        "PETRORUN_TEST_MODE": "true",
        "PETRORUN_BASE_PRICE_IDR": "150000",
        "PETRORUN_ADMIN_FEE_IDR": "0",
        "PETRORUN_JERSEY_SURCHARGE_2XL_IDR": "0",
        "PETRORUN_JERSEY_SURCHARGE_3XL_IDR": "0",
        "PETRORUN_JERSEY_SURCHARGE_4XL_IDR": "0",
        "PETRORUN_BANK_NAME": TEST_BANK,
        "PETRORUN_BANK_ACCOUNT_NUMBER": "0000000000",
        "PETRORUN_BANK_ACCOUNT_HOLDER": "PETROLIDA STAGING TEST ONLY",
        "PETRORUN_EARLY_BIRD_PRICE_IDR": "125000",
        "PETRORUN_EARLY_BIRD_QUOTA": "25",
        "PETRORUN_LATE_PRICE_IDR": "139000",
        "PETRORUN_REGISTRATION_START_AT": (day - timedelta(days=1)).isoformat(),
        "PETRORUN_REGISTRATION_END_AT": end.isoformat(),
        "PETRORUN_LATE_START_AT": late.isoformat(),
        "PETRORUN_RACEPACK_AT": racepack.isoformat(),
    }


def configure(environment: Path, rendered: dict[str, str], mode: str, now: datetime) -> None:
    if mode not in ("enable", "restore"):
        raise ValueError("mode must be enable or restore")
    if rendered.get("APP_NAME") != "petrolida-staging-api":
        raise ValueError("refusing to modify a non-staging application")
    backup = environment.parent / ".petrorun-before-test.json"
    raw = dict(line.split("=", 1) for line in environment.read_text(encoding="utf-8").splitlines() if line.startswith("PETRORUN_") and "=" in line)
    if mode == "restore":
        previous = json.loads(backup.read_text(encoding="utf-8"))
        replace_environment(environment, {**{key: None for key in raw}, **previous})
        return
    if not backup.exists():
        with backup.open("x", encoding="utf-8") as output:
            os.chmod(backup, 0o600)
            json.dump(raw, output)
    values = test_values(now)
    if not rendered.get("PETRORUN_ACCESS_SECRET"):
        values["PETRORUN_ACCESS_SECRET"] = secrets.token_urlsafe(48)
    # Preserve existing credentials. A missing bootstrap account is provisioned
    # on backend restart using a new random password, never a shared default.
    if not rendered.get("ADMIN_EMAIL"):
        values["ADMIN_EMAIL"] = "admin@petrolida.com"
    if not rendered.get("ADMIN_NAME"):
        values["ADMIN_NAME"] = "Staging Admin"
    if not rendered.get("ADMIN_PASSWORD"):
        values["ADMIN_PASSWORD"] = secrets.token_urlsafe(24)
    replace_environment(environment, values)


def compose(*arguments: str, capture: bool = False) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", "compose", "--env-file", ".env", "-f", "compose.staging.yaml", *arguments], cwd=STAGING_DIRECTORY, check=True, capture_output=capture, text=True)


def rendered_environment() -> dict[str, str]:
    return json.loads(compose("config", "--format", "json", capture=True).stdout)["services"]["backend"]["environment"]


def main() -> None:
    import fcntl

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("enable", "restore"), default="enable")
    parser.add_argument("--export-admin", action="store_true")
    args = parser.parse_args()
    with (STAGING_DIRECTORY / ".deploy.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        rendered = rendered_environment()
        if rendered.get("APP_NAME") != "petrolida-staging-api":
            raise ValueError("refusing to access a non-staging application")
        if args.export_admin:
            # Caller MUST pipe this directly to encryption. Never write this
            # output to a CI log or a plaintext artifact.
            json.dump({"email": rendered.get("ADMIN_EMAIL", ""), "password": rendered.get("ADMIN_PASSWORD", "")}, sys.stdout)
            return
        environment = STAGING_DIRECTORY / ".env"
        original = environment.read_bytes()
        try:
            configure(environment, rendered, args.mode, datetime.now(WIB))
            compose("config", "--quiet", capture=True)
            compose("up", "-d", "--no-deps", "--force-recreate", "backend", capture=True)
            for _ in range(90):
                try:
                    with urllib.request.urlopen("http://127.0.0.1:28998/api/v1/health", timeout=4):
                        if args.mode == "restore":
                            (STAGING_DIRECTORY / ".petrorun-before-test.json").unlink()
                        print(f"Staging PetroRun test configuration: {args.mode}; backend healthy")
                        return
                except OSError:
                    time.sleep(2)
            raise RuntimeError("backend readiness failed")
        except Exception:
            # Restore the previous runtime configuration if recreation fails.
            environment.write_bytes(original)
            os.chmod(environment, 0o600)
            compose("up", "-d", "--no-deps", "--force-recreate", "backend", capture=True)
            raise


if __name__ == "__main__":
    main()
