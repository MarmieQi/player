"""Validate deployment interpolation without starting Docker or reading a real .env."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
TEMPLATE = (ROOT / ".env.example").read_text(encoding="utf-8")
COMPOSE = (ROOT / "docker-compose.yml").read_text(encoding="utf-8")
KEYS = set(re.findall(r"^([A-Z][A-Z0-9_]*)=", TEMPLATE, re.M))
REQUIRED = {
    "MYSQL_PASSWORD": "fixture-user-password",
    "MYSQL_ROOT_PASSWORD": "fixture-root-password",
    "APP_ADMIN_PASSWORD": "fixture-admin-password",
    "APP_ADMIN_JWT_SECRET": "fixture-jwt-secret-not-for-production",
}


def render(values):
    environment = os.environ.copy()
    for key in KEYS | {
        "COMPOSE_FILE", "COMPOSE_PROFILES", "COMPOSE_PROJECT_NAME",
        "COMPOSE_ENV_FILES", "COMPOSE_DISABLE_ENV_FILE",
    }:
        environment.pop(key, None)
    with tempfile.TemporaryDirectory(prefix="player-compose-check-") as folder:
        env_file = Path(folder) / "fixture.env"
        env_file.write_text("\n".join(f"{key}='{value}'" for key, value in values.items()), encoding="utf-8")
        return subprocess.run([
            "docker", "compose", "--project-directory", str(ROOT),
            "--env-file", str(env_file), "-p", "player-config-check",
            "-f", str(ROOT / "docker-compose.yml"), "config", "--format", "json",
        ], cwd=ROOT, env=environment, capture_output=True, text=True, encoding="utf-8")


class ComposeConfigTests(unittest.TestCase):
    def config(self, overrides=None):
        result = render(REQUIRED | (overrides or {}))
        self.assertEqual(result.returncode, 0, result.stderr)
        # Compose config escapes every $ as $$ so its output can be loaded again.
        # Undo that serialization layer before checking the resolved values.
        # See docker/compose cmd/compose/config.go, runConfig.
        return json.loads(result.stdout.replace("$$", "$"))["services"]

    def test_every_template_setting_is_documented_and_connected(self):
        used = set(re.findall(r"\$\{([A-Z][A-Z0-9_]*)", COMPOSE))
        self.assertEqual(KEYS, used)
        lines = TEMPLATE.splitlines()
        for index, line in enumerate(lines):
            if re.match(r"^[A-Z][A-Z0-9_]*=", line):
                previous = index - 1
                while previous >= 0 and re.match(r"^[A-Z][A-Z0-9_]*=", lines[previous]):
                    previous -= 1
                self.assertTrue(previous >= 0 and lines[previous].startswith("#"), line)

    def test_missing_required_values_fail_before_starting_services(self):
        for key in REQUIRED:
            with self.subTest(key=key):
                values = dict(REQUIRED)
                values.pop(key)
                self.assertNotEqual(render(values).returncode, 0)
        with self.subTest(empty_template=True):
            values = dict(re.findall(r"^([A-Z][A-Z0-9_]*)=(.*)$", TEMPLATE, re.M))
            self.assertNotEqual(render(values).returncode, 0)

    def test_default_deployment_reuses_database_credentials(self):
        services = self.config()
        mysql = services["mysql"]["environment"]
        backend = services["backend"]["environment"]
        self.assertEqual(mysql["MYSQL_PASSWORD"], backend["PLAYER_STATS_DATASOURCE_PASSWORD"])
        self.assertEqual(mysql["MYSQL_USER"], backend["PLAYER_STATS_DATASOURCE_USERNAME"])
        self.assertEqual(backend["PLAYER_STATS_DATASOURCE_URL"], "jdbc:mysql://mysql:3306/player_stats")
        self.assertEqual(services["frontend"]["image"], "ghcr.io/marmieqi/player-frontend:latest")
        self.assertEqual(services["backend"]["image"], "ghcr.io/marmieqi/player-backend:latest")
        self.assertEqual([p["published"] for p in services["frontend"]["ports"]], ["9493", "8843"])

    def test_custom_values_keep_versions_paths_and_passwords_consistent(self):
        password = "fixture $dollar # hash : colon"
        tag = "1234567890abcdef"
        services = self.config({
            "MYSQL_PASSWORD": password, "MYSQL_DATABASE": "custom_stats", "MYSQL_USER": "custom_user",
            "IMAGE_TAG": tag, "PUBLIC_PORT": "19493", "ADMIN_PORT": "18843",
            "DATA_DIRECTORY": "./custom-data", "PLAYER_LOGS_LIVE_CACHE_SECONDS": "60",
        })
        backend = services["backend"]["environment"]
        self.assertEqual(backend["PLAYER_STATS_DATASOURCE_PASSWORD"], password)
        self.assertEqual(services["mysql"]["environment"]["MYSQL_PASSWORD"], password)
        self.assertEqual(backend["PLAYER_STATS_DATASOURCE_URL"], "jdbc:mysql://mysql:3306/custom_stats")
        self.assertEqual(backend["PLAYER_STATS_DATASOURCE_USERNAME"], "custom_user")
        self.assertEqual(backend["PLAYER_LOGS_LIVE_CACHE_SECONDS"], "60")
        for name in ["frontend", "backend"]:
            self.assertTrue(services[name]["image"].endswith(":" + tag))
        self.assertEqual([p["published"] for p in services["frontend"]["ports"]], ["19493", "18843"])
        self.assertEqual(Path(services["mysql"]["volumes"][0]["source"]).resolve(), (ROOT / "custom-data/mysql").resolve())
        self.assertIn('"${MYSQL_ROOT_PASSWORD}"', services["mysql"]["healthcheck"]["test"][1])

    def test_retired_business_environment_is_not_forwarded(self):
        services = self.config({
            "PLAYER_LOGS_SMB_HOST": "retired-host",
            "PLAYER_LOGS_SMB_PASSWORD": "retired-password",
            "PLAYER_LOGS_SMB_DIRECTORY": "retired-directory",
            "PLAYER_LOGS_IMPORTER_SKIP_TODAY": "false",
        })
        backend = services["backend"]["environment"]
        self.assertFalse(any(key.startswith("PLAYER_LOGS_SMB_") for key in backend))
        self.assertNotIn("PLAYER_LOGS_IMPORTER_SKIP_TODAY", backend)

    def test_forwarded_backend_environment_is_actually_read(self):
        services = self.config()
        source = (ROOT / "backend-go/internal/config/config.go").read_text(encoding="utf-8")
        recognized = set(re.findall(r'(?:env|csvEnv|intEnv|boolEnv)\("([A-Z][A-Z0-9_]*)"', source))
        forwarded = set(services["backend"]["environment"]) - {"TZ"}
        self.assertFalse(forwarded - recognized, f"Unread backend settings: {forwarded - recognized}")


if __name__ == "__main__":
    unittest.main(verbosity=2)
