import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "set-staging-announcement.py"
SPEC = importlib.util.spec_from_file_location("staging_announcement", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class StagingAnnouncementTest(unittest.TestCase):
    def test_staff_release_preserves_interview_and_other_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            original = "DB_PASS=sample=a=b\nOPREC_INTERVIEW_ANNOUNCEMENT_AT=2026-10-08T00:00:00+07:00\nOPREC_ANNOUNCEMENT_AT=old\nOPREC_ANNOUNCEMENT_AT=duplicate\nBACKEND_TAG=existing\n"
            environment.write_text(original, encoding="utf-8")
            MODULE.set_release(environment, "2026-10-15T00:00:00+07:00", stage="staff")
            expected = original.replace("OPREC_ANNOUNCEMENT_AT=old\nOPREC_ANNOUNCEMENT_AT=duplicate\n", "OPREC_ANNOUNCEMENT_AT=2026-10-15T00:00:00+07:00\n")
            self.assertEqual(environment.read_text(encoding="utf-8"), expected)
            MODULE.set_release(environment, "2026-10-15T00:00:00+07:00", stage="staff")
            self.assertEqual(environment.read_text(encoding="utf-8"), expected)

    def test_invalid_stage_never_changes_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            original = b"DB_PASS=sample\n"
            environment.write_bytes(original)
            with self.assertRaises(ValueError):
                MODULE.set_release(environment, "2026-10-15T00:00:00+07:00", stage="production")
            self.assertEqual(environment.read_bytes(), original)

    def test_append_preserves_existing_settings_and_missing_final_newline(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            original = "# staging configuration\nDB_PASS=sample=a=b\nJWT_SECRET=sample-secret"
            environment.write_text(original, encoding="utf-8")
            MODULE.set_release(environment, "2026-10-08T00:00:00+07:00")
            self.assertEqual(environment.read_text(encoding="utf-8"), original + "\nOPREC_INTERVIEW_ANNOUNCEMENT_AT=2026-10-08T00:00:00+07:00\n")

    def test_update_removes_duplicate_release_keys_and_is_idempotent(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            environment.write_text("DB_PASS=sample\nOPREC_INTERVIEW_ANNOUNCEMENT_AT=old\nOPREC_INTERVIEW_ANNOUNCEMENT_AT=duplicate\nBACKEND_TAG=existing\n", encoding="utf-8")
            MODULE.set_release(environment, "2026-10-08T00:00:00+07:00")
            first = environment.read_bytes()
            MODULE.set_release(environment, "2026-10-08T00:00:00+07:00")
            self.assertEqual(environment.read_bytes(), first)
            self.assertEqual(first.count(b"OPREC_INTERVIEW_ANNOUNCEMENT_AT="), 1)
            self.assertIn("DB_PASS=sample\n", environment.read_text(encoding="utf-8"))
            self.assertIn("BACKEND_TAG=existing\n", environment.read_text(encoding="utf-8"))

    def test_preserves_crlf_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            original = b"DB_PASS=sample=a=b\r\nBACKEND_TAG=existing\r\n"
            environment.write_bytes(original)
            MODULE.set_release(environment, "2026-10-08T00:00:00+07:00")
            self.assertEqual(environment.read_bytes(), original + b"OPREC_INTERVIEW_ANNOUNCEMENT_AT=2026-10-08T00:00:00+07:00\r\n")

    def test_invalid_release_never_changes_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            environment = Path(directory) / ".env"
            original = b"DB_PASS=sample\n"
            environment.write_bytes(original)
            for value in ["", "2026-10-08T00:00:00", "2026-02-30T00:00:00+07:00", "2026-10-08T00:00:00+07:00; echo sample"]:
                with self.subTest(value=value), self.assertRaises(ValueError):
                    MODULE.set_release(environment, value)
                self.assertEqual(environment.read_bytes(), original)


if __name__ == "__main__":
    unittest.main()
