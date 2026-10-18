# Local Adapter Bot Framework

The Go CLI and local service run declarative prompt bots through one resident llama.cpp model. The `development` branch includes the dashboard, workflows, recovery and bounded baseline package imports. Trained adapters and their lifecycle remain Phase 2 work. See [todo.md](todo.md) for acceptance status and [specs](specs/project.spec.md) for module pseudocode.

Prompt-only bots are temporary baseline fixtures. The intended specialized bots use QLoRA-trained adapters; JSON remains their configuration and metadata. The recorded baseline is **9/12**, with the search-to-read quality failure explicitly deferred to adapter evaluation. `eval` currently exits 1 for that known failure; expectations have not been weakened.

## Install on Windows

Requirements: Go 1.26 or later, Git, PowerShell, curl, and a GPU driver supporting Vulkan (or CPU inference). No Python is required. The pinned model is about 1.84 GB; allow several GB more for the engine and Go cache. Artifacts, metadata and caches stay in this project and are ignored by Git.

From the repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1
.\bin\bots.exe bot register bots/extract
.\bin\bots.exe bot register bots/workspace-qa
.\bin\bots.exe eval -contexts 1024,2048,4096 -output evaluation/baseline.json
```

Setup verifies the exact artifact checksums in [baseline.lock.json](artifacts/baseline.lock.json), installs pinned Go dependencies, runs tests and builds the CLI. Re-registering an existing ID fails deliberately; inspect it or remove its definition first. Registration retains absolute references to its package directory, so keep that directory in place.

## Use

```powershell
.\bin\bots.exe diagnostics
.\bin\bots.exe bot list
.\bin\bots.exe bot inspect workspace-qa
.\bin\bots.exe bot validate bots/workspace-qa
.\bin\bots.exe settings extract -temperature 0 -max-tokens 200
.\bin\bots.exe run extract -input 'Invoice INV-104 from Cedar Supplies. Total USD 149.50.'
.\bin\bots.exe run workspace-qa -input 'Who owns Project Cedar? Cite your source.'
.\bin\bots.exe run workspace-qa -interactive -stream
.\bin\bots.exe run workspace-qa -session SESSION_ID -input 'What database does it use?'
.\bin\bots.exe run show RUN_ID
```

Put global `-config path` before the command and run flags after the bot ID. Run results are JSON by default; `-json=false` prints the answer. Streams go to stderr, preserving JSON on stdout. Interactive mode keeps the model resident and reuses only that session; `/quit` exits and Ctrl+C cancels. Every ordinary run creates a fresh isolated session unless `-session` is explicit. History can only resume under the same bot ID and version. Exit codes: 0 success/help, 1 failure, 130 interruption.

`framework.json` resolves paths relative to itself. Generation precedence is framework defaults (temperature 0, max 512, context 4096), then bot settings, then run flags. Engine `context_size` is the maximum allowed resident context; each run loads its effective size. The same base and context reuse weights; context changes unload before reloading. `device: Vulkan0` selects the RTX GPU on this laptop; inspect `runtime/llama/llama-server.exe --list-devices` on other machines. One OS lock prevents simultaneous CLI model copies, and the runtime queues concurrent callers within a process. Independent CLI invocations unload on exit; use interactive mode or evaluation for residency across tasks.

## Add a bot

Copy a sample directory, set a new lowercase ID and version, and edit its instructions, grants, generation settings, budgets, context roots and fixture suite. Run `bot validate DIRECTORY`, then `bot register DIRECTORY`. The definition is strict JSON schema version 1; unknown fields and broken references fail. The base alias must match `framework.json`. No framework edits are needed.

Available file tools: `file-read`, `file-search`, `file-write`. Only declared grants are exposed and every call is independently checked. Arguments use an explicit root index and relative path. Canonical root handles block traversal and symlink/junction escapes. Writes create new files only, never overwrite. Read/write data is limited to 64 KiB, search/output to 16 KiB, and search visits at most 2000 entries. Tool timeouts default to five seconds. Trusted external Go tools can use the [plugin SDK](docs/local-service.md); arbitrary shell/code tools remain disabled.

Run budgets count prompt plus completion tokens across all turns, tool calls and total seconds (including queue/load). Each generation also reserves its context/output limits. Length truncation is a failed run. Inference gets at most one retry before any streamed text; tools are never retried automatically. Failed messages remain in the audit database but do not reenter resumed conversation history.

## Dashboard and workflows

The dashboard uses **HTMX + Tailwind CSS** with centralized Go template components. See [dashboard authoring](docs/dashboard.md) for the component structure and asset build commands.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/start.ps1
```

Open the link printed by the service. It includes a per-start access token; the dashboard removes it from the address bar. Install a trusted local directory or checksummed ZIP, choose a bot, submit a task, inspect output/trace, or cancel it. One bounded queue shares the GPU. The service listens only on loopback and runs offline after setup. Ctrl+C stops it.

Workflows use explicit text/JSON dependencies, checkpoint each transition, and keep bot histories separate. Safe completed/pending checkpoints can resume; uncertain tool actions cannot be replayed automatically. API examples, package format, extension authoring, retention and recovery: [local operating guide](docs/local-service.md). Live laptop and browser evidence: [Phase 3 acceptance](evaluation/phase3-acceptance.json), [dashboard checks](evaluation/dashboard-acceptance.json).

## Offline operation

After online setup, normal CLI startup needs only the executable, `framework.json`, registered bot directories, the unpacked engine and GGUF. The engine starts with `--offline` on an ephemeral loopback port; runtime does not contact remote inference services. No API key is needed.

To reproduce the build using downloaded Go modules and artifacts:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1 -Offline
```

The offline script disables Go dependency/checksum/toolchain downloads. Keep `.cache` to rebuild offline. A previously built CLI does not need Go or the module cache to run.

## Troubleshooting and verification

- Missing engine/model: rerun setup online. A checksum mismatch means the download is incomplete or changed; remove that specific ignored artifact and rerun setup.
- No Vulkan device or out of memory: close GPU-heavy applications, reduce context, or set `engine.gpu_layers` to 0 for CPU mode. Inspect `.local/engine.log`; the default engine logging level excludes prompts.
- `llama-server` missing from PATH is harmless when `configured_engine` is available. The CLI uses its configured absolute binary path.
- Busy inference lock: another CLI process is active. Finish/cancel it; OS locks release automatically after exit, without deleting lock files.
- Invalid/oversized context: keep run context within the configured engine maximum and reserve output tokens. Tool schemas consume context too.
- Git ownership errors during build: setup uses `-buildvcs=false`; Git commands can use a command-scoped `-c safe.directory=ABSOLUTE_REPO` when running under a different Windows account.
- State lives in `.local/metadata.db`; application records redact recognized credential assignments and bearer/key patterns. Tasks and retrieved files are still local persisted data; avoid submitting unrelated secrets. Redaction is pattern-based.

```powershell
$env:GOCACHE = "$PWD/.cache/go-build"
$env:GOMODCACHE = "$PWD/.cache/go-mod"
go test ./...
go vet ./...
go build -buildvcs=false -o bin/bots.exe ./cmd/bots
```

Tests cover configuration, package registration, SQLite persistence and ownership, tools/grants/timeouts, Windows junction escapes, HTTP/SSE parsing, cancellation, budgets, inference queueing and recovery. Live inference evidence and laptop defaults are recorded in [evaluation](evaluation/README.md). Fixture success is a small baseline, not a claim of general quality or adapter improvement.
