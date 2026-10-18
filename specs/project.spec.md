# Project scope

Phase 1: declarative prompt bots -> one resident local model -> permission-checked tools -> isolated SQLite runs -> measured baseline.
Phase 2: QLoRA-trained bot specialization -> exact adapter compatibility -> package lifecycle -> safe switching -> held-out specialization comparisons. JSON definitions remain metadata/configuration, not the final source of specialization. QLoRA describes training against a quantized base; the deployed artifact is a LoRA adapter.
Phase 3: shared service API/dashboard -> workflows/recovery -> local release.

User-requested order (2026-10-18): implement Phase 3 on development before Phase 2. Prompt-only service/product contracts are implemented and tested; adapter-dependent acceptance remains pending. Each Phase 3 ticket has a feature branch fast-forwarded into development; main retains the approved Phase 1 baseline.

Each ticket starts a feature branch from the preceding ticket. Module specifications describe the implemented contracts; todo.md records acceptance evidence.

User-approved deferral: prompt-only search -> read quality remains 0/3 in the three-context baseline (overall 9/12). Preserve that fixture for adapter evaluation; do not keep optimizing role prompts or assume training fixes it.
