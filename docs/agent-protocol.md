# Nexis Agent Protocol

Control channel:

```text
ws://console-host:47151/ws
```

All messages are JSON WebSocket text frames.

## Register

```json
{
  "type": "register",
  "component": "nexis-probe",
  "agent_id": "probe-gd-ct-v4",
  "name": "广东电信IPv4",
  "token": "replace-with-token",
  "version": "0.1.0",
  "capabilities": ["tcp-forward-probe", "reverse-logical-responder"],
  "meta": {
    "region": "广东",
    "isp": "电信",
    "network_stack": "IPv4"
  }
}
```

## Register Ack

```json
{
  "type": "register_ack",
  "session_id": "session_...",
  "heartbeat_interval_sec": 15,
  "lease_ttl_sec": 90,
  "server_time": 1711111111111,
  "reverse_tunnel": {
    "enabled": true,
    "listen_port": 47300
  }
}
```

## Heartbeat

```json
{
  "type": "heartbeat",
  "session_id": "session_...",
  "ts": 1711111111111,
  "stats": {
    "cpu_usage_percent": 12.3,
    "cpu_cores": 4
  }
}
```

## Run Test

```json
{
  "type": "run_test",
  "job_id": "job_...",
  "test_type": "forward_direct_tcp",
  "node_id": "node-oracle",
  "probe_id": "probe-gd-ct-v4",
  "target": {
    "host": "127.0.0.1",
    "port": 47211
  },
  "params": {
    "probe_count": 5,
    "connect_timeout_ms": 3000,
    "read_timeout_ms": 3000,
    "write_timeout_ms": 3000,
    "interval_ms": 200
  }
}
```

## Reverse Logical Stream

Console sends stream frames to a Probe over the control channel:

```json
{ "type": "reverse_stream_open", "stream_id": "stream_..." }
{ "type": "reverse_stream_data", "stream_id": "stream_...", "data": "base64..." }
{ "type": "reverse_stream_close", "stream_id": "stream_..." }
```

The Probe returns `reverse_stream_data` with base64-encoded TCP probe acknowledgements.

