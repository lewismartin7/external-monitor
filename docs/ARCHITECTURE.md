# Architecture

## Source and dependencies

| Path | Responsibility |
| --- | --- |
| main.go | Configuration, polling, state transitions, SMTP transport, persistence and HTTP dashboard/API |
| email.go | Multipart MIME construction, display-name formatting, escaped HTML alert presentation |
| main_test.go | Six unit tests using temporary config files and in-memory email rendering |
| go.mod | Module external-monitor; Go 1.27.3, standard library only |
| config/config.example.json | Generic configuration example; not the active target list |
| Dockerfile | Static Linux binary built with Go 1.27.3 Alpine; Alpine 3.22 runtime with CA roots and timezone data |
| compose.yaml / compose.build.yaml | Published image deployment / local build override |
| .github/workflows/container.yml | Test, vet, amd64 image build and conditional GHCR publishing |
| .github/dependabot.yml | Weekly Actions and Docker dependency update checks |

## Execution and concurrency

Startup reads config once, loads environment and saved state, optionally sends test mail, starts one goroutine per target, then serves HTTP. Each goroutine checks immediately and then on an interval ticker. The shared HTTP client has a total request timeout, TLS minimum 1.2 and rejects redirects once the redirect chain reaches five prior requests. Checks use GET with User-Agent ExternalMonitor/1.0. Success requires an allowed final HTTP status and, if set, a literal case-sensitive substring within the first 1 MiB of body. Oversized bodies are truncated without a separate size error. Certificate expiry comes from the final response's leaf certificate.

Each target serializes its own checks and notifications. Different targets run concurrently. A shared RWMutex protects state and serialization; SMTP runs outside that lock. Slow SMTP can delay the target's next check. There is no cross-process locking or graceful shutdown handler.

## Alert state

New targets start unknown. First success changes unknown to up. First failure changes unknown to pending; the configured consecutive failure threshold changes status to down and queues DOWN. Recovery from down requires the success threshold and queues RECOVERED only if a DOWN mail was acknowledged. Failed mail remains pending and is retried on later checks. Certificate warnings are keyed by expiry date (YYYY-MM-DD) and acknowledged only after mail succeeds.

Known edge case: pending does not explicitly become up on success before reaching down. Do not silently change this in a documentation task. Pending DOWN mail may also still be retried while successful checks have not yet reached the recovery threshold.

## Persistence and identity

STATE_FILE defaults to /data/state.json: a JSON object keyed by target name, values containing:

- name, url, status; consecutive_failures and consecutive_successes;
- last_checked, last_changed, latency_ms, http_status, detail;
- certificate_expires, certificate_warning_for;
- state_alert_pending, certificate_alert_pending, down_alert_sent.

Time values use Go time.Time JSON encoding. Every check and mail acknowledgement marshals the entire map, writes a sibling .tmp file (0600), then renames it. The parent is created with 0700 when missing. This is replacement-based persistence, not a transactional database or fsync durability guarantee. Save failures are logged and do not stop monitoring. Read failures and invalid JSON are silently ignored at startup. Structurally unexpected but valid JSON is not comprehensively validated.

Renaming a target starts a new identity; removed names remain in saved state but are omitted from the API. Changing a URL under the same name preserves counters/alerts and the saved state's old URL because existing entries are not refreshed from config. There is no version field, cleanup policy or migration runner. Only one process may write a state file.

## HTTP contracts

| Route | Response |
| --- | --- |
| /healthz | HTTP 200, text/plain, ok followed by newline; process HTTP health only |
| /api/status | JSON object: site_name, targets array in config order, generated_at timestamp |
| / | Server-rendered escaped HTML cards with target status, URL, detail, latency and last check |
| Other paths | HTTP 404 through dashboard handler |

Handlers do not restrict HTTP methods or authenticate clients. There are no mutation/configuration APIs, live dashboard refresh, CORS policy or application TLS listener. /api/status exposes target addresses and diagnostic details. Health does not establish target health, SMTP delivery or state durability. The -healthcheck CLI always probes 127.0.0.1:8080, even if LISTEN_ADDR is overridden.

## Mail and test coverage

SMTP supports implicit TLS or STARTTLS with minimum TLS 1.2. Other mode strings take the unupgraded connection path; only the documented secure modes should be configured. Authentication uses smtp.PlainAuth when username is nonempty. Headers reject CR/LF in subject, sender and recipients; MIME uses quoted-printable text/plain and text/html alternatives. Presentation values are escaped.

Tests cover config defaults, rejecting a file URL, integer membership, multipart email, HTML escaping/down styling and sender display formatting. No current tests cover polling transitions, HTTP handlers, persistence failure/restart, SMTP retries, TLS expiry, redirects or concurrent access. No browser/integration suite exists. See [handoff](HANDOFF.md) for verification actually performed.
