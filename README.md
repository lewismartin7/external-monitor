# External Monitor

External Monitor checks HTTP(S) endpoints from an independent host. It sends HTML and plain-text email for confirmed outages, recoveries, and expiring TLS certificates. A small LAN dashboard shows the current status, and saved state preserves alert counters across restarts.

## Prerequisites

- Docker Engine with Compose v2 on a host that can reach the monitored endpoints and your SMTP provider.
- A trusted LAN address for the dashboard and authorized sender and recipient addresses for email.
- Go 1.27.3 for development. The app uses the Go standard library.

## Quick start

Clone or download this repository on the independent host. For a new deployment, follow [Installation](docs/operations.md#installation) to prepare the private config, SMTP settings, data directory, and LAN binding. From the checkout root start the service:

```sh
docker compose pull
docker compose up -d
```

Open the dashboard on the configured LAN address and port (8099 by default), then run the [deployment checks](docs/operations.md#deployment-verification-and-monitoring).

## Documentation

- [Operations](docs/operations.md) · installation, usage, configuration, updates, and recovery
- [Architecture](docs/architecture.md) · components, data flows, and state
- [Testing](docs/testing.md) · isolated fixtures and release criteria
- [To-do](docs/todo.md) · active and future work

## License

MIT
