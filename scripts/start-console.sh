#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GOCACHE="${GOCACHE:-/private/tmp/nexis-go-cache}"
CONFIG="${NEXIS_CONFIG:-deploy/config/console.yaml}"
DB_HOST="${NEXIS_DB_HOST:-127.0.0.1}"
DB_PORT="${NEXIS_DB_PORT:-55432}"
DB_NAME="${NEXIS_DB_NAME:-nexis}"
DB_USER="${NEXIS_DB_USER:-nexis}"
DB_PASSWORD="${NEXIS_DB_PASSWORD:-nexis}"
LOCAL_PG_DIR="${NEXIS_LOCAL_PG_DIR:-data/postgres}"
LOCAL_PG_LOG="${NEXIS_LOCAL_PG_LOG:-data/postgres.log}"

wait_for_port() {
  local host="$1"
  local port="$2"
  local waited=0
  while ! nc -z "$host" "$port" >/dev/null 2>&1; do
    if [ "$waited" -ge 30 ]; then
      echo "PostgreSQL did not become ready on ${host}:${port}" >&2
      exit 1
    fi
    sleep 1
    waited=$((waited + 1))
  done
}

find_pg_bin() {
  if [ -n "${NEXIS_PG_BIN_DIR:-}" ] && [ -x "${NEXIS_PG_BIN_DIR}/postgres" ]; then
    echo "$NEXIS_PG_BIN_DIR"
    return 0
  fi
  if command -v postgres >/dev/null 2>&1; then
    dirname "$(command -v postgres)"
    return 0
  fi
  for dir in \
    /opt/homebrew/opt/postgresql@16/bin \
    /opt/homebrew/opt/postgresql@15/bin \
    /usr/local/opt/postgresql@16/bin \
    /usr/local/opt/postgresql@15/bin; do
    if [ -x "$dir/postgres" ]; then
      echo "$dir"
      return 0
    fi
  done
  return 1
}

ensure_local_postgres() {
  local pg_bin="$1"
  mkdir -p "$(dirname "$LOCAL_PG_DIR")"
  if [ ! -s "$LOCAL_PG_DIR/PG_VERSION" ]; then
    LC_ALL="${LC_ALL:-en_US.UTF-8}" "$pg_bin/initdb" -D "$LOCAL_PG_DIR" --auth=trust
  fi
  if ! "$pg_bin/pg_isready" -h "$DB_HOST" -p "$DB_PORT" >/dev/null 2>&1; then
    LC_ALL="${LC_ALL:-en_US.UTF-8}" "$pg_bin/pg_ctl" -D "$LOCAL_PG_DIR" -l "$LOCAL_PG_LOG" -o "-p $DB_PORT" start
  fi
  wait_for_port "$DB_HOST" "$DB_PORT"
  "$pg_bin/psql" -h "$DB_HOST" -p "$DB_PORT" -d postgres -v ON_ERROR_STOP=1 \
    -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${DB_USER}') THEN CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}'; END IF; END \$\$;"
  if ! "$pg_bin/psql" -h "$DB_HOST" -p "$DB_PORT" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" | grep -q 1; then
    "$pg_bin/createdb" -h "$DB_HOST" -p "$DB_PORT" -O "$DB_USER" "$DB_NAME"
  fi
}

if [ "${NEXIS_SKIP_DB_START:-0}" != "1" ]; then
  if command -v docker >/dev/null 2>&1; then
    docker compose -f deploy/docker-compose.yml up -d postgres
    wait_for_port "$DB_HOST" "$DB_PORT"
  elif pg_bin="$(find_pg_bin)"; then
    ensure_local_postgres "$pg_bin"
  else
    echo "Neither Docker nor PostgreSQL binaries were found." >&2
    echo "Install Docker/PostgreSQL, or set NEXIS_SKIP_DB_START=1 and NEXIS_DATABASE_DSN to an existing PostgreSQL DSN." >&2
    exit 1
  fi
fi

exec go run ./apps/console -config "$CONFIG"
