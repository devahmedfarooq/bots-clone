# Service

CLI validate/register and HTTP install -> application.Catalog -> strict bot config -> base match -> file roots and grants -> registry.
serve -> loopback listener -> process inference lock -> shared Store + Runner + model Manager -> bounded FIFO -> one worker.
Submit -> validate input/settings/bot -> checkpoint queued -> enqueue snapshot -> checkpoint running BEFORE Runner -> persist redacted result -> publish terminal state.
Catalog.Check validates bot/base/roots/grant definitions for directory installs, queue submissions and every workflow step before execution. Optional bot_version pins submissions atomically to the loaded bot snapshot.
Cancel queued -> mark cancelled; cancel running -> cancel only its context; never cancel another run.
Restart -> mark queued/running jobs interrupted and running model runs uncertain -> do not replay tool actions.
GET bots/runs/trace/jobs; POST packages/jobs/cancel; job events via SSE until terminal state.
Service stores no raw submission inputs. Streams contain redacted job snapshots, not raw model token fragments.
Access -> exact listener Host (DNS rebinding denied) -> same origin if present -> reject cross-site fetch -> constant-time per-start bearer token on every API/UI fragment -> JSON-only API mutations, bounded URL-encoded UI forms. No remote bind allowed.
Browser boot token comes from URL fragment, removed immediately; assets use strict self-only CSP and no external resources.
