# Phase 1 acceptance — 2026-10-18

Framework acceptance is complete. The user explicitly deferred the prompt-only search -> read quality failure to QLoRA adapter evaluation. Failed expectations remain intact; evaluation still returns exit code 1 for them.

## Recorded live runs

| Context | Fixture pass rate | Cold first case | Warm case durations | Total tokens | Peak sampled GPU memory | Peak sampled system RAM |
| --- | --- | --- | --- | --- | --- | --- |
| 1024 | 3/4 | 4.966 s | 0.308 / 1.597 / 1.246 s | 2423 | 3079 MiB | 13.67 GiB |
| 2048 | 3/4 | 4.171 s | 0.345 / 0.714 / 1.274 s | 2423 | 3221 MiB | 13.77 GiB |
| 4096 | 3/4 | 4.343 s | 0.333 / 0.729 / 1.270 s | 2423 | 3432 MiB | 13.84 GiB |

Source: [baseline.json](baseline.json), using workspace bot 1.0.1 and extraction bot 1.0.0. Twelve runs started exactly three model processes: one per context size, reused across both bots and all four fixtures at that size. Context changes unload before reloading.

- Extraction: 6/6; direct workspace reads: 3/3; search followed by reading the matching file: 0/3.
- The initial run is retained in [initial-baseline.json](initial-baseline.json). First-ever initialization took 22.602 s for its first case and sampled up to 15.54 GiB system RAM, including other applications.
- The final code uses structured path/line/text search records and workspace bot 1.0.2. Its focused context-2048 run remains 3/4: [refinement.json](refinement.json). Further prompt optimization was stopped at the user's request.
- Memory is device-wide/system-wide, sampled once per second; it includes other applications and is not isolated model memory. Short synthetic fixtures do not establish long-input quality or maximum hardware limits. These development fixtures are not a held-out evaluation.

## Laptop defaults and reproducibility

[Hardware inventory](hardware.json): RTX 5050 Laptop GPU, 8151 MiB VRAM, driver 610.74, 16 logical CPU threads, 16,873,545,728 usable physical RAM bytes. The pinned engine detects the RTX as Vulkan0. Exact engine/model versions and verified hashes are in [baseline.lock.json](../artifacts/baseline.lock.json).

Keep the tested 4096-token context ceiling/default, one active model, Vulkan0, temperature 0 and at most 512 completion tokens per generation (extraction uses 256). Larger contexts remain unmeasured. Allow up to two minutes for startup; first initialization can use substantially more RAM than warm operation. Keep sufficient RAM free from other applications before loading.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1
.\bin\bots.exe bot register bots/extract
.\bin\bots.exe bot register bots/workspace-qa
.\bin\bots.exe eval -contexts 1024,2048,4096 -output evaluation/baseline.json
```

Those registrations assume a fresh registry. Current bot 1.0.2 differs from the preserved three-context run; its focused result is recorded separately. The known quality failure is expected to produce a nonzero evaluation exit, and is not hidden as a successful test.

Verified after implementation:

- `scripts/setup.ps1 -Offline`: pinned artifact hashes, cached Go dependencies, all contract tests, CLI rebuild and diagnostics pass with Go network downloads disabled.
- `go vet ./...`: passes.
- Tests cover bot validation/registration, settings, SQLite persistence and bot/version ownership, denied/malformed tool calls, time/output/token budgets, Windows junction/traversal escapes, HTTP/SSE parsing, cancellation, queueing and recovery.

## Deferred quality work

QLoRA-trained adapters will provide specialization; JSON remains package metadata and configuration. Reuse the failed search/read fixture during Phase 2 and add held-out variants. Do not assume an adapter fixes path selection without measuring it. See [model decision](../docs/model-decision.md).
