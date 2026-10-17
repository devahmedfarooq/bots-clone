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

- [ ] **P2-01 — Specify adapter compatibility.** Implement the adapter manifest contract. Done when compatibility uses exact base/tokenizer/runtime requirements rather than parameter count or model family alone.
- [ ] **P2-02 — Prove the deployment path.** Depends on P1-06. Test one compatible adapter against the chosen quantized base and engine, including conversion if needed. Done when reference and deployed outputs are evaluated on the same fixtures.
- [ ] **P2-03 — Implement adapter provider operations.** Add validate, load, activate, deactivate, and unload behind the provider interface. Done when unsupported formats/features produce actionable errors.
- [ ] **P2-04 — Add artifact storage.** Store versioned model/adapter metadata, checksums, and provenance without committing weights. Done when missing, corrupted, and mismatched artifacts are rejected before activation.
- [ ] **P2-05 — Implement atomic adapter switching.** Drain active generation, change the adapter, and clear or segregate incompatible caches. Done when switching A → B → base produces isolated, repeatable runs.
- [ ] **P2-06 — Implement switch recovery.** Roll back to a known usable state after failed activation or insufficient memory. Done when an invalid adapter cannot leave the service using the wrong bot weights.
- [ ] **P2-07 — Define portable bot packages.** Bundle definition, adapter manifest, artifact references, prompt templates, context configuration, and evaluation fixtures. Done when packages have a documented versioned layout.
- [ ] **P2-08 — Implement package lifecycle.** Add local install, validate, upgrade, rollback, export, and uninstall. Done when a third bot can be added through a package alone and removing it preserves shared artifacts still in use.
- [ ] **P2-09 — Implement context source interface.** Support explicit files/directories and relevant text retrieval within granted roots. Done when input size is bounded and sources remain attributable.
- [ ] **P2-10 — Add context budgeting.** Reserve space for instructions, tool results, and output; retrieve or compact the remainder. Done when long tasks stay within the model's configured context limit.
- [ ] **P2-11 — Harden untrusted inputs.** Keep retrieved documents/tool outputs separate from authority and enforce permissions outside the model. Done when adversarial fixtures cannot grant tools or widen filesystem access.
- [ ] **P2-12 — Define specialization datasets.** Document role/task scope, example schema, source/license tracking, train/validation/test splits, and deduplication. Done when evaluation fixtures are excluded from training.
- [ ] **P2-13 — Build dataset validation.** Check formatting, expected schemas, duplicates, source metadata, and sensitive data. Done when rejected records include reasons and clean records can be exported for training.
- [ ] **P2-14 — Add reproducible QLoRA training configuration.** Keep Python-based ML training tools separate from the Go backend. Record exact base/tokenizer revisions, seed, data version, 4-bit training quantization, LoRA settings, and environment. Done when a small training smoke run produces a loadable artifact and manifest consumable by the inference provider; use external GPU hardware only if authorized and needed.
- [ ] **P2-15 — Train or acquire the first QLoRA-trained task adapter.** Use verified task data or a compatible licensed artifact. Done when provenance is recorded and the adapter passes P2-02 through the normal package path.
- [ ] **P2-16 — Compare specialization fairly.** Compare base-only, role-prompted base, and adapter plus role prompt with matching task/context/tool budgets. Include the deferred Phase 1 search-to-read fixture and held-out variants. Done when held-out quality, latency, and memory results show gains, regressions, or no improvement explicitly.
- [ ] **P2-17 — Validate a second specialization.** Add a materially different adapter-backed bot using the same base and contracts. Done when it requires no framework code changes and passes compatibility and switching checks.
- [ ] **P2-18 — Optimize laptop memory.** Measure context limits, cache precision, adapter memory, and switch overhead. Done when a stable configuration fits measured available VRAM and documents any CPU offload.
- [ ] **P2-19 — Document bot authoring.** Provide a minimal package template and steps for prompt-only and trained-adapter bots. Done when the template installs and its example evaluation runs end to end.

**Phase exit:** two adapter-backed specializations install and switch through the public package interface, compatibility failures are safe, and held-out comparisons are published. An adapter that offers no measured benefit remains experimental.

## Phase 3: Reliable workflows and usable local product

Deliverable: a dependable local service and dashboard for installing bots, running individual tasks or explicit workflows, inspecting results, and reproducing behavior.

- [ ] **P3-01 — Add the service API.** Expose bot/package inspection, runs, cancellation, and streamed events using existing application services. Done when CLI and API share the same validation, permissions, and execution paths.
- [ ] **P3-02 — Secure local service boundaries.** Bind locally by default, restrict origins, and authenticate any explicitly enabled remote access. Done when unauthorized requests cannot run tools or install packages.
- [ ] **P3-03 — Build a minimal dashboard.** Show installed bots, compatibility, effective settings, active/queued runs, outputs, and failures. Done when a user can install a local package, choose a bot, run a task, and cancel it.
- [ ] **P3-04 — Add resource-aware scheduling.** Keep a single GPU execution slot with queue limits and cancellation. Done when multiple API/UI requests cannot accidentally create concurrent model copies or cross-wire outputs.
- [ ] **P3-05 — Add explicit workflow definitions.** Define ordered/dependent bot steps with typed inputs/outputs. Done when a workflow validates before execution and routes step results without sharing unrelated histories.
- [ ] **P3-06 — Implement workflow execution.** Schedule ready steps, checkpoint transitions, and stop or retry under declared rules. Done when a two-bot workflow completes and a failed dependency blocks dependent steps correctly.
- [ ] **P3-07 — Add resumable runs.** Restore checkpoints after restart and distinguish completed from uncertain tool actions. Done when recovery does not blindly repeat writes or external side effects.
- [ ] **P3-08 — Add optional task routing.** Start with explicit selection and deterministic matching; permit a small router later if evaluation justifies it. Done when ambiguity requests a choice and routing cannot override tool grants.
- [ ] **P3-09 — Complete the tool plugin SDK.** Use a versioned subprocess protocol for external tool plugins, with a Go SDK and validated plugin metadata. Treat plugin executables as trusted code. Done when a sample plugin registers without core edits and model use still requires grants.
- [ ] **P3-10 — Isolate executable tools.** If shell/code tools are enabled, run them in a constrained subprocess/container with bounded resources and explicit mounts. Done when timeout, network, and filesystem restrictions are verified before exposing those tools to bots.
- [ ] **P3-11 — Handle credentials securely.** Resolve opt-in external-tool credentials outside model context and redact sensitive event fields. Done when prompts, logs, exports, and package manifests contain no fixture secrets.
- [ ] **P3-12 — Add observability.** Record run status, durations, tool failures, model/adapter versions, memory, and termination reasons. Done when a failed run can be diagnosed from its local trace without logging secrets.
- [ ] **P3-13 — Add retention and cleanup.** Configure run-history retention and remove unused artifacts with reference checks and an inspectable deletion plan. Done when cleanup preserves active packages, shared weights, and requested run records.
- [ ] **P3-14 — Harden package imports.** Validate archive paths, checksums, size limits, and manifests; never execute package install hooks automatically. Done when malicious or oversized package fixtures fail safely.
- [ ] **P3-15 — Run reliability evaluation.** Cover repeated switching, queue pressure, cancellation, corrupt artifacts, out-of-memory recovery, and service restart. Done when there are no unresolved state leaks or silent wrong-adapter runs.
- [ ] **P3-16 — Run end-to-end acceptance.** Exercise CLI, API, dashboard, package installation, individual bots, and workflows. Done when documented examples produce expected outputs on the target laptop.
- [ ] **P3-17 — Package the local release.** Provide repeatable installation/startup, dependency checks, upgrade notes, and metadata migrations with backups. Done when a fresh install and an upgrade both pass smoke checks.
- [ ] **P3-18 — Publish operating documentation.** Explain extension authoring, adapter compatibility, limitations, hardware measurements, recovery, and evaluation. Done when another developer can add a bot without modifying the framework.
- [ ] **P3-19 — Prepare the final demo and release evidence.** Demonstrate prompt-only bots, adapter-backed bots, safe switching, and a multi-bot workflow. Done when the release includes exact versions, benchmark results, known limitations, and reproducible commands.

**Phase exit:** a fresh installation can add and run a new compatible bot package through CLI/UI without framework edits; workflows recover predictably, permissions hold, and laptop performance is documented.

## Deferred scope

These are not prerequisites for the three phases: a custom inference engine, training a base model from scratch, simultaneous GPU workers, a public bot marketplace, arbitrary cross-base adapter loading, reinforcement learning, or claims of general autonomous intelligence.
