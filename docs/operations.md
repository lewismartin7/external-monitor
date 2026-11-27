# Operations

## Installation

Use Docker Engine with Compose v2 on an independent host that can reach your targets and SMTP provider. The Compose service is **monitor** and the container is **external-monitor**. It uses a read-only root filesystem, UID:GID 99:100, a writable data mount, and a 64 MiB memory limit.

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

Edit `.env` with the SMTP credentials, sender, recipient, timezone, and the Docker host's LAN IP. Keep the completed file private.

Start the monitor:

```sh
docker compose pull
docker compose up -d
```

Open the dashboard at `http://DOCKER-HOST-LAN-IP:8099`. The dashboard and status API have no authentication. Keep this port LAN-only: do not forward it through the router or publish it through a reverse proxy. Compose defaults to binding all host interfaces; select a trusted LAN address and enforce firewall restrictions.

## How to use the app

Open the LAN dashboard to see each target’s status, last check, latency, and details. For a machine-readable view use **/api/status**. Use [Alert state](architecture.md#alert-state) to interpret pending, down, and recovery transitions.

To add or update a target, edit the private **config/config.json**, validate the JSON, and run **docker compose up -d --force-recreate monitor**. Recreation reloads config even when an editor replaced the bind-mounted file. Choose target names deliberately because [saved state is keyed by name](architecture.md#persistence-and-identity).

## Configuration

JSON is read at startup from CONFIG_FILE. The example chooses timeout 12; the code default is 10.

| JSON field | Default and rules |
| --- | --- |
| site_name / email_display_name | External Services / External Monitor |
| interval_seconds | 60; minimum 15 |
| timeout_seconds | 10; positive and less than interval |
| failures_before_alert / successes_before_recovery | 3 / 2 when zero; negative values are not rejected |
| certificate_warning_days | 14 when zero; negative values are not rejected |
| targets | Nonempty array; unique nonempty names, nonempty valid HTTP(S) URLs with host |
| targets[].expected_status | [200] if omitted/empty |
| targets[].contains | Optional literal case-sensitive body substring |

Unknown JSON fields are ignored. Target URLs have no host allowlist and must be administrator-controlled. Avoid credentials in URLs because addresses appear in status, logs and email.

| Environment | Default / purpose |
| --- | --- |
| CONFIG_FILE | /config/config.json; binary runtime path |
| STATE_FILE | /data/state.json; binary runtime path |
| LISTEN_ADDR | :8080; binary listener |
| TZ | America/Toronto in image/Compose; set desired operator timezone explicitly |
| SMTP_HOST | Empty; SMTP server |
| SMTP_PORT | 587; .env.example chooses 2525 |
| SMTP_TLS | starttls; use starttls or implicit |
| SMTP_USERNAME / SMTP_PASSWORD | Empty; credentials; authentication attempted when username set |
| SMTP_FROM | Empty; envelope sender |
| SMTP_TO | Empty; comma-separated recipients, whitespace trimmed |
| SEND_TEST_EMAIL_ON_START | false unless value equals true case-insensitively |
| DASHBOARD_BIND_IP / DASHBOARD_PORT | Compose interpolation only; defaults 0.0.0.0 / 8099 |

Compose passes SMTP/TZ/test-mail variables; it does not pass host CONFIG_FILE, STATE_FILE or LISTEN_ADDR unless Compose is deliberately extended. Keep completed .env private. Changing it requires docker compose up -d to recreate with new environment; docker compose restart monitor alone does not apply environment changes.

## Deployment verification and monitoring

From the deployment checkout:

```sh
docker compose ps
docker compose logs --tail=100 monitor
curl --fail http://DOCKER-HOST-LAN-IP:8099/healthz
curl --fail http://DOCKER-HOST-LAN-IP:8099/api/status
```

Use the configured LAN bind address instead of loopback if applicable. Status output and logs can contain private endpoint information; redact before sharing. Container health checks run every 30s, timeout 5s, retries 3, start period 10s, but check only the local HTTP endpoint.

Confirm that the configured targets appear in the API and receive a recent check. Monitor service health and logs for repeated state-write errors or SMTP failures. Use the [email delivery check](testing.md#email-delivery-check) for an intentional delivery test.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Dashboard unreachable | Compare the configured LAN bind address and port with the Compose port mapping and firewall. |
| Status stays pending or down | Check the configured status and body match, target reachability from the remote host, and the failure/recovery thresholds. |
| Alerts missing | Check SMTP connectivity and credentials, sender authorization, and logs for retry failures. |
| Settings appear stale | Recreate the container to pick up the current config bind mount and environment. |
| State fails to save | Confirm the data directory is writable by UID:GID 99:100 and check free space. |

## Upgrades

Before updates, back up private config, environment, state and the current image identifier. On the intended host, apply the update:

```sh
cd /mnt/user/appdata/external-monitor
git pull --ff-only
docker compose pull
docker compose up -d
```

Adjust the checkout path to match the host. Git pull updates source, Compose pull fetches images, and up applies them. Neither replaces the private monitored site list. For a config edit that may replace the bind-mounted file inode (for example sed -i), use docker compose up -d --force-recreate monitor after validating JSON. A simple restart reloads config only if the mount still references the updated file. Do not use line-number-based edits as a migration mechanism.

The workflow runs test/vet and Buildx on pushes to main, v* tags, PRs targeting main and manual dispatch. Non-PR runs log in with GITHUB_TOKEN and publish GHCR images; PRs build without publishing. Tags include conditional latest, Git tag and sha metadata. Build platform is linux/amd64 only. No remote deployment step exists. A documentation push can trigger image publication. Fork migration must review permissions, package visibility, workflow triggers and Compose image coordinates before pushing.

## Backup and restore

Back up .env, config/config.json, data/state.json and any existing state.json.tmp to a restricted location outside this repository. Record the source revision, Compose files and deployed image digest/ID. Do not use a backup directory inside the Docker build context: current .dockerignore does not exclude all private backups/data. Never build with extra private artifacts present in the context.

For a consistent cold backup, arrange a monitoring maintenance window, stop only service monitor, copy the private files/data directory with permissions into an external timestamped backup, then start monitor again. Stopping creates a monitoring gap. A missing state file on first installation is normal.

Restore with monitor stopped: retain an external copy of the current files, restore the matching config/.env/data and expected ownership (Compose 99:100; data writable), validate JSON privately, then recreate/start the service with the selected compatible image. Confirm process health, target state and mail delivery as authorized. Restored counters/pending alerts can cause retries or suppress an alert previously acknowledged; deleting state resets alert history. There is no built-in restore API or schema migration tool.

## Rollback

Before deployment record docker inspect external-monitor --format '{{.Image}}' and preserve a usable immutable image reference (or save the image externally), plus the source revision and private backup. latest is mutable and is not a rollback reference.

To roll back in an maintenance window, stop monitor, restore the previous source/Compose and private files if their formats changed, select the recorded compatible image via an external Compose override, and run docker compose -f compose.yaml -f /secure/path/rollback.yaml up -d --force-recreate monitor. The override should set services.monitor.image to the retained immutable reference; use --pull never if relying on a locally saved image. Validate health/status and ownership. There is no automatic downgrade/migration support; never assume old code can read a newly changed state schema.
