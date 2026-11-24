# GitHub Copilot handoff

Prepared 2026-11-20. Scope: documentation and repository hygiene in External Monitor only. Facts derive from source, manifests, tests and existing project documentation. No live state or completed environment was read, no remote monitor inspected, and no service, infrastructure, source behavior, Git history or publication changed.

## Product requirements to preserve

- Independent external HTTP(S) checks from a separately located host.
- Per-target allowed statuses and optional literal response text; thresholds for outage/recovery.
- Persistent counters, pending notification retries and certificate expiry warning acknowledgements.
- Styled, escaped HTML email with plain-text fallback and configurable sender display/site name.
- A small read-only status dashboard kept on a trusted LAN, without introducing public access assumptions.
- Private target list, SMTP environment and state survive repository/image updates.

## Repository migration checklist

1. Use external-monitor as repository root, not the appdata share. A Git repository and container publishing workflow already exist; do not initialize another repository.
2. Work from a separate local checkout. Privately back up the remote operator's .env, config and state outside the repository before any operational migration. This source checkout is not a verified copy of Jesse's current monitored sites.
3. Review git status --short, git ls-files and ignore rules. No private .env, active config or state files were found in the tracked-file list during this review; only .env.example, config/config.example.json and data/.gitkeep are intentional templates/placeholders. Ignore rules do not remove files from history.
4. Previous conversation records removal of personal domains from current files while leaving historical commits intact. This task did not audit or rewrite history. Review full history privately before changing visibility; do not reintroduce actual domains into examples.
5. Confirm the intended repository owner, visibility, remote and GHCR permissions. Existing README/Compose/workflow coordinates reflect current checked-in configuration, not an instruction to create or publish anything.
6. Install/verify Go 1.27.3, run go test ./... and go vet ./... locally or in CI. Assess toolchain and base image availability before relying on the build; they were not queried from registries during handoff.
7. Review workflow triggers before any push: non-PR runs publish images. Coordinate changes to hardcoded Compose image coordinates if moving to another repository. Publication does not deploy Jesse's container.
8. Provide this handoff and a feature request to Copilot. Plan remote config/state migration separately with the operator. Keep source/image updates separate from private target-list edits and preserve name-based identity deliberately.
9. Before a later deployment, agree a maintenance window, immutable rollback image and private backup/restore verification. Remote access and operator action remain required; no remote update was performed here.

## Starting prompt for Copilot

> Read .github/copilot-instructions.md, README.md and docs/HANDOFF.md, docs/ARCHITECTURE.md and docs/OPERATIONS.md. Summarize polling, alert retries, name-keyed state and the private configuration/deployment boundaries. Run go test ./... and go vet ./... with disposable storage and report actual results. Do not contact production endpoints, send mail, touch private data, deploy, restart, commit or publish without a separate request. Then implement the next requested change with focused validation and documentation updates.

## Known limitations and follow-ups

These are source observations, not changes authorized by this handoff:

- No dashboard/API authentication, application TLS or enforced LAN firewall. Default Compose bind is all interfaces.
- Pending status can remain pending after a successful check; state transitions need dedicated tests before a fix.
- State identity is name-based; URL edits retain saved URL/counters, removed targets remain in state and renames lose continuity.
- Whole-map JSON writes, no schema version, cross-process locks, explicit fsync durability, corruption reporting or automatic migrations. Invalid JSON/read errors silently discard saved history at startup; valid unexpected structures can fail later.
- Config does not hot reload. Private target changes are not distributed by Git or image updates. Bind-mounted file replacement may require container recreation.
- SMTP has no complete session timeout; failed sends retry but crash windows can duplicate messages. Unsupported TLS mode strings do not enforce encryption.
- No per-target auth headers, custom methods, bounded history, alert routing or administration UI. Body matching is limited to 1 MiB and certificates belong to the final response after redirects.
- Six unit tests omit polling/state/SMTP/HTTP integration and concurrency coverage. No browser tests or live mail validation were performed.
- Only amd64 images are built. Mutable latest and unpinned runtime base tag reduce deployment reproducibility. Local container override shares production project/container naming.
- Current .dockerignore is narrower than .gitignore: private state/backups could enter the build context even though the multi-stage Dockerfile copies only Go source/module into build stages. Review before building; no Docker behavior change was made here.

## Verification performed

- Read workspace standard, applicable root AGENTS.md and Kits structural reference. No nested project AGENTS.md was found.
- Reviewed all tracked application source, six tests, JSON example, Docker/Compose manifests, Actions workflow and Dependabot configuration. Did not inspect completed .env, live state or active target contents.
- Checked documentation file links, required files, ignore rules against representative private/generated paths, tracked-file hygiene and final diff scope with read-only/static checks. See final handoff report for results.
- Go and Docker executables were not available in this session. go test, go vet, race checks, Docker/Compose execution and image builds were not run. Commands were compared with source/manifests; production health, SMTP, DNS and restore behavior remain unverified.

No commits, pushes, deployment commands, service restarts, application changes or live data modifications were performed. Remaining work: run toolchain checks, review publication/history/privacy decisions and coordinate any later remote operator migration.
