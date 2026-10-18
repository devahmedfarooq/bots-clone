"""Write exact base provenance after a verified local GGUF conversion."""
import hashlib
import json
from pathlib import Path
import sys

root = Path(__file__).resolve().parent.parent
source = root / "models/qwen3-source"
lock = json.loads((source / "source.lock.json").read_text())

def sha(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for data in iter(lambda: stream.read(1 << 20), b""):
            h.update(data)
    return h.hexdigest()

for item in lock["siblings"]:
    if item["rfilename"].endswith(".safetensors"):
        if sha(source / item["rfilename"]) != item["lfs"]["sha256"]:
            raise ValueError("source checksum mismatch")
for item in lock["siblings"]:
    local = source / item["rfilename"]
    if local.is_file() and "lfs" not in item:
        data = local.read_bytes()
        blob = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
        if blob != item["blobId"]:
            raise ValueError("source metadata changed: " + item["rfilename"])
base = {"schema_version": 1, "repository": lock["id"], "revision": lock["sha"],
    "architecture": "qwen3", "tokenizer_repository": lock["id"], "tokenizer_revision": lock["sha"],
    "tokenizer_sha256": sha(source / "tokenizer.json"),
    "sha256": sha(root / "models/Qwen3-1.7B-phase2-Q8_0.gguf"), "quantization": "Q8_0"}
(root / "artifacts/phase2-base.json").write_text(json.dumps(base, indent=2) + "\n")
config = json.loads((root / "framework.json").read_text())
config["home"] = str(root / ".local/phase2")
config["engine"].update({"model_path": str(root / "models/Qwen3-1.7B-phase2-Q8_0.gguf"),
    "binary": str(root / "runtime/llama/llama-server.exe"), "base_manifest": str(root / "artifacts/phase2-base.json"),
    "runtime_revision": "b11669"})
(root / ".local/phase2-framework.json").write_text(json.dumps(config, indent=2))
print(json.dumps(base, indent=2))
