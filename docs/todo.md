# To-do

## Active

- [ ] **TODO-001** Add regression tests for pending, down, and recovered transitions, then address the pending-to-up behavior.

- [ ] **TODO-002** Add test coverage for HTTP handlers, persistence across restarts, SMTP retries, certificate warnings, and concurrent access.

## Future

- [ ] **TODO-003** Define how target URL changes, renames, and removals should affect name-keyed state.

- [ ] **TODO-004** Report invalid state at startup and evaluate versioning, migration, and durability improvements.

- [ ] **TODO-005** Bound SMTP session duration and reject unsupported TLS mode settings.

- [ ] **TODO-006** Review reproducible image references and exclude private state and backups from the Docker build context.
