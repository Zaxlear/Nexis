# Nexis

Nexis is a TCP bidirectional network quality monitoring MVP.

It contains:

- `apps/console`: Nexis Console REST API, WebSocket control hub, scheduler, reverse logical TCP ports, PostgreSQL storage, and static Web hosting.
- `apps/web`: Nexis Console web UI styled after the provided light card dashboard reference.
- `agents/nexis-node`: Nexis Node agent with a TCP probe listener and reverse logical TCP runner.
- `agents/nexis-probe`: Nexis Probe agent with forward direct TCP probing and reverse logical stream responses.
- `packages/proto`: Shared JSON protocol structs.
- `packages/tcpprobe`: Shared TCP JSON Lines probe runner and echo responder.
- `packages/wslite`: Small no-dependency WebSocket client/server helper.
- `deploy`: PostgreSQL migration, Docker Compose, and systemd examples.

## Quick Start

Use a writable Go build cache on macOS sandboxed environments:

```bash
export GOCACHE=/private/tmp/nexis-go-cache
```

Start Nexis Console:

```bash
./start-console.sh
```

This script starts PostgreSQL automatically with Docker when available, otherwise it falls back to Homebrew PostgreSQL binaries if present. If you already have PostgreSQL, set `NEXIS_SKIP_DB_START=1` and `NEXIS_DATABASE_DSN`.

Open:

```text
http://127.0.0.1:47131
```

In separate terminals, start a local Node and Probe:

```bash
./start-node.sh
./start-probe.sh
```

The Console schedules:

- forward direct TCP: `Nexis Probe -> Nexis Node:47211`
- reverse logical TCP: `Nexis Node -> Nexis Console reverse port -> Nexis Probe`

## Default Ports

| Component | Port |
|---|---:|
| Nexis Console Web | 47131 |
| Nexis Console API | 47141 |
| Nexis Console Control | 47151 |
| Nexis Console Metrics / Debug | 47191 |
| Nexis Node TCP Test Listener | 47211 |
| Nexis Probe Reverse Tunnel Pool | 47300-47999 |

## Useful Commands

```bash
GOCACHE=/private/tmp/nexis-go-cache go test ./...
GOCACHE=/private/tmp/nexis-go-cache go build ./apps/console
GOCACHE=/private/tmp/nexis-go-cache go build ./agents/nexis-node
GOCACHE=/private/tmp/nexis-go-cache go build ./agents/nexis-probe
```

## Configuration

The default local Console script starts PostgreSQL via Docker Compose and stores agents, sessions, resource snapshots, TCP results, and reverse port assignments in PostgreSQL.

Console examples:

```bash
NEXIS_WEB_ADDR=:47131 \
NEXIS_API_ADDR=:47141 \
NEXIS_CONTROL_ADDR=:47151 \
NEXIS_PUBLIC_HOST=127.0.0.1 \
go run ./apps/console
```

Node examples:

```bash
NEXIS_NODE_ID=node-oracle \
NEXIS_NODE_NAME=Oracle \
NEXIS_NODE_PUBLIC_IPV4=127.0.0.1 \
NEXIS_CONTROL_URL=ws://127.0.0.1:47151/ws \
go run ./agents/nexis-node
```

Probe examples:

```bash
NEXIS_PROBE_ID=probe-gd-ct-v4 \
NEXIS_PROBE_NAME=广东电信IPv4 \
NEXIS_CONTROL_URL=ws://127.0.0.1:47151/ws \
go run ./agents/nexis-probe
```

The Console seeds demo data by default for the UI. Disable it with:

```bash
go run ./apps/console -seed-demo=false
```

Metrics/debug endpoints:

```text
http://127.0.0.1:47191/metrics
http://127.0.0.1:47191/debug/state
```

YAML examples live in:

```text
deploy/config/console.yaml
deploy/config/nexis-node.yaml
deploy/config/nexis-probe.yaml
```

Docker assets live in `deploy/`:

```bash
docker compose -f deploy/docker-compose.yml up --build
```
