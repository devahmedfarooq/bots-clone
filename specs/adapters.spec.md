# Adapter artifacts and switching

STATUS implementation; live QLoRA acceptance tracked separately in todo.md

VALIDATE(manifest, base, runtime):
  strict versioned manifest; immutable source/license/checksum required
  match exact base repository/revision, tokenizer repository/revision/digest,
    architecture, GGUF base digest, quantization, engine revision and conversion
  reject unsupported modules/features/formats; verify regular GGUF bytes before activation
STORE(source): stream SHA256 -> temporary file -> content-addressed immutable path
  never overwrite a digest; reverify on resolve; weights stay ignored
ACQUIRE(model, adapter): hold existing single slot for entire run
  load base; validate/load adapter; clear slot cache; set exactly one scale or all zero
  verify engine reports expected scales; release only after caller completes
  failure -> disable adapters + erase cache; if recovery uncertain unload owned process
  never execute a request after failed activation; later caller reloads known base
ENGINE: pinned llama.cpp preloads adapter pool at scale zero; switching uses /lora-adapters
  new pool entries or unloading weights require serial process restart; no simultaneous copies
  all generation sends explicit adapter scales and cache_prompt=false
