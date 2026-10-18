# Phase 3 reliability evidence

Automated contracts: bounded queue pressure; queued/active cancellation; a single execution slot and isolated outputs; generation/OOM fixture failure followed by a usable run; restart without replay; workflow checkpoint resume and uncertain-action refusal; corrupt/checksummed/oversized/traversal package rejection; authenticated same-origin API; secret-pattern redaction.

Fake providers verify orchestration deterministically. They do not establish GPU OOM recovery or adapter-switch equivalence. Adapter switching, corrupt adapter weights and trained-specialization release checks remain blocked by unfinished Phase 2.

Commands: `go test ./...`, `go test -race ./...`, `go vet ./...` with project-local Go caches. Live service acceptance is recorded separately.
