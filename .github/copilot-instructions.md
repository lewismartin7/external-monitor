# External Monitor repository instructions

Read [README](../README.md), [architecture](../docs/architecture.md), [operations](../docs/operations.md), [testing](../docs/testing.md), and [to-do](../docs/todo.md) before changes.

- Keep the small Go standard-library implementation. The module and Docker build currently require Go 1.27.3. No third-party Go modules or frontend build exist.
- Preserve independent HTTP(S) checks, configured status/text matching, failure/recovery thresholds, pending email retries, certificate warning acknowledgement and multipart escaped HTML/plain-text mail.
- State is keyed by target name. Preserve alert counters and acknowledgement fields; plan backup, migration and rollback before changing persisted structures or identity rules.
- Use a separate local checkout and disposable storage. Never read into examples, commit or overwrite private .env, config/config.json, data files, target lists, email previews or backups. Keep public examples generic.
- Run go test ./... and go vet ./... for behavior changes. Existing tests use t.TempDir and do not contact targets or SMTP. Use go test -race ./... when supported for concurrency changes. Do not launch the app with production configuration for tests.
- The dashboard and status API have no authentication. Keep them LAN-only. Compose defaults to binding all host interfaces; do not describe that as enforced LAN isolation.
- One process owns each state file. Mutex protection is in-process only. Do not add replicas against shared storage.
- Config loads only at startup. Git/image updates do not update the private monitored target list. Env changes require container recreation; restart alone does not reload Compose environment.
- Production operations are separately authorized work. Do not deploy, restart, send test mail, edit infrastructure or use host ownership commands during routine development.
- Existing GitHub Actions tests/vets and publishes amd64 images on non-PR events. A push can publish an image; it does not update a remote container. Do not push as part of a documentation task.
- Update related docs with contract changes. Report checks actually run and distinguish source review from live verification.
