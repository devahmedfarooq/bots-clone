# Phase 1 model decision

Candidate: official **Qwen3-1.7B Q8_0 GGUF**, served by **llama.cpp b11669** with its Windows Vulkan backend. [Artifact lock](../artifacts/baseline.lock.json) records exact revisions, filenames, size and upstream SHA-256 digests.

Reasons: an official quantized artifact, small enough to test on the measured 8 GB GPU, one general model for both tasks, a native Qwen chat template with tool support, and a documented future GGUF LoRA deployment route. Q8_0 trades disk/weight memory for less quantization loss than more aggressive compression; this baseline does not claim it is the optimal quantization.

The [official model card](https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/blob/90862c4b9d2787eaed51d12237eafdfe7c5f6077/README.md) declares Apache-2.0. [llama.cpp](https://github.com/ggml-org/llama.cpp/tree/b11669) is MIT licensed. Keep those upstream licenses with redistributed artifacts. The Go dependencies retain their own licenses in the module cache.

Use `--jinja` and disable thinking in each chat-template request. Both bots use the same immutable artifact and model alias; all specialization in Phase 1 is through instructions and grants. Vulkan avoids requiring a separately installed CUDA toolkit. Actual device selection, loading, native tool-call behavior and fixture quality must be established by the checked-in live evaluation, not inferred from the model family.

Adapter deployment path for Phase 2: pin the exact original Qwen base/tokenizer revision used for training, validate architecture and target modules, convert compatible PEFT LoRA with llama.cpp's `convert_lora_to_gguf.py`, and load the resulting GGUF adapter using the engine's LoRA support. Compare original and deployed fixtures before promotion. The source safetensors revision behind this acquired GGUF is not asserted by this project; establish it or rebuild a base GGUF from a pinned source before declaring any adapter compatible. Phase 1 rejects all adapter references. Conversion and adapter equivalence are intentionally Phase 2 acceptance work.

References: [native tool calling](https://github.com/ggml-org/llama.cpp/blob/b11669/docs/function-calling.md), [server and LoRA options](https://github.com/ggml-org/llama.cpp/blob/b11669/tools/server/README.md), [LoRA converter](https://github.com/ggml-org/llama.cpp/blob/b11669/convert_lora_to_gguf.py).

The initial and refined three-context baselines passed 9/12 cases. The search task selected a different file after finding a relevant match. Clearer instructions and explicit path/line/text search records did not resolve that behavior: the latest workspace bot (1.0.2) passed 3/4 cases at context 2048. Existing evidence is preserved in `evaluation/initial-baseline.json`, `evaluation/baseline.json` and `evaluation/refinement.json`. These are development fixtures, not held-out quality comparisons.

The user explicitly deferred this prompt-only search/read quality issue to adapter-backed evaluation. Phase 1 retains the failed expectation and nonzero evaluation exit code. JSON definitions remain configuration/metadata; intended bot specialization comes from QLoRA-trained adapters in Phase 2. A QLoRA adapter is not assumed to fix the failure until it passes the same fixture.
