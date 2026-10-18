# Authoring local bot packages

A package contains bot.json, optional adapter.json, prompt/context files and fixtures.json. package.json uses schema_version 1, matching ID/version, and a SHA256 files map covering every payload. ZIP imports are bounded and never run hooks. Instructions are the prompt template; context_sources are granted directories, while optional retrieval entries select files/directories by root index and relative path. Retrieval needs file-read. Use module specs for the contracts.

## Prompt-only template

Copy examples/package-template; choose a new ID/version, task instructions, grants and evaluation expectations. Run:

    bin/bots.exe bot validate examples/package-template
    bin/bots.exe bot register examples/package-template
    bin/bots.exe run package-template -input "Reply with only the word READY."
    bin/bots.exe package export package-template .local/package-template.zip
    bin/bots.exe package uninstall package-template
    bin/bots.exe package install .local/package-template.zip

Export captures bounded context and evaluation files. Reinstalling the ZIP requires no framework source edits. Evaluation uses the normal eval command; keep a separate framework home for authoring so unrelated bot suites do not affect results.

## Adapter-backed packages

First establish exact base provenance. The Phase 1 acquired GGUF does not establish its original source revision. Use a locally rebuilt GGUF from pinned source, then record repository/revision, architecture, tokenizer repository/revision, tokenizer.json SHA256, GGUF SHA256 and quantization in engine.base_manifest. Set engine.runtime_revision to the pinned llama.cpp tag. An adapter also records runtime_sha256 for the actual executable; both the catalog and provider check it.

Use training/README.md to validate task data, train QLoRA separately, and convert using the pinned converter. adapter.json requires schema_version, id/version, the full base identity, runtime=llama.cpp, runtime_revision, runtime_sha256, format=gguf-lora, conversion=llama.cpp/REVISION/convert_lora_to_gguf.py, source, license, sha256, artifact, target_modules and training_method=qlora. Unsupported PEFT/DoRA/modules are rejected before generation. Keep training and conversion provenance beside the package.

Portable adapter.json uses artifact=sha256:DIGEST, with the same digest in sha256. Set bot.json adapter=adapter.json. Import weights separately:

    bin/bots.exe artifact import models/adapters/task.gguf DIGEST
    bin/bots.exe package install TASK.zip
    bin/bots.exe package upgrade TASK_NEW_VERSION.zip
    bin/bots.exe package rollback TASK_ID 1.0.0
    bin/bots.exe package export TASK_ID .local/task-export.zip
    bin/bots.exe package uninstall TASK_ID

Upgrade requires a different version and preserves immutable rollback snapshots. Rollback rechecks compatibility. Uninstall preserves histories and shared weights; no automatic artifact deletion occurs. Transfer referenced GGUF artifacts separately when moving a package to another laptop.

The installed adapter pool preloads at scale zero. A/B/base switching holds the single inference slot, erases cache and verifies engine scales. Adding a new adapter while running restarts the owned engine serially; ordinary switches within the loaded pool reuse the base. Failed activation returns an error and recovers base or unloads the process. Models and adapters remain local; inference works offline after artifacts/dependencies are present.

## Verify specialization

    bin/bots.exe compare -bots EXTRACT_ID,WORKSPACE_ID -contexts 2048,4096 -output evaluation/phase2-comparison.json
    bin/bots.exe compare -bots EXTRACT_ID,WORKSPACE_ID -switch-cycles 3 -output evaluation/phase2-switching.json

Compare reference PEFT and deployed GGUF on identical text fixtures first. Publish all three quality arms with latency/memory, including failures. Include original search/read fixtures and held-out variants. A small synthetic smoke adapter remains experimental, including when it offers no measured benefit. CPU-offload/context/KV-cache limits need measured laptop evidence; engine.cache_type supports f16 or q8_0.
