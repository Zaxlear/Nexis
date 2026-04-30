# Nexis Console

Nexis Console is the central control plane.

It starts:

- Web UI on `47131`
- REST API on `47141`
- Agent control WebSocket on `47151`
- Metrics/debug endpoint on `47191`
- Probe reverse logical ports from `47300` to `47999`

## Run

```bash
./start-console.sh
```

## API

```text
GET http://127.0.0.1:47141/api/v1/nodes
GET http://127.0.0.1:47141/api/v1/probes
GET http://127.0.0.1:47191/metrics
GET http://127.0.0.1:47191/debug/state
```

## Agent Tokens

For local development, the Console accepts first registration and stores that token hash in the configured store.

For stricter local/prod behavior, configure token allowlists:

```bash
NEXIS_AGENT_TOKENS='node-local-oracle=nexis-local-token,probe-local-guangdong-ct-v4=nexis-local-token' \
go run ./apps/console
```

## Storage

By default `deploy/config/console.yaml` enables PostgreSQL:

```text
postgres://nexis:nexis@127.0.0.1:55432/nexis?sslmode=disable
```

The Console persists agents, sessions, node resource snapshots, TCP test results, and reverse tunnel port assignments.
