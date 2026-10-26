# Performance boundaries

The gateway uses immutable copy-on-write native state, latest-state backpressure,
versioned WebSocket deltas, lazy command bodies and bounded history pages.
These are transport/presentation optimizations, not alternative execution state.

History reads and original-owner subscription recovery remain read-only. Lists,
notification subscription and selected-chat restoration run independently so a
slow background feed does not block first readable content. Catalog reads are
lazy and bounded. Client message nodes/body caches are reused where identities
and revisions remain valid.

Use `go test ./internal/desktop ./internal/server` for regression checks and
`scripts/perf-open.cjs` only in an explicitly configured local test environment.
Keep timing evidence and any private fixtures in ignored `.local/`. No historical
cross-project speed ratio is presented as current product acceptance.
