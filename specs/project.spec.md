# Project scope

Phase 1: declarative prompt bots -> one resident local model -> permission-checked tools -> isolated SQLite runs -> measured baseline.
Phase 2: QLoRA-trained bot specialization -> exact adapter compatibility -> package lifecycle -> safe switching -> held-out specialization comparisons. JSON definitions remain metadata/configuration, not the final source of specialization. QLoRA describes training against a quantized base; the deployed artifact is a LoRA adapter.
Phase 3: shared service API/dashboard -> workflows/recovery -> local release.

Each ticket starts a feature branch from the preceding ticket. Module specifications describe the implemented contracts; todo.md records acceptance evidence.

User-approved deferral: prompt-only search -> read quality remains 0/3 in the three-context baseline (overall 9/12). Preserve that fixture for adapter evaluation; do not keep optimizing role prompts or assume training fixes it.
