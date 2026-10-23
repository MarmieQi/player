"""Regression checks for the one-time migration; fake credentials and no Docker daemon."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("migration", ROOT / "scripts/migrate_legacy_config.py")
migration = importlib.util.module_from_spec(spec)
spec.loader.exec_module(migration)
FIXTURE = (ROOT / "scripts/tests/fixtures/legacy-compose.yml").read_text(encoding="utf-8")


class MigrationTests(unittest.TestCase):
    def run_fixture(self, source, action):
        with tempfile.TemporaryDirectory(prefix="player-legacy-test-") as folder:
            root = Path(folder)
            old = root / "old-compose.yml"
            old.write_text(source, encoding="utf-8")
            return action(root, old)

    def test_migrates_values_and_comments_without_touching_source(self):
        def check(root, old):
            before = old.read_bytes()
            output = root / "migrated.env"
            migration.migrate(old, output)
            self.assertEqual(old.read_bytes(), before)
            content = output.read_text(encoding="utf-8")
            self.assertIn("# 登录令牌签名密钥", content)
            self.assertIn('APP_ADMIN_JWT_SECRET="fixture-jwt-secret-keep-unchanged"', content)
            self.assertIn('MYSQL_PASSWORD="fixture-db-password"', content)
            self.assertIn("COMPOSE_PROJECT_NAME=", content)
            self.assertNotIn("PLAYER_LOGS_SMB_ENABLED=", content)
            if os.name != "nt":
                self.assertEqual(output.stat().st_mode & 0o777, 0o600)
        self.run_fixture(FIXTURE, check)

    def test_older_compose_without_live_settings_retains_runtime_defaults(self):
        fixture = "\n".join(line for line in FIXTURE.splitlines() if "PLAYER_LOGS_LIVE_" not in line)
        def check(root, old):
            output = root / "migrated.env"
            migration.migrate(old, output)
            actual = migration.model(migration.COMPOSE, root, output)["services"]["backend"]["environment"]
            for key, default in migration.LEGACY_LIVE_DEFAULTS.items():
                self.assertEqual(actual[key], default)
        self.run_fixture(fixture, check)

    def test_explicit_live_cache_and_directory_are_not_replaced(self):
        fixture = FIXTURE.replace("PLAYER_LOGS_LIVE_TEMP_DIRECTORY: /data/temp/live-log-query", "PLAYER_LOGS_LIVE_TEMP_DIRECTORY: /data/temp/custom-live")
        fixture = fixture.replace("PLAYER_LOGS_LIVE_CACHE_SECONDS: 120", "PLAYER_LOGS_LIVE_CACHE_SECONDS: 75")
        def check(root, old):
            output = root / "migrated.env"
            migration.migrate(old, output)
            actual = migration.model(migration.COMPOSE, root, output)["services"]["backend"]["environment"]
            self.assertEqual(actual["PLAYER_LOGS_LIVE_CACHE_SECONDS"], "75")
            self.assertEqual(actual["PLAYER_LOGS_LIVE_TEMP_DIRECTORY"], "/data/temp/custom-live")
        self.run_fixture(fixture, check)

    def test_does_not_overwrite_existing_output(self):
        def check(root, old):
            output = root / ".env"
            output.write_text("existing-private-config", encoding="utf-8")
            with self.assertRaises(migration.MigrationError):
                migration.migrate(old, output)
            self.assertEqual(output.read_text(), "existing-private-config")
        self.run_fixture(FIXTURE, check)

    def test_special_characters_and_old_custom_values_survive(self):
        # Dollars, hashes, apostrophes and backslashes must remain literal.
        password = "fixture $token # value ' quoted \\ suffix\\"
        fixture = FIXTURE.replace("fixture-db-password", json.dumps(password))
        fixture = fixture.replace('"9493:80"', '"19493:80"').replace('"8843:8081"', '"18843:8081"')
        fixture = fixture.replace(":latest", ":abc123").replace("./docker-data/", "./saved-data/")
        def check(root, old):
            output = root / "migrated.env"
            migration.migrate(old, output)
            actual = migration.model(migration.COMPOSE, root, output)["services"]
            self.assertEqual(actual["backend"]["environment"]["PLAYER_STATS_DATASOURCE_PASSWORD"], password)
            self.assertEqual(actual["mysql"]["environment"]["MYSQL_PASSWORD"], password)
            self.assertTrue(actual["frontend"]["image"].endswith(":abc123"))
            self.assertEqual(actual["frontend"]["ports"][0]["published"], "19493")
            self.assertEqual(Path(actual["mysql"]["volumes"][0]["source"]).resolve(), (root / "saved-data/mysql").resolve())
        self.run_fixture(fixture, check)

    def test_refuses_settings_the_new_template_cannot_preserve(self):
        variants = {
            "different_database_passwords": FIXTURE.replace("PLAYER_STATS_DATASOURCE_PASSWORD: fixture-db-password", "PLAYER_STATS_DATASOURCE_PASSWORD: other-value"),
            "different_image_tags": FIXTURE.replace("player-backend:latest", "player-backend:other"),
            "host_bound_port": FIXTURE.replace('"9493:80"', '"127.0.0.1:9493:80"'),
            "custom_setting": FIXTURE.replace("SERVER_PORT: 8080", "SERVER_PORT: 9090"),
            "separate_volume_root": FIXTURE.replace("./docker-data/mysql:", "./separate/mysql:"),
            "external_variable": FIXTURE.replace("APP_ADMIN_JWT_SECRET: fixture-jwt-secret-keep-unchanged", "APP_ADMIN_JWT_SECRET: ${OLD_JWT_SECRET}"),
        }
        for name, fixture in variants.items():
            with self.subTest(name=name):
                def check(root, old):
                    output = root / "migrated.env"
                    with self.assertRaises(migration.MigrationError) as caught:
                        migration.migrate(old, output)
                    self.assertFalse(output.exists())
                    self.assertNotIn("fixture-db-password", str(caught.exception))
                    self.assertNotIn("fixture-jwt-secret-keep-unchanged", str(caught.exception))
                self.run_fixture(fixture, check)


if __name__ == "__main__":
    unittest.main(verbosity=2)
