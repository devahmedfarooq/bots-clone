# Adapter Bot Framework: Implementation Tickets

## Goal

Build a local framework where registering a bot definition and a compatible adapter adds a specialized bot without changing framework code. Coding is one possible bot, not the framework's purpose.

Target: RTX 5050 laptop. Start with one shared quantized base model, one active bot at a time, and CPU orchestration. Measure actual VRAM, RAM, and context limits before setting defaults. Local inference must work offline after dependencies and model artifacts are downloaded; training can use external hardware.

## Working rules

- Each checkbox is an implementation ticket for the agent; mark it complete only after its acceptance condition passes.
- Execute phases in order. Ticket references identify dependencies; otherwise use listed order.
- Initialize Git before implementation and give each feature its own branch, such as `feature/p1-03-bot-schema`.
- Keep implementations small, readable, and reusable. Reuse validation, execution, and model-loading paths across bots.
- Use focused tests for contracts, permissions, state, and failures. Use real inference checks for model integration.
- Record hardware measurements and evaluation evidence; do not claim specialization gains without results.
- Require user input only for unavailable credentials, paid resources, or decisions that cannot be inferred safely.

## Core contracts

- **Bot definition:** ID, version, description, instructions, base-model reference, optional adapter reference, tool grants, context sources, generation settings, budgets, and evaluation suite.
- **Adapter manifest:** artifact checksum, source, license, exact base revision, architecture, tokenizer requirements, adapter format, and supported runtime/conversion path.
- **Run:** bot/version, input, isolated conversation state, tool events, outputs, usage, termination reason, and errors. Never store secrets in logs.
- **Extension boundary:** new bots use declarative definitions; new tool implementations use a documented plugin interface.

## Phase 1: Shared runtime and configurable bots

Deliverable: a CLI runs multiple prompt-configured bots through the same local model with isolated state and enforceable tool permissions. Establish the baseline before adding trained adapters.

- [x] **P1-01 — Bootstrap repository.** Initialize Git, create the project structure, and record the three-phase scope. Done when feature branches can be created and generated files/model weights are ignored.
- [x] **P1-02 — Establish the stack.** Use Go for the backend, orchestrator, CLI, and tool execution, with typed configuration, SQLite metadata, and an existing inference engine behind a provider interface. Connect to the inference engine through its supported API or process interface. Done when Go dependencies are pinned and a clean setup passes a smoke check; Python is not required for the Go backend.
- [x] **P1-03 — Define bot configuration.** Implement the core bot schema with versioning and clear validation errors. Done when valid definitions load and invalid IDs, settings, and references fail predictably.
- [x] **P1-04 — Implement configuration resolution.** Define precedence for framework defaults, bot settings, and per-run overrides; resolve artifact paths consistently. Done when effective settings are inspectable and invalid overrides are rejected.
- [x] **P1-05 — Inventory the laptop.** Detect GPU/VRAM, driver, RAM, and available disk. Done when a diagnostics command reports actual hardware and missing runtime dependencies.
- [x] **P1-06 — Select a baseline model and engine.** Verify license, tool-call behavior, quantization, hardware support, and an adapter deployment path. Done when one candidate loads and completes a local smoke task; record the decision.
- [x] **P1-07 — Implement inference provider interface.** Expose load, generate/stream, cancel, health, and unload operations. Done when the orchestrator has no engine-specific calls outside its provider implementation.
- [x] **P1-08 — Manage model residency.** Serialize GPU inference, reuse loaded weights, and release resources on shutdown. Done when repeated bot runs avoid duplicate model loads and simultaneous callers queue safely.
- [x] **P1-09 — Implement bot registry.** Register, list, inspect, validate, and remove local bot definitions. Done when a bot can be installed from a directory without editing framework source.
- [x] **P1-10 — Isolate run state.** Persist run IDs, bot versions, messages, and statuses separately. Done when separate bots/sessions cannot receive each other's conversation history.
- [x] **P1-11 — Implement tool contracts.** Define typed arguments/results, errors, timeouts, output limits, and tool registration. Done when malformed calls fail before execution and tool errors return structured results.
- [x] **P1-12 — Enforce tool grants.** Check each call against the bot's declared capabilities. Done when unknown and ungranted tools are denied independently of model instructions.
- [x] **P1-13 — Add scoped file tools.** Implement read, search, and controlled write tools. Done when traversal, absolute-path escape, and symlink/junction escape checks prevent access outside allowed roots.
- [x] **P1-14 — Implement the execution loop.** Connect input, generation, tool calls, tool results, and final output. Done when a bot completes a multi-step fixture and stops at its token, time, or tool-call budget.
- [x] **P1-15 — Add CLI commands.** Support diagnostics, bot registration/listing/inspection, interactive runs, and machine-readable results. Done when two bots run through the same interface with meaningful exit codes.
- [x] **P1-16 — Add cancellation and failure handling.** Handle model failure, invalid output, tool timeout, and user cancellation with bounded retries. Done when failed runs terminate clearly and later runs still work.
- [x] **P1-17 — Add two baseline bots.** Provide a structured document-extraction bot and a workspace-question-answering bot using the same model. Done when each has a definition, fixtures, and expected outputs with no role-specific framework branches.
- [x] **P1-18 — Establish baseline evaluation.** Measure fixture pass rate, latency, token usage, and peak VRAM/RAM at several context sizes. Done when results and practical laptop defaults are saved reproducibly.
- [x] **P1-19 — Document local setup.** Write installation, model acquisition, CLI usage, troubleshooting, and offline startup instructions. Done when a clean environment can reproduce the baseline demo.

**Phase exit:** both bots run locally through one shared model, state stays isolated, permissions hold, and the baseline has recorded measurements. Trained adapters are not required yet.

**Phase 1 completed (2026-10-18), with an explicitly approved quality deferral.**

- Go CLI/runtime, SQLite isolation, registry, settings, tool permissions, confined file access, budgets, cancellation and model residency are implemented. Each implementation ticket has its feature branch and module pseudocode spec.
- All contract tests and `go vet ./...` pass. The documented offline setup verifies pinned artifacts, uses cached dependencies, rebuilds and passes its smoke checks.
- Real Qwen3-1.7B Q8_0 / llama.cpp Vulkan runs measured contexts 1024, 2048 and 4096. Baseline: **9/12**; exactly one model process per context, shared across both bots. Measurements and tested laptop defaults: [evaluation/README.md](evaluation/README.md).
- **Deferred by user:** search finds the correct file but the prompt-only bot chooses a different path for its next read. Keep the failed fixture and nonzero evaluation exit; reassess with QLoRA adapters in Phase 2. No claim that training will automatically fix it.
- Prompt-only bot definitions are baseline fixtures. Intended specialization comes from **QLoRA-trained adapters**; JSON remains configuration, package metadata and grants.

## Phase 2: Adapter-backed specialization and bot packages

Deliverable: installing a compatible adapter-backed package creates a new specialized bot; switching bots safely switches adapters without changing the base model or leaking state.

- [x] **P2-01 — Specify adapter compatibility.** Implement the adapter manifest contract. Done when compatibility uses exact base/tokenizer/runtime requirements rather than parameter count or model family alone.
- [x] **P2-02 — Prove the deployment path.** Depends on P1-06. Test one compatible adapter against the chosen quantized base and engine, including conversion if needed. Done when reference and deployed outputs are evaluated on the same fixtures.
- [x] **P2-03 — Implement adapter provider operations.** Add validate, load, activate, deactivate, and unload behind the provider interface. Done when unsupported formats/features produce actionable errors.
- [x] **P2-04 — Add artifact storage.** Store versioned model/adapter metadata, checksums, and provenance without committing weights. Done when missing, corrupted, and mismatched artifacts are rejected before activation.
- [x] **P2-05 — Implement atomic adapter switching.** Drain active generation, change the adapter, and clear or segregate incompatible caches. Done when switching A → B → base produces isolated, repeatable runs.
- [x] **P2-06 — Implement switch recovery.** Roll back to a known usable state after failed activation or insufficient memory. Done when an invalid adapter cannot leave the service using the wrong bot weights.
- [x] **P2-07 — Define portable bot packages.** Bundle definition, adapter manifest, artifact references, prompt templates, context configuration, and evaluation fixtures. Done when packages have a documented versioned layout.
- [x] **P2-08 — Implement package lifecycle.** Add local install, validate, upgrade, rollback, export, and uninstall. Done when a third bot can be added through a package alone and removing it preserves shared artifacts still in use.
- [x] **P2-09 — Implement context source interface.** Support explicit files/directories and relevant text retrieval within granted roots. Done when input size is bounded and sources remain attributable.
- [x] **P2-10 — Add context budgeting.** Reserve space for instructions, tool results, and output; retrieve or compact the remainder. Done when long tasks stay within the model's configured context limit.
- [x] **P2-11 — Harden untrusted inputs.** Keep retrieved documents/tool outputs separate from authority and enforce permissions outside the model. Done when adversarial fixtures cannot grant tools or widen filesystem access.
- [x] **P2-12 — Define specialization datasets.** Document role/task scope, example schema, source/license tracking, train/validation/test splits, and deduplication. Done when evaluation fixtures are excluded from training.
- [x] **P2-13 — Build dataset validation.** Check formatting, expected schemas, duplicates, source metadata, and sensitive data. Done when rejected records include reasons and clean records can be exported for training.
- [x] **P2-14 — Add reproducible QLoRA training configuration.** Keep Python-based ML training tools separate from the Go backend. Record exact base/tokenizer revisions, seed, data version, 4-bit training quantization, LoRA settings, and environment. Done when a small training smoke run produces a loadable artifact and manifest consumable by the inference provider; use external GPU hardware only if authorized and needed.
- [x] **P2-15 — Train or acquire the first QLoRA-trained task adapter.** Use verified task data or a compatible licensed artifact. Done when provenance is recorded and the adapter passes P2-02 through the normal package path.
- [x] **P2-16 — Compare specialization fairly.** Compare base-only, role-prompted base, and adapter plus role prompt with matching task/context/tool budgets. Include the deferred Phase 1 search-to-read fixture and held-out variants. Done when held-out quality, latency, and memory results show gains, regressions, or no improvement explicitly.
- [x] **P2-17 — Validate a second specialization.** Add a materially different adapter-backed bot using the same base and contracts. Done when it requires no framework code changes and passes compatibility and switching checks.
- [x] **P2-18 — Optimize laptop memory.** Measure context limits, cache precision, adapter memory, and switch overhead. Done when a stable configuration fits measured available VRAM and documents any CPU offload.
- [x] **P2-19 — Document bot authoring.** Provide a minimal package template and steps for prompt-only and trained-adapter bots. Done when the template installs and its example evaluation runs end to end.

**Phase exit:** two adapter-backed specializations install and switch through the public package interface, compatibility failures are safe, and held-out comparisons are published. An adapter that offers no measured benefit remains experimental.

**Phase 2 completed (2026-10-18).** All 19 tickets have implementation and acceptance evidence: [report](evaluation/phase2.md), [machine-readable summary](evaluation/phase2-summary.json). Two locally trained adapters install and switch (9/9; one process). Matched synthetic comparison: base 2/22, role prompt 3/22, adapter + role 19/22. Both adapters remain experimental; three search-to-read cases still fail. Real GPU OOM was not forced; P3-15 remains pending. Work is isolated on `feature/p2-completion`.

## Phase 3: Reliable workflows and usable local product

Deliverable: a dependable local service and dashboard for installing bots, running individual tasks or explicit workflows, inspecting results, and reproducing behavior.

- [x] **P3-01 — Add the service API.** Expose bot/package inspection, runs, cancellation, and streamed events using existing application services. Done when CLI and API share the same validation, permissions, and execution paths.
- [x] **P3-02 — Secure local service boundaries.** Bind locally by default, restrict origins, and authenticate any explicitly enabled remote access. Done when unauthorized requests cannot run tools or install packages.
- [x] **P3-03 — Build a minimal dashboard.** Show installed bots, compatibility, effective settings, active/queued runs, outputs, and failures. Done when a user can install a local package, choose a bot, run a task, and cancel it.
- [x] **P3-04 — Add resource-aware scheduling.** Keep a single GPU execution slot with queue limits and cancellation. Done when multiple API/UI requests cannot accidentally create concurrent model copies or cross-wire outputs.
- [x] **P3-05 — Add explicit workflow definitions.** Define ordered/dependent bot steps with typed inputs/outputs. Done when a workflow validates before execution and routes step results without sharing unrelated histories.
- [x] **P3-06 — Implement workflow execution.** Schedule ready steps, checkpoint transitions, and stop or retry under declared rules. Done when a two-bot workflow completes and a failed dependency blocks dependent steps correctly.
- [x] **P3-07 — Add resumable runs.** Restore checkpoints after restart and distinguish completed from uncertain tool actions. Done when recovery does not blindly repeat writes or external side effects.
- [x] **P3-08 — Add optional task routing.** Start with explicit selection and deterministic matching; permit a small router later if evaluation justifies it. Done when ambiguity requests a choice and routing cannot override tool grants.
- [x] **P3-09 — Complete the tool plugin SDK.** Use a versioned subprocess protocol for external tool plugins, with a Go SDK and validated plugin metadata. Treat plugin executables as trusted code. Done when a sample plugin registers without core edits and model use still requires grants.
- [ ] **P3-10 — Isolate executable tools.** If shell/code tools are enabled, run them in a constrained subprocess/container with bounded resources and explicit mounts. Done when timeout, network, and filesystem restrictions are verified before exposing those tools to bots.
- [ ] **P3-11 — Handle credentials securely.** Resolve opt-in external-tool credentials outside model context and redact sensitive event fields. Done when prompts, logs, exports, and package manifests contain no fixture secrets.
- [x] **P3-12 — Add observability.** Record run status, durations, tool failures, model/adapter versions, memory, and termination reasons. Done when a failed run can be diagnosed from its local trace without logging secrets.
- [ ] **P3-13 — Add retention and cleanup.** Configure run-history retention and remove unused artifacts with reference checks and an inspectable deletion plan. Done when cleanup preserves active packages, shared weights, and requested run records.
- [x] **P3-14 — Harden package imports.** Validate archive paths, checksums, size limits, and manifests; never execute package install hooks automatically. Done when malicious or oversized package fixtures fail safely.
- [ ] **P3-15 — Run reliability evaluation.** Cover repeated switching, queue pressure, cancellation, corrupt artifacts, out-of-memory recovery, and service restart. Done when there are no unresolved state leaks or silent wrong-adapter runs.
- [x] **P3-16 — Run end-to-end acceptance.** Exercise CLI, API, dashboard, package installation, individual bots, and workflows. Done when documented examples produce expected outputs on the target laptop.
- [x] **P3-17 — Package the local release.** Provide repeatable installation/startup, dependency checks, upgrade notes, and metadata migrations with backups. Done when a fresh install and an upgrade both pass smoke checks.
- [x] **P3-18 — Publish operating documentation.** Explain extension authoring, adapter compatibility, limitations, hardware measurements, recovery, and evaluation. Done when another developer can add a bot without modifying the framework.
- [ ] **P3-19 — Prepare the final demo and release evidence.** Demonstrate prompt-only bots, adapter-backed bots, safe switching, and a multi-bot workflow. Done when the release includes exact versions, benchmark results, known limitations, and reproducible commands.

**Phase exit:** a fresh installation can add and run a new compatible bot package through CLI/UI without framework edits; workflows recover predictably, permissions hold, and laptop performance is documented.


**Phase 3 development status (2026-10-18): 14/19 tickets complete; Phase 3 is not fully complete.**

User requested Phase 3 before Phase 2. Each Phase 3 ticket has a feature branch; tested changes are integrated into `development`. `main` remains the Phase 1 baseline.

- API/dashboard share Catalog validation, Runner permissions, SQLite state and one model manager. Loopback only, exact Host/origin checks, per-start bearer authentication, bounded queue and cancellation.
- Typed DAG workflows validate before execution, isolate step sessions, block failed dependencies and resume completed/pending checkpoints without replaying uncertain effects. Trusted subprocess SDK, run/tool lifecycle traces and checksummed baseline ZIP imports are implemented.
- Tests, race detection and vet pass. Offline setup passes. Real Qwen invoice extraction, SSE, cancellation, two-bot workflow and browser installation/run/trace/cancel/mobile checks passed: [API evidence](evaluation/phase3-acceptance.json), [browser evidence](evaluation/dashboard-acceptance.json). Fresh and prior-schema metadata tests verify backups and preserved records.
- **P3-10 pending:** arbitrary shell/code tools remain disabled; trusted plugins have host permissions. No claim of verified filesystem/network sandboxing.
- **P3-11 pending:** pattern redaction is implemented. Automatic approval review rejected environment credential forwarding because sensitive sources/recipient plugins were not explicitly authorized. Credential forwarding remains disabled.
- **P3-13 partial:** inspectable run retention protects active/requested/workflow records. All artifacts are preserved; reference-aware unused-weight deletion requires Phase 2 storage/lifecycle.
- **P3-15 partial:** queue, cancellation, restart, corrupt baseline archives and fake OOM recovery pass. Phase 2 verifies live adapter switching and corruption rejection; real GPU OOM recovery remains unverified.
- **P3-19 pending:** prompt-only product evidence exists; trained adapter demo and final adapter release evidence require Phase 2.
- P3-17/P3-18 describe the verified local development release and supported baseline extensions. Full trained package upgrade/rollback/export remains Phase 2.

## Deferred scope

These are not prerequisites for the three phases: a custom inference engine, training a base model from scratch, simultaneous GPU workers, a public bot marketplace, arbitrary cross-base adapter loading, reinforcement learning, or claims of general autonomous intelligence.
## Autonomous bot operation and interface

- [x] Provide a Kanban dashboard and chat interface for instructions to the manager bot.
- [x] Let users set goals, priorities, budget limits, and approval rules.
- [x] Use Inbox -> Ready -> In Progress -> Review -> Done, plus Blocked and Needs Your Approval states.
- [x] Have the manager bot break goals into tasks, assign worker bots, track progress, verify results, and confirm completion.
- [x] Have worker bots pick up assigned cards and communicate progress, questions, blockers, and results through the board.
- [x] Store requirements, ownership, decisions, history, and outputs on each task card.
- [x] Let the manager approve routine work automatically and request user approval according to configured rules.
- [x] Support 24/7 background operation with persistent storage, queued jobs, automatic restarts, and recovery after interruptions.
- [x] Support the local RTX 5050 laptop; document that continuous operation requires the host to remain powered on and awake.
- [x] Write the module pseudocode spec before implementing this workflow on a dedicated feature branch.

## Human usability (planned)

Implementation order and acceptance: [human-usability plan](docs/human-usability-plan.md).
Module pseudocode: [human-usability spec](specs/human-usability.spec.md).

- [x] HU-01: Readable worker results with copy/download and visible review status.
- [ ] HU-02: Persist and present one verified final deliverable per goal.
- [ ] HU-03: Minimal realistic plans with valid workers and complete delivery.
- [ ] HU-04: Clear connection recovery and simple goal creation.
- [ ] HU-05: Meaningful progress, linked job details and actionable failures.
- [ ] HU-06: Contextual approvals, answers and tracked revisions.
- [ ] HU-07: Shared navigation, responsive layouts and accessibility.
- [ ] HU-08: Real-model/browser acceptance for the full goal-to-result journey.

## Bot Studio usability (BS)

Problem: studio needs a manager plan + per-task review by a 1.7B model before any result appears; UI is a 7-column kanban with a hidden-required form, broken chat form, `alert()` errors, and a preview iframe sharing the app origin. Spec: [bot-studio spec](specs/bot-studio.spec.md). Branch: `feature/studio-usability`.

- [x] BS-01: Direct mode (backend). Goal `direct` skips manager planning/review: one work card owned by the chosen assistant; plain-text output accepted as the result; marked Done. Manager optional. Tests.
- [x] BS-02: Studio UI rewrite. Pick assistant -> describe -> Run (Ctrl+Enter). "My work" list with plain statuses (Working / Done / Needs you / Failed). Result shown inline with Copy, Download, sandboxed Live Preview. Kanban moved into an optional "Board view". Direct by default; manager planning under Advanced.
- [x] BS-03: Fix defects: chat form crash, `allow-same-origin` preview sandbox, hidden `required` selects, `alert()`, stuck "Connecting…", token-expired recovery message.
- [x] BS-04: Update spec, keep `go vet`/`go test ./internal/service` green, JS syntax check.

## Manager delegation (BS-05)

Goal: user states one goal; the manager bot splits it into subtasks, assigns the small bots and reviews them. No manual bot picking. Branch `feature/manager-delegation`. Spec: [bot-studio spec](specs/bot-studio.spec.md) MANAGER_DELEGATION.

- [x] BS-05a: Studio defaults to "Manager decides" with all workers; picking one bot (direct) stays as an override.
- [x] BS-05b: Reliable plans: fewest tasks, exact worker IDs in the prompt, specific rejection reason, one bounded correction attempt.
- [x] BS-05c: Final result tab combining reviewed subtask outputs; visible "N of M subtasks done".
- [x] BS-05d: Verify with the real local model (poem + multi-part goal); record failures honestly.

BS-05 live evidence (real Qwen3-1.7B, 2026-10-19): poem -> 1 task, writer, reviewed, Done. Coffee shop landing page + announcement -> 2 tasks (landing-page, writer), both reviewed, Done in 5 runs / ~15k tokens.
Known limits: the 1.7B reviewer is lenient (approved a one-paragraph "3-stanza" poem; writer output can include chatty preamble). Delegated planning needed a worked example, structured-field tolerance and a one-shot correction to be reliable. Not tested: approvals, restart mid-goal, UI in a real browser against live data.

## Goal workspaces (WS)

Plan from the file/workspace review. Spec: [goal workspace spec](specs/goal-workspace.spec.md). Branch `feature/goal-workspaces`.

- [x] WS-01: Per-goal workspace folder, path-only file tools (root optional), versioned writes, parent dirs auto-created.
- [ ] WS-02: Artifact table (goal, task, path, hash, version, run) + Files tab (preview/download/zip).
- [ ] WS-03: Task inputs/outputs in the manager plan; per-task grants (workers write only their output area).
- [ ] WS-04: Projects grouping goals; attached read-only input folders; retention/cleanup.

WS-01 live evidence (real Qwen3-1.7B, 2026-10-19): the coffee-shop landing page + announcement goal ran twice with identical file names; both finished Done, each saving index.html/announcement.txt in its own `.local/workspaces/<goal>/` folder, with nothing written into the repo.

CTX-16K (2026-10-19): engine.context_size raised 8192 -> 16384 after measuring VRAM on the RTX 5050 (3889 / 4341 / 5245 MiB at 4096 / 8192 / 16384, ~113 KB per token, f16 KV). About 2.9 GB stays free. 32768 would need roughly 7 GB total and was not adopted. Normal bot context stays 4096; growth is automatic and per task.
