# Inference and residency

Provider: load(model), count(template tokens), generate(optional text callback), cancel, health, unload.
Llama provider owns one offline loopback server process, an ephemeral port and a bounded startup/shutdown wait. Model alias must equal requested base. Reuse healthy process; restart failed process. Never log prompts at application level.
Render chat template then tokenize to reserve exact prompt budget; submit OpenAI-compatible tool messages. Reject malformed responses and truncated SSE. Context cancellation closes HTTP generation. Shutdown kills/reaps only the owned process.
Manager: context-aware single inference slot -> load/reuse provider -> caller executes whole tool loop -> release slot. Close waits for slot then permanently unloads. One CLI process owns the GPU; CLI lock prevents competing processes.
