# Inference and residency

```text
acquire(context, model):
  wait_for_single_slot_or_cancel()
  if process_is_healthy AND same_model_and_context: reuse()
  else: unload_owned_process(); start_offline_loopback_engine(); await_health()
  return release_slot
count(request): return tokenize(apply_chat_template(request)).length
generate(request): decode_json_or_sse(http_with_cancellation(request))
shutdown(): acquire_slot(); kill_and_reap_owned_process()
```

Provider: load(model ID + context size), count(template tokens), generate(optional text callback), cancel, health, unload.
Llama provider owns one offline loopback server process, an ephemeral port and a bounded startup/shutdown wait. Model alias must equal requested base. Reuse healthy process at the same context size; unload before changing context or recovering a failed process. Never log prompts at application level.
Render chat template then tokenize to reserve exact prompt budget; submit OpenAI-compatible tool messages. Reject malformed responses and truncated SSE. Context cancellation closes HTTP generation. Shutdown kills/reaps only the owned process.
Manager: context-aware single inference slot -> load/reuse provider -> caller executes whole tool loop -> release slot. Close waits for slot then permanently unloads. One CLI process owns the GPU; CLI lock prevents competing processes.

Adapters extend the provider through AdapterProvider (validate/load/activate/deactivate/unload). Manager selects only while holding its single run slot. Pinned installed pool starts at scale zero; generation always supplies explicit scales. Verify reported engine state and erase slot cache on switches; recover to base or unload on failure. Pool additions/unloads restart serially. See adapters.spec.md.

Pinned b11669 requires one comma-separated --lora argument. Enable an owned --slot-save-path for cache erasure; never save/restore bot caches. Verify engine-reported adapter paths and scales. Native tests retain the initial HTTP 501 failure and corrected 9/9 one-process switching result.
