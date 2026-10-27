# External Monitor

A lightweight, outbound-only HTTP(S) availability monitor for Unraid and other Docker hosts. It checks public endpoints from a remote location and sends styled HTML email alerts for confirmed outages, recoveries, and expiring TLS certificates.

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

Clone or download this repository into the Docker host's application-data directory:

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

Open the dashboard at `http://DOCKER-HOST-LAN-IP:8099`. Keep this port LAN-only: do not forward it through the router or publish it through a reverse proxy.

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

## Local development

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
docker compose -f compose.yaml -f compose.build.yaml down
```

Run the Go checks directly or in a Go container:

```sh
go test ./...
go vet ./...
```

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
