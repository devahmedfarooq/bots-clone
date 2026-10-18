# Local development release

setup.ps1 -> verify pinned engine/model checksums -> dependencies/tests -> build executable -> diagnostics; -Offline disables downloads.
start.ps1 -> require existing binary/config -> loopback service -> printed per-start fragment token -> Ctrl+C graceful shutdown.
State Open -> inspect schema version -> reject newer schema -> existing older database: SQLite VACUUM INTO unique snapshot BEFORE migration -> backup failure aborts -> create additive checkpoint table -> schema version 2. Fresh DB skips backup.
Upgrade -> stop service -> offline setup -> start -> additive migration preserves sessions/runs/messages/events. Backups remain beside metadata.db; restore only while stopped.
This is a development baseline release. Adapter release, constrained arbitrary execution, external credential forwarding, and unused-weight cleanup are not included.
