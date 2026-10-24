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
    "APP_ADMIN_PASSWORD": "fixture-admin-password",
    "APP_ADMIN_JWT_SECRET": "fixture-jwt-secret-not-for-production",
}


def render(values, filename="docker-compose.yml"):
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
            "-f", str(ROOT / filename), "config", "--format", "json",
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

    def test_sqlite_deployment_has_only_application_services(self):
        services = self.config()
        self.assertEqual(set(services), {"backend", "frontend"})
        backend = services["backend"]["environment"]
        self.assertEqual(backend["PLAYER_DATABASE_PATH"], "/data/database/player.db")
        self.assertFalse(any("MYSQL" in key or "DATASOURCE" in key for key in backend))
        mounts = services["backend"]["volumes"]
        self.assertEqual({v["target"] for v in mounts}, {"/data/database", "/data/synced-logs", "/data/temp"})
        self.assertEqual([p["published"] for p in services["frontend"]["ports"]], ["9493", "8843"])

    def test_custom_version_and_data_root(self):
        services = self.config({"IMAGE_TAG": "fixture-sha", "DATA_DIRECTORY": "./custom-data", "PUBLIC_PORT": "19493", "ADMIN_PORT": "18843"})
        for name in ["backend", "frontend"]:
            self.assertTrue(services[name]["image"].endswith(":fixture-sha"))
        for mount in services["backend"]["volumes"]:
            self.assertTrue(Path(mount["source"]).resolve().is_relative_to((ROOT / "custom-data").resolve()))
        self.assertEqual([p["published"] for p in services["frontend"]["ports"]], ["19493", "18843"])

    def test_forwarded_backend_environment_is_actually_read(self):
        services = self.config()
        source = (ROOT / "backend-go/internal/config/config.go").read_text(encoding="utf-8")
        recognized = set(re.findall(r'(?:env|csvEnv|intEnv|boolEnv)\("([A-Z][A-Z0-9_]*)"', source))
        forwarded = set(services["backend"]["environment"]) - {"TZ"}
        self.assertFalse(forwarded - recognized, f"Unread backend settings: {forwarded - recognized}")


if __name__ == "__main__":
    unittest.main(verbosity=2)
