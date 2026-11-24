# Operations

## Boundaries and prerequisites

Use a separate local checkout for development. Go 1.27.3 is declared by go.mod and the Docker builder; verify toolchain availability before migration. Docker Engine and Compose v2 are needed for containers. Host administration examples below are POSIX/Unraid commands, not PowerShell commands. Do not execute production commands as documentation validation.

The checked-in Compose project is external-monitor, service monitor, container external-monitor. It runs ghcr.io/lewismartin7/external-monitor:latest with restart unless-stopped, UID:GID 99:100, a read-only root filesystem, a 16 MiB /tmp tmpfs, dropped capabilities, no-new-privileges, pids limit 64 and memory limit 64m. The image alone uses its named monitor user; Compose overrides this.

Compose mounts ./config/config.json read-only and ./data read/write. It publishes host port 8099 to 8080 by default, on 0.0.0.0. There is no explicit custom network, proxy, TLS ingress or Docker socket. The dashboard must be restricted by host binding/firewall; the application does not enforce LAN-only access. Outbound DNS, target HTTP(S) and SMTP access are required.

Historical operator context says the independent monitor runs on Jesse's remote Unraid server. That server's paths, active image, configuration, ownership, firewall and health were not inspected. This source share is not proof of remote deployment state. Do not put actual monitored domains in public documentation.

## Local checks and development

From the checkout root:

```sh
go test ./...
go vet ./...
```

Tests use temporary files and no network or production state. Optional concurrency validation on supported toolchains: go test -race ./....

For a manual local run, use only a disposable working directory and a loopback fixture. Start a fixture serving a static directory with python3 -m http.server 18081 --bind 127.0.0.1 in another terminal. Then, from the source checkout on a POSIX host:

```sh
go build -o /tmp/external-monitor-dev .
DEV_DIR=$(mktemp -d)
cat > "$DEV_DIR/config.json" <<'JSON'
{"targets":[{"name":"Local fixture","url":"http://127.0.0.1:18081/","expected_status":[200]}]}
JSON
CONFIG_FILE="$DEV_DIR/config.json" STATE_FILE="$DEV_DIR/state.json" LISTEN_ADDR=127.0.0.1:18080 SMTP_HOST= SMTP_FROM= SMTP_TO= SEND_TEST_EMAIL_ON_START=false /tmp/external-monitor-dev
```

Reserve the binary path and ports for this development session. Stop both processes when finished and remove only the known disposable directory/binary. The binary does not load .env automatically. SMTP is disabled in this example; failures may log missing-SMTP errors but cannot send mail.

The existing container override can be used in a separate disposable checkout:

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
docker compose -f compose.yaml -f compose.build.yaml down
```

It retains the fixed container/project name, private bind paths and port mapping. Do not run it alongside production on the same Docker daemon or against production config/data. Set up disposable config and data first; example URLs are placeholders, not local test fixtures.

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

## Installation and deployment

Follow the [README quick start](../README.md#quick-start) in a new checkout on the intended remote host. The mkdir/cp commands are first-install only; never overwrite existing config or .env during updates. chown/chmod require appropriate host privileges and apply only to this app's data directory. There are no bundled proxy, DNS or Unraid administration utilities.

Before updates, back up private config, environment, state and the current image identifier. Then, when deployment is explicitly requested:

```sh
cd /mnt/user/appdata/external-monitor
git pull --ff-only
docker compose pull
docker compose up -d
```

This path is the documented convention, not verified on Jesse's host. Git pull updates source, Compose pull fetches images, and up applies them. Neither replaces the private monitored site list. For a config edit that may replace the bind-mounted file inode (for example sed -i), use docker compose up -d --force-recreate monitor after validating JSON. A simple restart reloads config only if the mount still references the updated file. Do not use line-number-based edits as a migration mechanism.

The workflow runs test/vet and Buildx on pushes to main, v* tags, PRs targeting main and manual dispatch. Non-PR runs log in with GITHUB_TOKEN and publish GHCR images; PRs build without publishing. Tags include conditional latest, Git tag and sha metadata. Build platform is linux/amd64 only. No remote deployment step exists. A documentation push can trigger image publication. Fork migration must review permissions, package visibility, workflow triggers and Compose image coordinates before pushing.

## Diagnostics and email test

On the authorized host:

```sh
docker compose ps
docker compose logs --tail=100 monitor
curl --fail http://127.0.0.1:8099/healthz
curl --fail http://127.0.0.1:8099/api/status
```

Use the configured LAN bind address instead of loopback if applicable. Status output and logs can contain private endpoint information; redact before sharing. Container health checks run every 30s, timeout 5s, retries 3, start period 10s, but check only the local HTTP endpoint.

To test delivery deliberately, set SEND_TEST_EMAIL_ON_START=true in private .env, run docker compose up -d, confirm delivery, then restore false and run up again. This sends real mail. Check SMTP connectivity/authentication, sender authorization and logs if mail fails. Monitor retries pending alerts on later checks; no bounded SMTP session timeout covers all operations.

## Backup and restore

Back up .env, config/config.json, data/state.json and any existing state.json.tmp to a restricted location outside this repository. Record the source revision, Compose files and deployed image digest/ID. Do not use a backup directory inside the Docker build context: current .dockerignore does not exclude all private backups/data. Never build with extra private artifacts present in the context.

For a consistent cold backup, arrange a monitoring maintenance window, stop only service monitor, copy the private files/data directory with permissions into an external timestamped backup, then start monitor again. Stopping creates a monitoring gap. Do not perform this as a handoff check. A missing state file on first installation is normal.

Restore with monitor stopped: retain an external copy of the current files, restore the matching config/.env/data and expected ownership (Compose 99:100; data writable), validate JSON privately, then recreate/start the service with the selected compatible image. Confirm process health, target state and mail delivery as authorized. Restored counters/pending alerts can cause retries or suppress an alert previously acknowledged; deleting state resets alert history. There is no built-in restore API or schema migration tool.

## Rollback

Before deployment record docker inspect external-monitor --format '{{.Image}}' and preserve a usable immutable image reference (or save the image externally), plus the source revision and private backup. latest is mutable and is not a rollback reference.

To roll back in an approved maintenance window, stop monitor, restore the previous source/Compose and private files if their formats changed, select the recorded compatible image via an external Compose override, and run docker compose -f compose.yaml -f /secure/path/rollback.yaml up -d --force-recreate monitor. The override should set services.monitor.image to the retained immutable reference; use --pull never if relying on a locally saved image. Validate health/status and ownership. There is no automatic downgrade/migration support; never assume old code can read a newly changed state schema.
