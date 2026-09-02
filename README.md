# rookery-node

Relay node for [Rookery](https://github.com/proxt/rookery) — a WebRTC-tunnel
VPN. This repo is just the relay server (`rookeryd`): it terminates client
WebRTC sessions and relays TCP/UDP traffic. It holds no state of its own —
users, subscriptions, and traffic stats all live on the **panel**, which is
part of the main [proxt/rookery](https://github.com/proxt/rookery) repo along
with the Windows client.

A node is useless on its own: it needs to be registered with a running panel
first (panel admin UI → Nodes → Add), which is where you get the
`node_id`/`api_key` this node's config needs.

## Build

```
make build          # bin/rookeryd — static linux/amd64 binary, CGO_ENABLED=0
make docker-build    # local Docker image (for testing, not publishing)
make test
make lint
```

Every push to `master` publishes `ghcr.io/proxt/rookery-node` via
`.github/workflows/docker.yml`.

## Deploy (Docker, recommended)

1. Register this node in the panel's admin UI first — copy the `node_id` and
   `api_key` it gives you.
2. Install Docker if you don't have it: `curl -fsSL https://get.docker.com | sudo sh`
3. On the VDS:
   ```
   mkdir -p ~/rookery-node && cd ~/rookery-node
   curl -O https://raw.githubusercontent.com/proxt/rookery-node/master/deploy/docker-compose.yml
   cp configs/node.example.yaml node.yaml   # or just create node.yaml directly
   ```
   Fill in `panel_addr`, `node_id`, `api_key` in `node.yaml` (see
   `configs/node.example.yaml` for every field).
4. `sudo docker compose up -d`

`network_mode: host` in the compose file is required, not optional — pion's
ICE agent needs to see the VDS's real public interface, not Docker's internal
bridge address.

Put Caddy in front for TLS — see `deploy/Caddyfile.example`.

### Ports

- **TCP 443** (or 80 for ACME) on Caddy.
- **UDP `ice_udp_port`** (default 51000) — the only UDP port needed; it's
  fixed via `SetICEUDPMux`, no ephemeral range.
- `listen_addr` (default `127.0.0.1:8080`) stays internal, behind Caddy.

## Deploy without Docker

See `deploy/systemd/rookery-node.service` and `deploy/Caddyfile.example`.
Build with `make build`, copy `bin/rookeryd` to `/opt/rookery/rookeryd`,
point `ExecStart`'s `-config` flag at your `node.yaml`.

## Why a separate repo

This node has no dependency on the panel or client beyond the wire protocol
(session tokens signed by the panel, verified here with the node's own
`api_key` — see `internal/signaling`). Keeping it in its own repo means you
can run/update relay nodes independently of the panel and client release
cycle, and a node's Docker image doesn't need anything from the rest of the
project checked out to build.
