# Nexis Console API

Base URL:

```text
http://127.0.0.1:47141/api/v1
```

OpenAPI draft:

```text
packages/proto/openapi/openapi.yaml
```

## Nodes

```http
GET /nodes
GET /nodes/{nodeId}
```

## Resource Series

```http
GET /nodes/{nodeId}/resource-series?range=1h&metric=cpu
```

Ranges:

- `1h`
- `6h`
- `24h`
- `7d`

Metrics:

- `cpu`
- `memory`
- `network`
- `disk`

## Connectivity

```http
GET /nodes/{nodeId}/connectivity?range=24h
```

Returns the latest forward direct TCP and reverse logical TCP result per Probe.

## Probe Pair Series

```http
GET /nodes/{nodeId}/probes/{probeId}/series?range=24h&metric=app_rtt&direction=forward
```

Directions:

- `forward`
- `reverse`

Metrics:

- `connect_latency`
- `app_rtt`
- `jitter`
- `success_rate`
- `timeout_rate`
- `probe_loss_rate`

## Probes

```http
GET /probes
```

## Metrics / Debug

Metrics are exposed on a separate high port:

```http
GET http://127.0.0.1:47191/metrics
GET http://127.0.0.1:47191/debug/state
```
