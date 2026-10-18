# Local service operating guide

This development product runs prompt-only bots on one local quantized base. Phase 2 adapters are unavailable; the baseline search-to-read quality failure remains unchanged. Remote service access, arbitrary shell/code execution, plugin credential forwarding and unused-weight deletion are unavailable.

## Install and start

Run `scripts/setup.ps1` once online, or `scripts/setup.ps1 -Offline` with existing pinned artifacts/dependencies. Start with `scripts/start.ps1`; use `-Port 8081` or `-Queue 4` if needed. Direct CLI: `bin/bots.exe serve -listen 127.0.0.1:8080 -queue 8`. Open its printed dashboard link. Treat that per-start token as private; each restart replaces it. Browser clients use Authorization headers and same-origin requests. Remote bind attempts fail.

Register a sample through CLI (`bot register bots/extract`) or the dashboard's local directory installer. Bot roots are explicit filesystem grants; review tools/folders before running. Directory packages retain absolute references, so keep their source directories. Archive imports copy validated payloads into `.local/packages`.

## API

All `/api/` routes require `Authorization: Bearer TOKEN`. JSON mutations also require `Content-Type: application/json`. Foreign origins and mismatched Host headers are rejected. GET `/api/bots` returns installed bot metadata, compatibility and resolved settings. GET `/api/runs` returns the latest 100 records; GET `/api/runs/ID/trace` returns the ordered local trace.

POST `/api/packages` with exactly one of `{"directory":"absolute-or-local-path"}` or `{"archive":"path.zip"}`. POST `/api/jobs` with `{"bot":"extract","input":"Invoice INV-104 from Cedar Supplies. Total USD 149.50."}`. Inspect GET `/api/jobs/ID`, cancel POST `/api/jobs/ID/cancel` with `{}`, or stream authenticated GET `/api/jobs/ID/events`. Events are redacted status/result snapshots, not token fragments. Disconnecting a watcher does not cancel its job. Queue saturation returns HTTP 429.

POST `/api/route` with `{"input":"@extract read this invoice"}` returns the deterministic choice. Multiple/no mentions return `needs_choice` and candidates. Explicit bot selection is the default; routing cannot change grants.

## Workflow authoring and recovery

POST `/api/workflows` with schema 1, an ID and 1..32 steps. Each step has `id`, `bot`, `input_type` and `output_type` (`text` or `json`), plus either literal `input` or `input_from`. Declare source IDs in `depends_on`; types must match. Cycles, missing bots, invalid values and duplicate IDs fail before execution. See [example](../examples/workflow.json).

Workflow workers submit through the same GPU queue, with at most eight concurrent workflow controllers. Every step gets a fresh session. Results cross only declared dependency edges; failed dependencies block their dependents. Independent steps may finish. `retries` 0..2 applies only to bots without tools.

Inspect GET `/api/workflows/ID`; cancel POST `/api/workflows/ID/cancel`. After restart, queued/running jobs become interrupted. Previously running SQLite runs are marked `restart_uncertain`. A `tool_started` without `tool_finished` can mean the action completed before the crash; inspect actual files and trace. File writes cannot be undone by cancellation.

POST `/api/workflows/ID/resume` is allowed only for interrupted workflows with completed/pending steps and unchanged bot versions. It skips completed results and validates output contracts. Uncertain or failed steps require review and a new explicit definition. Raw submission input is not saved in job records; literal workflow inputs are stored with pattern redaction. Retrieved context and user tasks may still contain sensitive local data.

## Baseline ZIP packages

At archive root, provide `bot.json` and `package.json`:

```json
{"schema_version":1,"id":"my-bot","version":"1.0.0","files":{"bot.json":"LOWERCASE_SHA256","workspace/example.txt":"LOWERCASE_SHA256"}}
```

List every regular payload file except package.json, using forward-slash relative names and exact SHA256. Bot ID/version must match. Context directories/evaluation files must stay inside the archive. Limits: 16 MiB compressed and unpacked, 1 MiB/file, 128 entries. Traversal, drive paths, backslashes, reserved Windows names, case duplicates, symlinks, special files, unlisted files and mismatched hashes fail. No installation hooks run. CLI import: `bin/bots.exe bot import example.zip`. These small baseline archives do not contain weights. Full trained package export/upgrade/rollback remains Phase 2.

## Trusted external tools

Build the sample with `go build -buildvcs=false -o bin/plugin-echo.exe ./cmd/plugin-echo`, add `"plugins":["examples/plugin.json"]` to framework.json, and explicitly grant `plugin-echo` in a bot's tools. Core code changes are unnecessary. Each invocation is a separate executable with protocol 1 and bounded JSON stdin/stdout. Arguments use a strict flat object schema supporting string, boolean, number and integer. Required fields, unknown fields and types are checked before execution. See [manifest](../examples/plugin.json), [SDK](../plugin/sdk.go) and [sample](../cmd/plugin-echo/main.go).

These executables are trusted code with the host account's permissions. Timeouts/output limits are not a filesystem/network sandbox. Packages cannot install plugins or widen framework configuration. No arbitrary shell/code tool is shipped. Credentials are not forwarded; plugins inherit only minimal Windows/temp environment entries. Recognized assignment/bearer/key patterns are redacted from tool outputs, persisted records and API responses. This does not detect every possible secret; CLI raw token streaming is not a secret-safe export path.

## Retention and upgrades

Stop the service before running `bin/bots.exe cleanup -before 2026-10-01T00:00:00Z -keep RUN_ID`. Default output is an inspectable plan. Add `-apply` only after reviewing it. Active runs, explicitly kept records and all workflow-referenced runs are preserved. Run messages/events and terminal job checkpoints are removed transactionally. Every model/adapter/package artifact is preserved until reference-aware artifact lifecycle exists.

For upgrades, stop service, run offline setup, then restart. Metadata schema version 2 adds checkpoints without changing existing session/run records. Existing older databases receive a SQLite snapshot beside metadata.db before migration. Backup failure aborts; newer unknown schemas are rejected. Restore while stopped by preserving the current database and sidecars first, then replacing metadata.db with the chosen backup. Never restore while SQLite or inference is running.

## Evidence and limits

`go test ./...`, `go test -race ./...`, and `go vet ./...` cover permissions, scheduling, recovery, typed workflows, archive attacks and persistence. Live API acceptance: `scripts/test-service.ps1 -KeepRunning`; browser acceptance: `node scripts/test-dashboard.mjs` (installed Chrome required); stop only that isolated test process with `scripts/stop-test-service.ps1`.

On this RTX 5050 laptop, the Phase 3 invoice and two-bot workflow preserve INV-104 / USD 149.50. The recorded completion snapshot is 3484 MiB aggregate device VRAM and 14037487616 bytes system used RAM; it is neither peak memory nor per-process allocation. Exact model/engine artifacts stay pinned in [baseline.lock.json](../artifacts/baseline.lock.json). Fake OOM failures test queue usability, not physical GPU exhaustion. Trained adapter equivalence/switching, full artifact cleanup and the final adapter demo remain incomplete.
