# Nexis Probe

Nexis Probe represents a local network probe that may not have public inbound IPv4.

It:

- connects outbound to Nexis Console control WebSocket
- registers as `nexis-probe`
- executes forward direct TCP tests to Nexis Node listener ports
- responds to reverse logical TCP streams over the existing control channel
- reconnects with exponential backoff

## Run

```bash
./start-probe.sh
```

## Important Settings

```text
NEXIS_PROBE_ID
NEXIS_PROBE_NAME
NEXIS_PROBE_TOKEN
NEXIS_CONTROL_URL
NEXIS_PROBE_REGION
NEXIS_PROBE_ISP
NEXIS_PROBE_STACK
NEXIS_PROBE_CONCURRENCY
```
