# Specialization data and training

VALIDATE(jsonl, evaluation jsonl):
  strict bounded records; IDs/task/group/split and source URI/license/revision required
  explicit synthetic provenance; only system/user/assistant text messages
  reject empty content, malformed expected JSON/schema, credentials/private keys
  hash normalized user input: reject duplicate/conflicting examples and evaluation overlap
  group cannot cross train/validation/test; evaluation fixtures excluded by input hash
  emit line-numbered rejection reasons; fail closed and do not export partial data
EXPORT: clean messages by split + SHA256 data lock; train consumes exact lock digest
TRAIN: separate Python environment; exact immutable base/tokenizer revisions and data hash
  seed; NF4 4-bit double quantization; pinned LoRA modules/rank/alpha/dropout
  assistant-only labels, bounded length, held-out validation, CUDA memory metrics
  save PEFT + provenance/environment; convert with pinned llama.cpp; package experimental
PROMOTE: requires same fixtures on PEFT reference and deployed GGUF, then held-out three-arm comparison
  never mark smoke training or synthetic contract fixtures as measured specialization
