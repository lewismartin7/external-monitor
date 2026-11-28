# Testing

## Source checks

Use Go 1.27.3 and run the following from the checkout root. Use the race detector when changing concurrent code and the toolchain supports it.

```sh
go test ./...
go vet ./...
go test -race ./...
```

The six unit tests use temporary files and in-memory email rendering. They cover config defaults, invalid URL schemes, allowed status lookup, multipart email, escaped HTML and sender formatting. Polling, HTTP handlers, SMTP retries, certificate expiry and persistence currently require manual checks.

## Test setup and data

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

For a container smoke test, use a disposable checkout on a separate Docker daemon:

```sh
docker compose -f compose.yaml -f compose.build.yaml up -d --build
docker compose -f compose.yaml -f compose.build.yaml down
```

It retains the fixed container/project name, private bind paths and port mapping. Do not run it alongside production on the same Docker daemon or against production config/data. Set up disposable config and data first; example URLs are placeholders, not local test fixtures.

## End-to-end workflow

1. Check **/healthz**, **/api/status**, and the dashboard on port 18080. Verify the loopback target reaches **up** and has a recent check.
2. Stop the fixture and wait for the configured number of failures. Confirm the target reaches **down**. Start the fixture again and confirm recovery after the configured number of successes.
3. Set a literal body match and change the fixture content to confirm both matching and failing responses. Use a local TLS fixture with a trusted test CA for certificate warning scenarios.

## Email delivery check

Use a disposable SMTP sink and test recipients to check DOWN, RECOVERED and certificate warning messages. Verify the plain-text and HTML parts, and test that pending notifications retry after a temporary SMTP failure.

For an intentional deployment delivery test, set **SEND_TEST_EMAIL_ON_START=true** in the private **.env**, run **docker compose up -d**, and confirm the message arrives. Restore the setting to **false** and run **docker compose up -d** again.

## Recovery checks

Restart against the same disposable state and confirm status and counters persist. Restore a saved copy of the fixture config and state using the [restore workflow](Project%20Docs/external-monitor/docs/operations.md#backup-and-restore), then confirm polling resumes and pending alerts behave as expected.

## Release criteria

- Go tests and vet pass. Run the isolated workflow for any changed polling, email, or persistence behavior.
- The linux/amd64 image builds and its health check passes with disposable config and data.
- If config or state compatibility changes, validate restore and rollback and document migration steps before release.
- Before publishing or moving repositories, confirm the intended repository visibility, GHCR permissions, and image coordinates. Review history privately for previously committed deployment details. [Operations](Project%20Docs/external-monitor/docs/operations.md#upgrades) explains workflow triggers.
