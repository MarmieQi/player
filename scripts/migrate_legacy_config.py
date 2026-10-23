"""One-time legacy Compose -> commented .env migration; never starts containers."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
EXAMPLE = ROOT / ".env.example"
COMPOSE = ROOT / "docker-compose.yml"
RETIRED = {
    "PLAYER_LOGS_IMPORTER_AUTO_RUN", "PLAYER_LOGS_IMPORTER_CRON",
    "PLAYER_LOGS_MAIN_TYPE", "PLAYER_LOGS_SUB_TYPE",
    "PLAYER_LOGS_MAIN_RECURSIVE", "PLAYER_LOGS_SUB_RECURSIVE",
    "PLAYER_LOGS_LOCAL_DIRECTORY", "PLAYER_LOGS_SMB_ENABLED",
}
REQUIRED = {"MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD", "APP_ADMIN_PASSWORD", "APP_ADMIN_JWT_SECRET"}
# Defaults of the Go backend in the original Linux/Alpine container when unset.
LEGACY_LIVE_DEFAULTS = {
    "PLAYER_LOGS_LIVE_CACHE_SECONDS": "120",
    "PLAYER_LOGS_LIVE_TEMP_DIRECTORY": "/tmp/player-stats-live-log-query",
}


class MigrationError(Exception):
    pass


def quote(value):
    value = str(value)
    if any(char in value for char in "\n\r\0"):
        raise MigrationError("配置包含换行或空字符，请手动迁移该值。")
    # Compose dotenv accepts JSON-style double quotes; $$ prevents interpolation.
    return json.dumps(value, ensure_ascii=False).replace("$", "$$")


def clean_environment():
    environment = os.environ.copy()
    keys = set(re.findall(r"^([A-Z][A-Z0-9_]*)=", EXAMPLE.read_text(encoding="utf-8"), re.M))
    for key in keys | {"COMPOSE_FILE", "COMPOSE_PROJECT_NAME", "COMPOSE_PROFILES", "COMPOSE_ENV_FILES", "COMPOSE_DISABLE_ENV_FILE"}:
        environment.pop(key, None)
    return environment


def model(path, directory, env_file, *, literal=False, project_name=None):
    command = ["docker", "compose", "--project-directory", str(directory), "--env-file", str(env_file)]
    if project_name:
        command += ["-p", project_name]
    command += ["-f", str(path), "config", "--no-env-resolution", "--format", "json"]
    if literal:
        command += ["--no-interpolate"]
    try:
        result = subprocess.run(command, cwd=directory, env=clean_environment(), capture_output=True,
                                text=True, encoding="utf-8", timeout=60)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise MigrationError("无法运行 Docker Compose 配置解析，请检查 Docker CLI / Compose 是否可用。") from error
    if result.returncode:
        # Docker diagnostics can contain credentials; do not echo them.
        raise MigrationError("Compose 配置无法解析，请在本机检查输入文件；为避免泄露密码，不输出解析原文。")
    # The interpolated config command escapes $ for reusable output; literal mode does not.
    output = result.stdout if literal else result.stdout.replace("$$", "$")
    return json.loads(output)


def text(value):
    if isinstance(value, bool):
        return str(value).lower()
    return str(value)


def extract(old):
    services = old.get("services", {})
    if set(services) != {"mysql", "backend", "frontend"}:
        raise MigrationError("仅支持项目原有的 mysql/backend/frontend 三个服务；额外服务需手动合并。")
    database = services["mysql"].get("environment", {})
    backend = services["backend"].get("environment", {})
    if not isinstance(database, dict) or not isinstance(backend, dict):
        raise MigrationError("无法识别旧配置的环境变量格式。")
    for key, value in list(database.items()) + list(backend.items()):
        if value is None or "${" in text(value):
            raise MigrationError(f"{key} 仍引用外部变量，请先将旧配置中的该项解析为实际值，再迁移。")
    defaults = dict(re.findall(r"^([A-Z][A-Z0-9_]*)=(.*)$", EXAMPLE.read_text(encoding="utf-8"), re.M))
    values = defaults | {key: text(value) for key, value in backend.items() if key in defaults}
    for key, default in LEGACY_LIVE_DEFAULTS.items():
        if key not in backend:
            values[key] = default
    for key in ["MYSQL_DATABASE", "MYSQL_USER", "MYSQL_PASSWORD", "MYSQL_ROOT_PASSWORD"]:
        if key not in database:
            raise MigrationError(f"旧配置缺少 {key}，请补充原值后再迁移。")
        values[key] = text(database[key])
    if backend.get("PLAYER_STATS_DATASOURCE_USERNAME") != values["MYSQL_USER"]:
        raise MigrationError("原 MySQL 与后端数据库用户名不一致，不能自动合并为单一配置。")
    if backend.get("PLAYER_STATS_DATASOURCE_PASSWORD") != values["MYSQL_PASSWORD"]:
        raise MigrationError("原 MySQL 与后端数据库密码不一致，请核对数据库实际密码后手动处理。")
    url = text(backend.get("PLAYER_STATS_DATASOURCE_URL", ""))
    parsed = urlsplit(url.removeprefix("jdbc:"))
    if parsed.scheme != "mysql" or parsed.netloc != "mysql:3306" or parsed.path != "/" + values["MYSQL_DATABASE"]:
        raise MigrationError("旧数据库连接不是模板内置 MySQL 或库名不一致，需保留自定义连接配置。")
    for key in REQUIRED:
        if not values.get(key, "").strip():
            raise MigrationError(f"旧配置缺少必填值 {key}，不会自动生成或更换密码。")
    tags = []
    for name in ["frontend", "backend"]:
        image = services[name].get("image", "")
        prefix = f"ghcr.io/marmieqi/player-{name}:"
        if not image.startswith(prefix) or "@" in image:
            raise MigrationError(f"{name} 使用自定义镜像或摘要，请手动保留镜像配置。")
        tags.append(image[len(prefix):])
    if tags[0] != tags[1]:
        raise MigrationError("前后端镜像标签不同，无法安全合并为 IMAGE_TAG；请先确认配套版本。")
    values["IMAGE_TAG"] = tags[0]
    ports = services["frontend"].get("ports", [])
    for target, key in [(80, "PUBLIC_PORT"), (8081, "ADMIN_PORT")]:
        matches = [port for port in ports if isinstance(port, dict) and port.get("target") == target]
        if len(matches) != 1 or not matches[0].get("published"):
            raise MigrationError(f"无法识别 {key} 的端口映射，请手动处理。")
        values[key] = text(matches[0]["published"])
    roots = []
    for service, target, suffix in [("mysql", "/var/lib/mysql", "mysql"),
                                     ("backend", "/data/synced-logs", "synced-logs"),
                                     ("backend", "/data/temp", "temp")]:
        matches = [volume for volume in services[service].get("volumes", [])
                   if isinstance(volume, dict) and volume.get("target") == target]
        if len(matches) != 1 or matches[0].get("type") != "bind":
            raise MigrationError("数据使用自定义或命名卷，不能自动转换为默认 DATA_DIRECTORY 布局。")
        source = Path(matches[0]["source"]).resolve()
        if source.name != suffix:
            raise MigrationError("数据挂载不符合 mysql/synced-logs/temp 子目录布局，请手动保留挂载。")
        roots.append(source.parent)
    if len(set(roots)) != 1:
        raise MigrationError("数据库、CSV、临时文件分散在不同目录，无法合并为单一 DATA_DIRECTORY。")
    # Keep absolute paths so moving the template cannot accidentally switch the database.
    values["DATA_DIRECTORY"] = roots[0].as_posix()
    return values


def commented_env(values, project_name):
    lines = []
    for line in EXAMPLE.read_text(encoding="utf-8").splitlines():
        match = re.match(r"^([A-Z][A-Z0-9_]*)=", line)
        lines.append(f"{match[1]}={quote(values[match[1]])}" if match else line)
    lines += ["", "# 工具已转义特殊字符；Compose 读取后仍为原值，请勿手工去掉转义。",
              "# 一次性迁移保留原 Compose 项目名，避免切换网络或创建另一套项目。",
              f"COMPOSE_PROJECT_NAME={quote(project_name)}", ""]
    return "\n".join(lines)


def differences(first, second, path=""):
    if isinstance(first, dict) and isinstance(second, dict):
        result = []
        for key in sorted(first.keys() | second.keys()):
            item = f"{path}.{key}" if path else key
            if key not in first or key not in second:
                result.append(item)
            else:
                result.extend(differences(first[key], second[key], item))
        return result
    return [] if first == second else [path]


def verify_equivalent(old, new):
    old = json.loads(json.dumps(old))
    new = json.loads(json.dumps(new))
    for item in [old, new]:
        for service in item["services"].values():
            service["environment"] = {key: text(value) for key, value in service.get("environment", {}).items()}
            for key in ["command", "entrypoint"]:
                if service.get(key) is None:
                    service.pop(key, None)
            for volume in service.get("volumes", []):
                if volume.get("type") == "bind":
                    volume["source"] = Path(volume["source"]).resolve().as_posix()
        for network in item.get("networks", {}).values():
            if network.get("ipam") == {}:
                network.pop("ipam")
    backend = old["services"]["backend"]["environment"]
    for key in RETIRED:
        backend.pop(key, None)
    # Current Go MySQLDSN ignores legacy JDBC query options; host, credentials and DB were checked above.
    backend["PLAYER_STATS_DATASOURCE_URL"] = new["services"]["backend"]["environment"]["PLAYER_STATS_DATASOURCE_URL"]
    for key, default in LEGACY_LIVE_DEFAULTS.items():
        backend.setdefault(key, default)
    # Newly explicit parameters retain the existing Go defaults when absent from old Compose.
    added_defaults = {"PLAYER_LOGS_XRAY_ANALYSIS_RETAINED_JOBS", "APP_CORS_ALLOWED_ORIGINS", "APP_CORS_ALLOWED_ORIGIN_PATTERNS"}
    for key in added_defaults:
        if key not in backend:
            new["services"]["backend"]["environment"].pop(key, None)
    # Healthcheck now quotes the same shell variable to support spaces in passwords.
    for item in [old, new]:
        check = item["services"]["mysql"].get("healthcheck", {}).get("test", [])
        if len(check) == 2 and check[0] == "CMD-SHELL":
            check[1] = check[1].replace("$$", "$").replace('-p"${MYSQL_ROOT_PASSWORD}"', '-p${MYSQL_ROOT_PASSWORD}')
    changed = differences(old, new)
    if changed:
        # Report field names only, never their potentially sensitive values.
        raise MigrationError("存在新模板无法完整保留的自定义配置，未写入 .env：" + ", ".join(changed[:15]))


def migrate(source, output, *, directory=None, project_name=None):
    source, output = Path(source).resolve(), Path(output).absolute()
    directory = Path(directory).resolve() if directory else source.parent
    if output.exists() or output.is_symlink():
        raise MigrationError("目标文件已存在，不会覆盖；请指定一个新的输出文件。")
    if not source.is_file():
        raise MigrationError("找不到旧 Compose 文件。")
    with tempfile.TemporaryDirectory(prefix="player-config-migration-") as folder:
        empty = Path(folder) / "empty.env"
        empty.write_text("", encoding="utf-8")
        old = model(source, directory, empty, literal=True, project_name=project_name)
        values = extract(old)
        content = commented_env(values, old["name"])
        candidate = Path(folder) / "candidate.env"
        candidate.write_text(content, encoding="utf-8")
        candidate.chmod(0o600)
        new = model(COMPOSE, directory, candidate, project_name=old["name"])
        # Literal mode is needed to preserve old passwords, but retains YAML scalar types
        # and implicit defaults. Canonicalize that resolved model with escaped literals
        # before comparing it with the new, interpolated model.
        resolved_old = Path(folder) / "resolved-old.json"
        resolved_old.write_text(json.dumps(old, ensure_ascii=False).replace("$", "$$"), encoding="utf-8")
        resolved_old.chmod(0o600)
        canonical_old = model(resolved_old, directory, empty, project_name=old["name"])
        verify_equivalent(canonical_old, new)
        # Exclusive creation guards against overwriting credentials, even if another process races us.
        descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(content)
    return output


def main():
    parser = argparse.ArgumentParser(description="一次性把旧 Compose 配置迁入带中文说明的 .env；不启动容器、不覆盖文件。")
    parser.add_argument("--from", dest="source", required=True, help="旧 docker-compose.yml 或备份文件")
    parser.add_argument("--output", default=".env", help="新配置输出路径，默认当前目录 .env；必须不存在")
    parser.add_argument("--project-directory", help="原部署目录；默认旧文件所在目录，用于正确解析相对挂载路径")
    parser.add_argument("--project-name", help="仅当原部署使用 docker compose -p 时，填写原项目名")
    args = parser.parse_args()
    try:
        output = migrate(args.source, args.output, directory=args.project_directory, project_name=args.project_name)
    except (MigrationError, OSError, ValueError) as error:
        print(f"迁移停止：{error}")
        return 1
    print(f"已生成：{output}")
    print("原文件、数据库及容器未修改。请本机核对，再使用新 Compose 模板部署；不要分享生成文件。")
    print("旧版无效开关不迁入；已有 SMB、任务及账号设置仍保留在数据库中。")
    return 0


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")
    raise SystemExit(main())
