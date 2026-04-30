#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GOCACHE="${GOCACHE:-/private/tmp/nexis-go-cache}"
CONFIG="${NEXIS_CONFIG:-deploy/config/nexis-node.yaml}"

exec go run ./agents/nexis-node -config "$CONFIG"

