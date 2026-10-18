# Live Phase 2 acceptance

PREPARE: rebuild quantized base from immutable checksummed original; train two task adapters
  validate/export independent train/validation/test; save training evidence and licenses
  convert pinned PEFT -> GGUF; evaluate reference and deployed on same text fixtures
PACKAGE: import artifacts -> export portable ZIP -> install public package interface
SWITCH: preload installed pool; repeat A -> B -> base with independent sessions
  compare repeated outputs, model process starts, elapsed latency, sampled VRAM/RAM
  corrupt-copy artifact/manifest -> activation fails -> valid base run still succeeds
COMPARE: same fixtures and budgets for base-only, role prompt, adapter + role prompt
  include prior search/read failure and held-out variants; retain failures
MEMORY: context sweep + supported KV precisions; measure adapter bytes and warm switch cost
  record device/system scope of samples, CPU offload, safe headroom and limitations
COMPLETE only when real evidence exists; synthetic protocol fixtures never satisfy training/live tickets

ACCEPTED 2026-10-18: evaluation/phase2.md and phase2-summary.json
  two real adapters; matched comparison 19/22; switching 9/9, one process
  keep three search failures; synthetic-template limits; actual GPU OOM remains P3 work
