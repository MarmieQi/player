#!/bin/sh
# Run from the existing deployment directory. No database/file backups or deletes.
set -eu

release=${1:-}
mode=${2:-migrate}
if [ "${#release}" -ne 40 ]; then
  echo 'Usage: sh migrate-sqlite.sh <40-character release SHA> [--resume]' >&2
  exit 2
fi
case "$release" in *[!0-9a-f]*) echo 'Invalid release SHA' >&2; exit 2;; esac
case "$mode" in migrate|--resume) ;; *) echo 'Unknown migration option' >&2; exit 2;; esac
test -f .env || { echo 'Keep the original .env in this directory' >&2; exit 1; }
test -f docker-compose.yml || { echo 'Old docker-compose.yml is missing' >&2; exit 1; }
test ! -e docker-compose.mysql.yml || { echo 'An earlier cutover exists; inspect it before proceeding' >&2; exit 1; }
docker compose config --services | grep -qx mysql || { echo 'Expected the old MySQL deployment' >&2; exit 1; }

old_mysql=${MIGRATION_MYSQL_CONTAINER:-player-stats-mysql}
test "$(docker inspect -f '{{.State.Running}}' "$old_mysql")" = true || { echo 'Start the old MySQL container first' >&2; exit 1; }
if [ -z "${MIGRATION_NETWORK:-}" ]; then
  networks=$(docker inspect -f '{{range $name, $settings := .NetworkSettings.Networks}}{{println $name}}{{end}}' "$old_mysql")
  # Do not guess between multiple networks.
  set -- $networks
  test "$#" -eq 1 || { echo 'Set MIGRATION_NETWORK to the old MySQL Docker network' >&2; exit 1; }
  MIGRATION_NETWORK=$1
fi
export MIGRATION_NETWORK
MIGRATION_MYSQL_ADDRESS=${MIGRATION_MYSQL_ADDRESS:-$old_mysql:3306}
export MIGRATION_MYSQL_ADDRESS

base="https://raw.githubusercontent.com/MarmieQi/player/$release"
curl -fsSL "$base/docker-compose.yml" -o docker-compose.yml.sqlite
curl -fsSL "$base/compose.migrate.yml" -o compose.migrate.yml
IMAGE_TAG=$release docker compose -f docker-compose.yml.sqlite config --quiet
IMAGE_TAG=$release docker compose -f compose.migrate.yml config --quiet
# Pull all images before interrupting the old application.
IMAGE_TAG=$release docker compose -f docker-compose.yml.sqlite pull backend frontend
IMAGE_TAG=$release docker compose -f compose.migrate.yml pull config-migrate

echo 'Stopping the old application; MySQL and original log files stay in place.'
docker compose stop frontend backend
if [ "$mode" = migrate ]; then
  IMAGE_TAG=$release docker compose -f compose.migrate.yml run --rm config-migrate
else
  IMAGE_TAG=$release docker compose -f docker-compose.yml.sqlite run --rm --no-deps --entrypoint sh backend -ec 'test -s /data/database/player.db'
fi
# Each file is atomic. On failure, rerun this script with the same SHA and
# --resume only after configuration migration has completed successfully.
IMAGE_TAG=$release docker compose -f docker-compose.yml.sqlite run --rm --no-deps backend rebuild

mv docker-compose.yml docker-compose.mysql.yml
mv docker-compose.yml.sqlite docker-compose.yml
if grep -q '^IMAGE_TAG=' .env; then
  sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=$release/" .env
else
  printf '\nIMAGE_TAG=%s\n' "$release" >> .env
fi
IMAGE_TAG=$release docker compose up -d --wait --wait-timeout 120 backend frontend
docker compose exec -T backend wget -q -O - http://127.0.0.1:8080/api/health
printf '\nSQLite application is running. Sign in again and verify settings and statistics.\n'
printf 'Old MySQL remains running. After acceptance: docker stop %s\n' "$old_mysql"
