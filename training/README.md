# Local QLoRA training

Python is separate from the Go backend. Install Python 3.11 and the pinned requirements in a dedicated environment. CUDA wheels target the laptop GPU; no external GPU service is used.

1. Download original Qwen/Qwen3-1.7B at immutable revision 70d244cc86ccca08cf5af4e1e306ecf908b1ad5e. Save the Hugging Face API revision response with blobs=true as source.lock.json. Verify source safetensors against its LFS digests.
2. Rebuild a Q8_0 GGUF with llama.cpp b11669 convert_hf_to_gguf.py. Write the base identity manifest with exact revisions, rebuilt GGUF SHA256 and tokenizer.json SHA256. Do not assert that the Phase 1 official GGUF has that source revision.
3. Validate task JSONL against independent evaluation JSONL with bots dataset validate DATA EVALUATION NEW_DIRECTORY. Group related templates together. Include representative licensed task examples; synthetic smoke data is not real-world quality evidence.
4. Set base_manifest, source_lock, source_dir, data_dir and output paths relative to the training config; specify seed, sequence_length, rank, alpha, dropout, target_modules, learning_rate, steps. Run python training/train.py CONFIG.
5. Convert the resulting PEFT directory with pinned convert_lora_to_gguf.py --base LOCAL_SOURCE --outtype f16 --outfile ADAPTER.gguf PEFT_DIRECTORY. Create adapter.json using the exact base identity, artifact digest, source/license and training provenance.
6. Import the GGUF by SHA256, package the adapter reference, install, then compare reference PEFT and deployed GGUF on identical fixtures before promotion. Compare base-only, role-prompted base and adapter plus role prompt under equal budgets, including the deferred search/read fixture and held-out variants.

Training uses NF4 double quantization and assistant-only loss. It rejects changed validated data, tokenizer or source weights. It records seed/config, dependency versions, loss, duration and CUDA memory. A smoke run does not prove improvement. Keep task adapters experimental until held-out measurements establish the outcome.

References: [PEFT quantized training](https://huggingface.co/docs/peft/developer_guides/quantization), [pinned converter](https://github.com/ggml-org/llama.cpp/blob/b11669/convert_lora_to_gguf.py).
