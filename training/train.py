"""Reproducible local QLoRA smoke/task training, separate from the Go runtime."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import platform
import random
import re
import time


def sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("config", type=Path)
    args = parser.parse_args()
    cfg = json.loads(args.config.read_text())
    root = args.config.resolve().parent
    path = lambda key: (root / cfg[key]).resolve()
    base = json.loads(path("base_manifest").read_text())
    if not re.fullmatch(r"[a-f0-9]{40}", base["revision"]):
        raise ValueError("immutable base revision required")
    source = json.loads(path("source_lock").read_text())
    if source["sha"] != base["revision"] or source["id"] != base["repository"]:
        raise ValueError("source provenance mismatch")
    for item in source["siblings"]:
        name = item["rfilename"]
        if name.endswith(".safetensors"):
            if sha(path("source_dir") / name) != item["lfs"]["sha256"]:
                raise ValueError("source weight checksum mismatch: " + name)
    for item in source["siblings"]:
        local = path("source_dir") / item["rfilename"]
        if local.is_file() and "lfs" not in item:
            data = local.read_bytes()
            blob = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
            if blob != item["blobId"]:
                raise ValueError("source metadata changed: " + item["rfilename"])
    if sha(path("source_dir") / "tokenizer.json") != base["tokenizer_sha256"]:
        raise ValueError("tokenizer checksum mismatch")
    lock = json.loads((path("data_dir") / "data.lock.json").read_text())
    for split in ["train", "validation", "test"]:
        if sha(path("data_dir") / (split + ".jsonl")) != lock["split_sha256"][split]:
            raise ValueError("validated dataset changed: " + split)
    os.environ.setdefault("CUBLAS_WORKSPACE_CONFIG", ":4096:8")
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig, set_seed
    from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training
    if not torch.cuda.is_available():
        raise RuntimeError("CUDA GPU required for the NF4 smoke run")
    set_seed(cfg["seed"])
    torch.use_deterministic_algorithms(True, warn_only=True)
    torch.cuda.reset_peak_memory_stats()
    tokenizer = AutoTokenizer.from_pretrained(path("source_dir"), local_files_only=True)
    quant = BitsAndBytesConfig(load_in_4bit=True, bnb_4bit_quant_type="nf4",
                              bnb_4bit_use_double_quant=True, bnb_4bit_compute_dtype=torch.bfloat16)
    model = AutoModelForCausalLM.from_pretrained(path("source_dir"), local_files_only=True,
        quantization_config=quant, device_map={"": 0}, dtype=torch.bfloat16,
        attn_implementation="sdpa")
    model.config.use_cache = False
    model = prepare_model_for_kbit_training(model, use_gradient_checkpointing=True)
    model = get_peft_model(model, LoraConfig(r=cfg["rank"], lora_alpha=cfg["alpha"],
        lora_dropout=cfg["dropout"], target_modules=cfg["target_modules"],
        bias="none", task_type="CAUSAL_LM"))

    def records(split):
        result = []
        for line in (path("data_dir") / (split + ".jsonl")).read_text().splitlines():
            record = json.loads(line)
            messages = record["messages"]
            ids = tokenizer.apply_chat_template(messages, tokenize=True, enable_thinking=False)
            prefix = tokenizer.apply_chat_template(messages[:-1], tokenize=True,
                add_generation_prompt=True, enable_thinking=False)
            if ids[:len(prefix)] != prefix:
                raise ValueError("assistant mask does not match pinned chat template")
            if len(ids) > cfg["sequence_length"] or len(ids) <= len(prefix):
                raise ValueError("record exceeds sequence length or has no target: " + record["id"])
            labels = [-100] * len(prefix) + ids[len(prefix):]
            result.append((torch.tensor([ids], device="cuda"), torch.tensor([labels], device="cuda")))
        if not result:
            raise ValueError("empty " + split + " split")
        return result

    train, validation = records("train"), records("validation")
    optimizer = torch.optim.AdamW((p for p in model.parameters() if p.requires_grad), lr=cfg["learning_rate"])
    losses = []
    started = time.time()
    model.train()
    generator = random.Random(cfg["seed"])
    for step in range(cfg["steps"]):
        optimizer.zero_grad(set_to_none=True)
        ids, labels = train[generator.randrange(len(train))]
        loss = model(input_ids=ids, labels=labels).loss
        if not torch.isfinite(loss):
            raise RuntimeError("non-finite loss")
        loss.backward()
        torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
        optimizer.step()
        losses.append(loss.item())
        print(json.dumps({"step": step + 1, "loss": losses[-1]}), flush=True)
    model.eval()
    with torch.no_grad():
        validation_loss = sum(model(input_ids=x, labels=y).loss.item() for x, y in validation) / len(validation)
    out = path("output")
    out.mkdir(parents=True, exist_ok=False)
    model.save_pretrained(out, safe_serialization=True)
    tokenizer.save_pretrained(out)
    evidence = {"config": cfg, "base": base, "data_lock": lock, "source_revision": source["sha"],
        "losses": losses, "validation_loss": validation_loss, "seconds": time.time() - started,
        "peak_cuda_allocated_bytes": torch.cuda.max_memory_allocated(),
        "peak_cuda_reserved_bytes": torch.cuda.max_memory_reserved(),
        "python": platform.python_version(), "platform": platform.platform(),
        "cuda": torch.version.cuda, "gpu": torch.cuda.get_device_name(),
        "packages": {p: importlib.metadata.version(p) for p in ["torch", "transformers", "peft", "accelerate", "bitsandbytes"]},
        "environment": sorted(str(d.metadata["Name"]) + "==" + d.version for d in importlib.metadata.distributions())}
    (out / "training.json").write_text(json.dumps(evidence, indent=2))
    print(json.dumps({"output": str(out), "validation_loss": validation_loss}), flush=True)


if __name__ == "__main__":
    main()
