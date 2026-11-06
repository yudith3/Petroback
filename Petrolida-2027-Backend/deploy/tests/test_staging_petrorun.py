import importlib.util
from datetime import datetime
import json
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "configure-staging-petrorun.py"
SPEC = importlib.util.spec_from_file_location("staging_petrorun", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
NOW = datetime.fromisoformat("2026-10-19T12:00:00+07:00")


class StagingPetroRunTest(unittest.TestCase):
    def test_enable_preserves_database_existing_credentials_and_order_secret(self):
        with tempfile.TemporaryDirectory() as directory:
            env = Path(directory) / ".env"
            original = "DB_PASS=secret=a=b\nBACKEND_TAG=existing\nADMIN_EMAIL=admin@petrolida.com\nADMIN_PASSWORD=existing-password\nPETRORUN_ACCESS_SECRET=existing-order-secret\nPETRORUN_BANK_NAME=old\nPETRORUN_BANK_NAME=duplicate\n"
            env.write_text(original, encoding="utf-8")
            rendered = {"APP_NAME": "petrolida-staging-api", "ADMIN_EMAIL": "admin@petrolida.com", "ADMIN_PASSWORD": "existing-password", "ADMIN_NAME": "Existing Admin", "PETRORUN_ACCESS_SECRET": "existing-order-secret"}
            MODULE.configure(env, rendered, "enable", NOW)
            result = env.read_text(encoding="utf-8")
            self.assertIn("DB_PASS=secret=a=b\nBACKEND_TAG=existing\n", result)
            self.assertIn("ADMIN_PASSWORD=existing-password\n", result)
            self.assertIn("PETRORUN_ACCESS_SECRET=existing-order-secret\n", result)
            self.assertIn("PETRORUN_TEST_MODE=true\n", result)
            self.assertIn("PETRORUN_BANK_NAME=" + MODULE.TEST_BANK + "\n", result)
            self.assertEqual(result.count("PETRORUN_BANK_NAME="), 1)
            first_backup = (env.parent / ".petrorun-before-test.json").read_bytes()
            MODULE.configure(env, rendered, "enable", NOW)
            self.assertEqual(env.read_text(encoding="utf-8"), result)
            self.assertEqual((env.parent / ".petrorun-before-test.json").read_bytes(), first_backup)

    def test_missing_bootstrap_admin_and_access_secret_get_random_values(self):
        with tempfile.TemporaryDirectory() as directory:
            env = Path(directory) / ".env"
            env.write_bytes(b"DB_PASS=unchanged\r\nJWT_SECRET=keep-without-final-newline")
            MODULE.configure(env, {"APP_NAME": "petrolida-staging-api"}, "enable", NOW)
            content = env.read_bytes()
            self.assertIn(b"DB_PASS=unchanged\r\nJWT_SECRET=keep-without-final-newline\r\n", content)
            values = dict(line.split("=", 1) for line in content.decode().splitlines())
            self.assertEqual(values["ADMIN_EMAIL"], "admin@petrolida.com")
            self.assertGreaterEqual(len(values["ADMIN_PASSWORD"]), 32)
            self.assertGreaterEqual(len(values["PETRORUN_ACCESS_SECRET"]), 32)
            self.assertNotEqual(values["ADMIN_PASSWORD"], values["PETRORUN_ACCESS_SECRET"])

    def test_restore_reinstates_original_event_settings_without_changing_admin(self):
        with tempfile.TemporaryDirectory() as directory:
            env = Path(directory) / ".env"
            env.write_text("DB_PASS=unchanged\nPETRORUN_BASE_PRICE_IDR=\nPETRORUN_ACCESS_SECRET='original-secret'\n", encoding="utf-8")
            rendered = {"APP_NAME": "petrolida-staging-api"}
            MODULE.configure(env, rendered, "enable", NOW)
            MODULE.configure(env, rendered, "restore", NOW)
            content = env.read_text(encoding="utf-8")
            self.assertIn("PETRORUN_BASE_PRICE_IDR=\n", content)
            self.assertIn("PETRORUN_ACCESS_SECRET='original-secret'\n", content)
            self.assertNotIn("PETRORUN_TEST_MODE", content)
            self.assertNotIn("PETRORUN_BANK_NAME", content)
            self.assertIn("ADMIN_PASSWORD=", content)
            self.assertIn("DB_PASS=unchanged\n", content)

    def test_production_identity_and_unknown_mode_never_change_files(self):
        with tempfile.TemporaryDirectory() as directory:
            env = Path(directory) / ".env"
            original = b"DB_PASS=unchanged\n"
            env.write_bytes(original)
            for mode, identity in [("enable", "petrolida-2027-api"), ("invalid", "petrolida-staging-api")]:
                with self.assertRaises(ValueError):
                    MODULE.configure(env, {"APP_NAME": identity}, mode, NOW)
                self.assertEqual(env.read_bytes(), original)
                self.assertFalse((env.parent / ".petrorun-before-test.json").exists())

    def test_test_window_is_open_now_and_all_dates_stay_ordered(self):
        for now in [NOW, datetime.fromisoformat("2026-12-01T23:59:00+07:00")]:
            values = MODULE.test_values(now)
            dates = [datetime.fromisoformat(values["PETRORUN_" + field]) for field in ["REGISTRATION_START_AT", "LATE_START_AT", "REGISTRATION_END_AT", "RACEPACK_AT"]]
            self.assertLess(dates[0], now)
            self.assertLess(now, dates[1])
            self.assertEqual(dates, sorted(dates))
            self.assertEqual(values["PETRORUN_BANK_ACCOUNT_NUMBER"], "0000000000")
        official = MODULE.test_values(NOW)
        self.assertEqual(official["PETRORUN_REGISTRATION_END_AT"], "2026-11-23T00:00:00+07:00")


if __name__ == "__main__":
    unittest.main()
