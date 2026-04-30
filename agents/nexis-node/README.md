# Nexis Node

Nexis Node represents a remote public node under test.

It:

- connects outbound to Nexis Console control WebSocket
- registers as `nexis-node`
- reports system stats in heartbeats
- listens for direct TCP probe traffic on `47211`
- runs reverse logical TCP tests against Console-assigned Probe reverse ports

## Run

```bash
./start-node.sh
```

## Important Settings

```text
NEXIS_NODE_ID
NEXIS_NODE_NAME
NEXIS_NODE_TOKEN
NEXIS_CONTROL_URL
NEXIS_NODE_PUBLIC_IPV4
NEXIS_NODE_LISTENER_PORT
```
