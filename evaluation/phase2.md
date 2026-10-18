# Phase 2 acceptance - 2026-10-18

Phase 2 is complete on `feature/p2-completion`. Two experimental QLoRA specializations install, upgrade, roll back, export and switch through the public package interface. The separate worktree preserves the active human-usability checkout. Runtime/package and training work were developed on `feature/p2-adapter-runtime-and-packages` and `feature/p2-qlora-training`.

## Models and training

Qwen/Qwen3-1.7B source revision `70d244cc86ccca08cf5af4e1e306ecf908b1ad5e` was converted to Q8_0 using llama.cpp b11669 (`620d2225cc436ab0b001e3200e590e3b286ef9ab`). Exact source, tokenizer, base, engine and adapter hashes are recorded in [summary](phase2-summary.json), [base manifest](../artifacts/phase2-base.json) and package manifests. The original default prompt-only model/configuration remains available.

Both adapters were trained locally on the RTX 5050 (8151 MiB): NF4 double quantization, rank 8, alpha 16, dropout 0, seven projection modules, seed 42, sequence length 512 and 32 steps. Each dataset has 64 training, 8 validation and 8 test records, with no normalized training-input overlap against comparison fixtures. Sources, licenses, split hashes and the pinned environment are recorded. Templates are shared across synthetic splits: these are narrow task experiments, not evidence of broad generalization.

Training loops took 42.896 seconds (extraction) and 39.966 seconds (workspace). Each peaked at 2,595,710,464 allocated / 3,852,468,224 reserved CUDA bytes. Device-wide training peak was approximately 7459 MiB with another service resident. Seed/configuration reproduction is supported; SDPA backward warned of nondeterminism, so bit-identical retraining is not claimed. Each converted F16 GGUF adapter is 17,459,776 bytes. Weights remain ignored local artifacts.

## Quality and reference deployment

[Comparison](phase2-comparison.json) uses the same base, task fixtures, grants and budgets across all three arms at context 2048.

| Arm | Extraction | Workspace | Total | Mean latency | Tokens | Peak device VRAM |
| --- | --- | --- | --- | --- | --- | --- |
| Base only | 0/10 | 2/12 | 2/22 | 4367 ms | 12426 | 6059 MiB |
| Role-prompted base | 2/10 | 1/12 | 3/22 | 3439 ms | 10060 | 6109 MiB |
| Adapter + role | 10/10 | 9/12 | 19/22 | 7385 ms | 9989 | 6084 MiB |

Adapters improve these synthetic fixtures but increase latency. All three workspace search cases still fail: `search-and-read`, `heldout-search-release`, `heldout-search-retention`. The Phase 1 search-to-read quality issue remains unresolved; failing expectations and nonzero evaluation results are retained.

[PEFT reference extraction](phase2-reference-extract.json) scored 8/10 against the same ten text fixtures used for deployed extraction (10/10). The reference omitted PKR totals in `extract-82` and `extract-86`. NF4 reference and Q8_0 deployment use different base quantizations; no numerical equivalence is claimed. Neither failed fixtures nor expectations were changed to conceal errors.

## Switching, packages and safety

[Final switching](phase2-switching.json): three A -> B -> base cycles pass 9/9 with one model process. Warm median acquisition was 6198 ms for extraction, 6374 ms for workspace and 2006 ms for base. This includes artifact verification and acquisition; it is not pure native switch time. The pinned engine requires a single CSV `--lora` argument and an owned slot-save directory for cache erasure. The [initial failed run](phase2-switching-initial.json) remains recorded.

[Package lifecycle](phase2-package-lifecycle.json) verifies upgrade to 0.1.1, rollback to 0.1.0, export/reinstall, a third package-only alias and uninstall preserving shared weights. [Artifact checks](phase2-artifact-safety.json) reject corrupted weights and a tokenizer mismatch. Provider tests cover failed activation/HTTP 500 recovery, state/path verification and base recovery or unload. Actual GPU OOM was not forced; real-OOM reliability remains pending under P3-15.

Exact compatibility checks cover base/tokenizer identity, runtime build/hash, format and artifact checksum. Bounded retrieval stays within granted roots, carries source attribution and remains untrusted input. Context budgeting preserves the current user input and tool-call pairs; escaping cannot create an unbounded compaction loop. Dataset validation rejects overlap, malformed output, missing provenance and recognized secret patterns.

## Laptop limits

| Context 4096 smoke configuration | Result | Peak device VRAM | Peak system RAM used |
| --- | --- | --- | --- |
| Base, F16 KV | 1/1 | 6211 MiB | 14,039,392,256 bytes |
| Two-adapter pool, F16 KV | 2/2 | 6322 MiB | 13,932,666,880 bytes |
| Two-adapter pool, Q8 KV | 2/2 | 6080 MiB | 13,903,814,656 bytes |

Vulkan device 0, `gpu_layers=99`; CPU weight offload was not configured. Samples are device/system-wide and include other applications, not isolated process memory. Context 4096 checks allocation and short tasks, not long-input quality. Full comparison uses 2048; F16 KV remains the conservative default. Q8 KV passed the short smoke and reduced the observed pool peak by 242 MiB. Individual raw memory reports and exit codes accompany this report.

## Reproduction and verification

Follow [training instructions](../training/README.md) to validate data, train, convert and package. Use `framework.adapters.json`; import both weights with their manifest hashes before installation. The prompt-only [template](../examples/package-template) passed its install/export/reinstall/evaluation path: [evidence](phase2-template.json).

```powershell
go build -buildvcs=false -o bin/bots-phase2.exe ./cmd/bots
python scripts/check-phase2.py --prepare-only
python scripts/check-phase2.py
```

The check script creates a private fresh home and preserves the existing service data. `--prepare-only` passed; the full script's constituent lifecycle, quality, switching and memory stages were run and recorded above. A complete quality run intentionally returns failure for the three retained search cases.

Final verification: Go tests, race tests, vet, Python compilation, package checksum checks and Git whitespace checks. Module pseudocode lives in `specs/`; no trained weights are committed. Phase 3's remaining tickets are outside this acceptance.
