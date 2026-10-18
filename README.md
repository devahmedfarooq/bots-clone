# Local Bots

**A team of small AI bots that runs entirely on your own laptop.** Give a manager bot a goal in plain language. It splits the work, hands each piece to the right specialist bot, reviews what comes back, and shows you a result you can read, copy, download or send back for changes. No cloud, no API key, no account.

Everything runs through **one local model** ([Qwen3-1.7B](https://huggingface.co/Qwen/Qwen3-1.7B-GGUF) on [llama.cpp](https://github.com/ggml-org/llama.cpp)) on a single consumer GPU, one task at a time. It was built and measured on an RTX 5050 laptop (8 GB), and works offline after setup.

> **Honest scope.** This is a framework and a working studio around a *small* model. It is good at making a 1.7B model usable (valid structure, bounded cost, safe file access, recoverable failures). It does not make the model smarter: output still needs a human read. See [Known limitations](#known-limitations).

## Contents

- [What you get](#what-you-get)
- [Quick start](#quick-start-windows)
- [Using Bot Studio](#using-bot-studio)
- [The bots](#the-bots)
- [How it works](#how-it-works)
- [Guardrails that keep a small model reliable](#guardrails-that-keep-a-small-model-reliable)
- [Configuration and hardware](#configuration-and-hardware)
- [CLI cheat sheet](#cli-cheat-sheet)
- [HTTP API and security](#http-api-and-security)
- [Write your own bot](#write-your-own-bot)
- [Development](#development)
- [Known limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)
- [Status and documentation](#status-and-documentation)

## What you get

- **Bot Studio** (`/board`): describe a goal, press *Assign task*, watch progress, read the result. Choose **Manager decides** or pick one specialist yourself.
- **Manager delegation**: the manager plans tasks, assigns workers, and reviews each result. Plans are forced into valid JSON with only real worker names.
- **Specialist bots** for writing, web pages, code, math and QA, defined by one small JSON file each. Adding a bot needs no framework changes.
- **Private workspace per goal**: every goal gets its own folder for files its bots save, so goals never overwrite each other.
- **Safe file tools**: bots can only read, search and write inside folders they were granted. Path traversal, absolute paths and symlink or junction escapes are blocked and tested.
- **One shared GPU model**: weights load once and are reused, with a single inference slot and a bounded queue.
- **Crash-safe state**: goals, tasks, runs and traces live in SQLite. Interrupted work is shown for review and never silently replayed.
- **Optional LoRA adapters** with exact compatibility checks and a separate QLoRA training path (experimental, see [limitations](#known-limitations)).
- **Workflows, plugins and packages**: explicit multi-bot pipelines, a Go plugin SDK for external tools, and checksummed bot packages (ZIP).

## Quick start (Windows)

**Requirements:** Windows, [Go](https://go.dev/dl/) 1.26+, Git, PowerShell, `curl`, and a GPU driver with Vulkan support (CPU also works, slowly). Roughly 6 GB free disk (the model is about 1.84 GB). Python is **not** needed to run the app.

```powershell
git clone <your-repository-url> local-bots
cd local-bots

# 1. Download the pinned engine and model (checksum-verified), run the tests, build bin\bots.exe
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1

# 2. Install the bots Bot Studio uses (safe to re-run)
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/register-studio-bots.ps1

# 3. Start the service
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/start.ps1
```

Open **http://127.0.0.1:8080/board**. The first task is slower while the model loads. Stop the service with `Ctrl+C`.

Equivalent direct command: `.\bin\bots.exe -config framework.json serve -listen 127.0.0.1:8080 -queue 8`.

Prefer the main dashboard for installing bots and running a single bot by hand? Open **http://127.0.0.1:8080/**.

## Using Bot Studio

1. **Assign to**: leave **Manager decides** selected for multi-step goals, or pick one specialist (Writer, Landing page, Code, Math, QA) to skip planning and review and get a faster answer.
2. **Describe the goal** in plain language. Be specific: names, numbers, constraints. Press **Assign task** (or `Ctrl+Enter`).
3. **My work** lists every request with a plain status: *Queued, Working, Checking, Needs your OK, Needs attention, Done*.
4. **Read the result** on the right. Use **Copy**, **Download** (the file extension follows the content), **Live preview** for HTML (sandboxed, no access to your session), or **Diagnostics** for the raw model output.
5. **Ask for changes** sends a follow-up and re-runs a finished direct task. A blocked task has a **Try again** button with the reason shown above it.
6. **History and limits** shows the goal's workspace folder, token use, and editable limits. **Board view** shows every task by state.

Under **Advanced** you can set priority, token and run budgets, and approval rules (*ask me before any assistant runs*, *sign off on results*, *before file/tool use*). By default nothing waits for you.

Example goals that work well on a 1.7B model:

| Goal | What happens |
| --- | --- |
| `Write a short poem about the sea.` | One writer task, reviewed, result shown. |
| `Build a simple landing page for a coffee shop called Bean There.` | One landing-page task; the HTML appears with a live preview. |
| `Build a landing page for Bean There, and write a short announcement post.` | Two tasks (landing-page and writer), reviewed separately. |

Tip: a goal with no "and", comma or semicolon is planned as a single task. Keep tasks small; split large documents or programs into stages.

## The bots

| Bot | Use it for | File tools |
| --- | --- | --- |
| `manager` | Plans goals into tasks and reviews results (not a worker) | none |
| `writer` | Articles, posts, poems, ideas, documentation | read, search, write |
| `landing-page` | Responsive HTML/CSS pages | read, search, write |
| `code-writer` | Scripts and programs (drafts; nothing is executed) | read, search, write |
| `math-solver` | Step-by-step math | none |
| `qa-assistant` | Code review, edge cases, test ideas | read, search, write |
| `extract`, `workspace-qa`, `task-worker` | Baseline fixtures for the evaluation suite and examples | varies |

Bot definitions live in [`bots/`](bots). The fixture bots are not needed for Bot Studio; register them with `.\bin\bots.exe bot register bots/extract` if you want to run the baseline evaluation.

## How it works

```
 browser ──► Bot Studio / dashboard ──► HTTP service (loopback only)
                                              │
        ┌─────────────────────────────────────┼──────────────────────────┐
        ▼                                     ▼                          ▼
  Autonomy board                          Job queue                  CLI (same code)
  goals → tasks → review               one GPU slot, bounded
  (plan / work / verify)                       │
        │                                      ▼
        └──────────────► Runner ──► tool loop ──► scoped file tools ──► goal workspace
                           │                      (grants checked outside the model)
                           ▼
                   Provider interface ──► llama.cpp (Vulkan, one resident model)
                           │
                   SQLite: goals, runs, messages, traces, checkpoints
```

**A manager-led goal:**

1. **Plan**: the manager returns tasks as schema-constrained JSON (title, requirements, acceptance, owner). The owner must be a real worker; plans are validated; one correction attempt is allowed; repeated steps for the same worker are merged.
2. **Work**: each task runs on its worker with the goal, the task and prior results. The worker replies with the finished deliverable (plain text is fine) and may save files in the goal workspace.
3. **Review**: the manager checks the result against the acceptance criteria and approves or rejects it.
4. **Deliver**: multi-task goals get a **Final result** tab combining the outputs in order. You approve only where you asked to.

**Package map** (module pseudocode for each lives in [`specs/`](specs)):

| Path | Responsibility |
| --- | --- |
| `cmd/bots` | CLI and `serve` entry point |
| `internal/service` | HTTP API, queue, autonomy board (goals, tasks, review), workflows, dashboard |
| `internal/runner` | Model/tool loop, budgets, context fitting, cancellation |
| `internal/tools` | `file-read`, `file-search`, `file-write`, plugin tools, grants |
| `internal/provider` | Engine interface and the llama.cpp implementation, model residency |
| `internal/registry`, `internal/config`, `internal/packages` | Bot definitions, settings resolution, bot packages |
| `internal/state` | SQLite persistence, redaction |
| `internal/contextsource` | Retrieval of relevant text from granted folders |
| `internal/diagnostics`, `internal/evaluation`, `internal/lock` | Hardware report, fixture evaluation, single-model OS lock |
| `internal/artifacts`, `internal/dataset`, `training/` | Adapter artifacts and the separate QLoRA training path |
| `plugin/` | Go SDK for external tool plugins |

## Guardrails that keep a small model reliable

These exist because each one fixed a failure seen with a real 1.7B model. They are deterministic code, not prompt hopes.

| Problem | Guardrail |
| --- | --- |
| Manager returns broken JSON or invents a worker | Manager output is **schema-constrained** (valid JSON, owners limited to real workers, bounded fields) |
| A simple request is split into pointless tasks | Single-clause goals are capped at **one task**; same-worker steps are **merged** |
| A worker repeats its own task text as the answer | **Echo guard**: one retry, then a clear block, never a silent approval |
| A worker re-sends the same tool call forever | **Loop guard**: the duplicate is ignored, tools are withdrawn so it must answer in text |
| The result is only "created index.html" | **Deliverable recovery** uses the content actually written, and unwraps a tool call printed as text |
| Goals overwrite each other's files | **Per-goal workspace**; saving over an existing name creates `name-v2.ext` |
| A review prompt does not fit the context | Context **grows per task** (4096 up to 16384), and tool evidence is summarised instead of repeating files |
| A result is too long to review whole | **Excerpt review**, never auto-approved: you are asked to sign off, and the stored output stays complete |
| Runaway cost | Token, run, time and tool-call **budgets** per goal and per run, reserved before dispatch |
| Crash or restart mid-goal | State is checkpointed; interrupted work is **blocked for review**, never silently re-run |

## Configuration and hardware

[`framework.json`](framework.json) (paths resolve relative to the file):

```json
{
  "home": ".local",
  "engine": {
    "binary": "runtime/llama/llama-server.exe",
    "model_path": "models/Qwen3-1.7B-Q8_0.gguf",
    "model_id": "qwen3-1.7b-q8",
    "context_size": 16384,
    "device": "Vulkan0",
    "gpu_layers": 99
  }
}
```

- `context_size` is the **maximum** a run may grow to. Bots start at their own size (normally 4096); memory is only used while a task runs at that size.
- `gpu_layers: 0` runs on CPU. `device` picks the GPU (`runtime\llama\llama-server.exe --list-devices`).
- Optional `engine.cache_type` (`f16` or `q8_0`) shrinks the KV cache at some quality cost.
- State, logs and goal workspaces live under `home` (`.local/`, ignored by Git).

**Measured on the RTX 5050 laptop (8151 MiB VRAM, Vulkan, f16 KV cache)** (total GPU memory in use, including about 1.6 GB the desktop already used):

| Context | GPU memory in use |
| --- | --- |
| 4096 | 3,889 MiB |
| 8192 | 4,341 MiB |
| 16384 | 5,245 MiB |

That is about 113 KB of KV cache per context token. Larger contexts than 16384 were not adopted. The recorded baseline suite scores **9/12**; the known failure (search then read the matching file) is kept visible, not hidden. Details: [evaluation/README.md](evaluation/README.md).

## CLI cheat sheet

Put `-config path` before the command; flags go after the bot ID. Results are JSON by default.

```powershell
.\bin\bots.exe diagnostics                     # GPU, RAM, engine and model check
.\bin\bots.exe bot list                        # installed bots
.\bin\bots.exe bot inspect writer              # definition + effective settings
.\bin\bots.exe bot validate bots/writer        # check a definition without installing
.\bin\bots.exe bot register bots/writer        # install (a duplicate id fails on purpose)
.\bin\bots.exe bot remove writer               # uninstall
.\bin\bots.exe run writer -input "Explain local inference in 80 words." -json=false
.\bin\bots.exe run writer -interactive -stream # chat, model stays loaded
.\bin\bots.exe run show RUN_ID                 # a stored run
.\bin\bots.exe settings writer -temperature 0.3 -max-tokens 1024
.\bin\bots.exe eval -contexts 1024,2048,4096 -output evaluation/baseline.json
.\bin\bots.exe serve -listen 127.0.0.1:8080 -queue 8
```

More commands: `package install | upgrade | rollback | export | uninstall`, `artifact import`, `compare`, `dataset validate`, `route`, `cleanup`. The full reference is in the [user guide](docs/user-guide.md).

## HTTP API and security

Main endpoints (JSON; mutations need `Content-Type: application/json`):

| Method and path | Purpose |
| --- | --- |
| `GET /api/bots` | Installed bots, compatibility, settings |
| `POST /api/goals` | Create a goal (`requirements`, `direct`, `manager`, `workers`, budgets, `rules`) |
| `GET /api/board` | All goals and task cards |
| `POST /api/goals/{id}/chat`, `/settings` | Follow-up instruction, change limits |
| `POST /api/cards/{id}/approve \| reject \| retry` | Act on a task card |
| `POST /api/jobs`, `GET /api/jobs/{id}`, `/events` | Run one bot directly; stream events (SSE) |
| `GET /api/runs`, `/api/runs/{id}/trace` | Stored runs and ordered traces |
| `POST /api/workflows` | Explicit multi-bot workflow |
| `POST /api/packages` | Install a bot package (directory or checksummed ZIP) |

**Security model:** the service binds to loopback only and checks the exact `Host` and `Origin`, so a web page in your browser cannot drive it. **There is no login.** Anyone who can run code on your machine can use the API, so do not expose the port. File access is enforced in Go outside the model, independent of anything a prompt says. Trusted plugins are ordinary executables with your account's permissions, and there is no shell or code-execution tool.

## Write your own bot

A bot is one strict JSON file (schema version 1; unknown fields are errors). Save this as `bots/release-notes/bot.json`:

```json
{
  "schema_version": 1,
  "id": "release-notes",
  "version": "1.0.0",
  "description": "Turns a changelog into friendly release notes.",
  "instructions": "You write clear, friendly release notes. Reply with the notes only.",
  "base_model": "qwen3-1.7b-q8",
  "tools": [],
  "context_sources": [],
  "generation": {"temperature": 0.3, "max_tokens": 1024, "context_size": 4096},
  "budgets": {"tokens": 12000, "tool_calls": 0, "seconds": 180}
}
```

```powershell
.\bin\bots.exe bot validate bots/release-notes
.\bin\bots.exe bot register bots/release-notes
```

It then appears in Bot Studio's picker (and the manager can assign it). Grant `file-read`, `file-search` or `file-write` in `tools` only if it needs them. Packaging, adapters and ZIP export: [docs/bot-authoring.md](docs/bot-authoring.md).

## Development

```powershell
go vet ./...
go test ./...
go build -buildvcs=false -o bin/bots.exe ./cmd/bots
node scripts/test-board-empty-goal.mjs      # Studio page regression test (no browser needed)
```

- Tests cover configuration, registration, SQLite persistence, tool grants and filesystem escapes (including Windows junctions), budgets, cancellation, queueing, recovery, schema-constrained planning and the guardrails above. Live-model checks are recorded under [`evaluation/`](evaluation).
- Dashboard CSS (Tailwind) is only rebuilt when you change it: `npm install` then `npm run build:dashboard`.
- Offline rebuild after a first online setup: `scripts/setup.ps1 -Offline`.
- **Conventions** ([AGENTS.md](AGENTS.md)): one Git branch per feature, small readable code that reuses existing paths (DRY, KISS), and a pseudocode spec in [`specs/`](specs) for every module that acts as the project's memory. [`todo.md`](todo.md) tracks tickets and acceptance evidence.

## Known limitations

- **Small-model quality.** A 1.7B model can be shallow, chatty or wrong. The reviewer is lenient: "Done" means the pipeline finished, so read the result.
- **Over-splitting** can still happen on goals that contain "and" or commas.
- **Output is deterministic** (temperature 0 for the manager), so retrying the same prompt gives the same answer. Rephrase to change it.
- **Windows first.** Scripts are PowerShell and the pinned engine is the Windows Vulkan build. The Go code has non-Windows stubs, but other platforms need their own engine artifact.
- **No sandbox for executables.** Plugins are trusted code; code and shell tools are intentionally not shipped. Generated code is a draft you run yourself.
- **Experimental adapters.** The trained example adapters show no proven quality gain yet; see [evaluation/phase2.md](evaluation/phase2.md).
- **Not yet verified:** real GPU out-of-memory recovery and adapter-switching corruption checks (tracked in [todo.md](todo.md)).

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| Setup says *Checksum mismatch* | The download is incomplete; delete that file under `models/` or `runtime/` and rerun setup. |
| `Busy inference lock` | Another `bots` process owns the model. Stop it (only one model copy runs at a time). |
| Port 8080 in use | Start with `scripts/start.ps1 -Port 8081`. |
| *Offline, retrying…* in Studio | The service stopped. Start it again; your goals are saved. |
| Task *Needs attention* | Read the message above the result, then **Try again** or rephrase. |
| Out of GPU memory or no Vulkan device | Close GPU-heavy apps, lower `context_size`, or set `gpu_layers` to 0. Check `.local/engine.log`. |
| Git or Go cache errors on build | Use `go build -buildvcs=false`, as the setup script does. |

## Status and documentation

Phase 1 (shared runtime and configurable bots) is complete and Phase 3 (service, dashboard, workflows, recovery) is largely complete. Phase 2 (adapter specialization) is implemented but still experimental. The autonomous Studio, manager delegation, per-goal workspaces and the guardrails above came after those phases; [todo.md](todo.md) has the ticket-level status and evidence.

| Document | Contents |
| --- | --- |
| [docs/user-guide.md](docs/user-guide.md) | Step-by-step manual: Studio, prompts, budgets, approvals, CLI, packages |
| [docs/bot-authoring.md](docs/bot-authoring.md) | Bot packages, adapters, export and install |
| [docs/local-service.md](docs/local-service.md) | API examples, packages, workflows, plugins, recovery |
| [docs/autonomous-operation.md](docs/autonomous-operation.md) | Continuous operation, adaptive context, budgets |
| [docs/model-decision.md](docs/model-decision.md) | Why this model and engine |
| [evaluation/](evaluation) | Measurements and acceptance evidence |
| [training/README.md](training/README.md) | Separate Python QLoRA training path |
| [specs/](specs) | Pseudocode for every module |

**License:** not yet chosen for this repository's own code. Upstream: Qwen3 is Apache-2.0 and llama.cpp is MIT; keep their licenses with any redistributed artifacts.
