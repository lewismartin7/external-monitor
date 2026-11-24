# External Monitor

A lightweight HTTP(S) availability monitor for Unraid and other Docker hosts. It checks public endpoints from a remote location and sends styled HTML email alerts for confirmed outages, recoveries, and expiring TLS certificates.

## Documentation

- [Copilot handoff and migration](docs/HANDOFF.md)
- [Architecture, persistence and APIs](docs/ARCHITECTURE.md)
- [Operations, configuration and recovery](docs/OPERATIONS.md)
- [Copilot repository instructions](.github/copilot-instructions.md)

Prerequisites: Go 1.27.3 for direct development, or Docker Engine with Compose v2 for containers. Use a separate checkout and disposable config/data for local work. The service uses only the Go standard library.

## Features

- Independent external HTTP(S) checks
- Configurable expected status codes and response text
- Failure and recovery thresholds to suppress brief interruptions
- HTML email with a plain-text fallback
- Custom sender display name and subject prefix
- Red DOWN and green UP status indicators
- Certificate-expiry warnings
- Persistent alert state across restarts
- Small LAN status dashboard
- Non-root, read-only container with all Linux capabilities removed

## Quick start

For a first installation, clone or download this repository into the intended Docker host's application-data directory. Do not overwrite existing private config or environment files:

```sh
cd /mnt/user/appdata
git clone https://github.com/lewismartin7/external-monitor.git
cd external-monitor
mkdir -p config data
cp config/config.example.json config/config.json
cp .env.example .env
chown -R 99:100 data
chmod 770 data
```

Edit `config/config.json` to define the identity, timing, and targets:

```json
{
  "site_name": "My External Services",
  "email_display_name": "My Monitor",
  "interval_seconds": 60,
  "timeout_seconds": 12,
  "failures_before_alert": 3,
  "successes_before_recovery": 2,
  "certificate_warning_days": 14,
  "targets": [
    {
      "name": "Photos",
      "url": "https://photos.example.com/api/server/ping",
      "expected_status": [200]
    }
  ]
}
```

Edit `.env` with the SMTP credentials, sender, recipient, timezone, and the Docker host's LAN IP. Never commit or share the completed file.

Start the monitor:

```sh
docker compose pull
docker compose up -d
```

Open the dashboard at `http://DOCKER-HOST-LAN-IP:8099`. The dashboard and status API have no authentication. Keep this port LAN-only: do not forward it through the router or publish it through a reverse proxy. Compose defaults to binding all host interfaces; select a trusted LAN address and enforce firewall restrictions.

## Email testing

Set this in `.env`:

```dotenv
SEND_TEST_EMAIL_ON_START=true
```

Apply it with `docker compose up -d`. After the test arrives, restore the setting to `false` and run the command again.

SMTP modes:

- `starttls`: commonly ports 587 or 2525
- `implicit`: commonly port 465

## Updating

```sh
cd /mnt/user/appdata/external-monitor
git pull --ff-only
docker compose pull
docker compose up -d
```

The private `.env`, `config/config.json`, and `data/state.json` files remain outside the image and are not replaced.

### Updating monitored targets

Edit the `targets` array in your private `config/config.json`, then run:

```sh
docker compose up -d --force-recreate monitor
```

Pulling the repository alone does not update the active target list. Recreation also refreshes the bind mount after an editor or sed replaces the config file. Changes to .env require recreation rather than restart.

## Local development

Use the isolated loopback fixture instructions in [Operations](docs/OPERATIONS.md#local-checks-and-development) for direct local runs. The container override below must run in a disposable checkout on a separate Docker daemon from production because it retains the fixed project/container name:

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
docker compose -f compose.yaml -f compose.build.yaml down
```

Run the Go checks directly or in a Go container:

```sh
go test ./...
go vet ./...
```

The six unit tests use temporary files and do not contact SMTP or monitored sites. The existing GitHub Actions workflow runs these checks and builds amd64 images; non-PR events can publish to GHCR. It does not deploy the remote monitor.

## Production overview

Compose runs service monitor as container external-monitor using the configured GHCR image, UID:GID 99:100, ./config/config.json read-only and ./data writable. Historically the independent deployment runs on Jesse's remote Unraid host; its current runtime was not verified for this handoff. See [Operations](docs/OPERATIONS.md) for deployment, backup/restore and rollback. Keep private target lists, environment and alert state out of Git.

## Security model

The container:

- Runs as Unraid's `99:100` user/group
- Uses a read-only root filesystem
- Drops all Linux capabilities
- Enables `no-new-privileges`
- Has no Docker socket or host networking
- Uses a 64 MB memory limit
- Requires only outbound network access and an optional LAN dashboard port

## License

MIT
