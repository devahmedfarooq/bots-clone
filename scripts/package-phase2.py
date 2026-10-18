"""Build versioned task packages from converted adapters without bundling weights."""
import hashlib
import json
from pathlib import Path
import shutil
import zipfile

root = Path(__file__).resolve().parent.parent
base = json.loads((root / "artifacts/phase2-base.json").read_text())

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

for task in ["extract", "workspace"]:
    name = "qlora-" + task
    out = root / "examples" / name
    out.mkdir(exist_ok=True)
    training = json.loads((root / "models/adapters" / task / "training.json").read_text())
    artifact = root / "models/adapters" / (task + ".gguf")
    original = root / "bots" / ("extract" if task == "extract" else "workspace-qa")
    bot = json.loads((original / "bot.json").read_text())
    bot.update({"id": name, "version": "0.1.0", "adapter": "adapter.json",
                "description": "Experimental synthetic QLoRA " + task + " specialization",
                "evaluation_suite": "fixtures.json"})
    if task == "workspace":
        # Preserve baseline tool fixtures while making context roots portable.
        roots = []
        for index, source in enumerate(bot["context_sources"]):
            dest = out / "context" / str(index)
            if dest.exists():
                raise ValueError("package output already contains context; use a fresh directory")
            shutil.copytree((original / source).resolve(), dest)
            roots.append("context/" + str(index))
        bot["context_sources"] = roots
        (out / roots[0] / "deployment.txt").write_text("Helios deployment is approved by Leena. Deployment happens Tuesday at 09:30.\n")
        (out / roots[0] / "snapshots.txt").write_text("Snapshot backups run Sunday at 07:00 and are retained for six weeks.\n")
    fixture_path = (original / json.loads((original / "bot.json").read_text())["evaluation_suite"]).resolve()
    fixtures = json.loads(fixture_path.read_text()) + json.loads((root / "training/data" / (task + "-fixtures.json")).read_text())
    (out / "bot.json").write_text(json.dumps(bot, indent=2) + "\n")
    (out / "fixtures.json").write_text(json.dumps(fixtures, indent=2) + "\n")
    # Text-only fixtures serve reference/deployed conversion checks before tool comparison.
    reference = [fixture for fixture in fixtures if not fixture.get("min_tool_calls", 0)]
    (out / "reference-fixtures.json").write_text(json.dumps(reference, indent=2) + "\n")
    adapter = {"schema_version": 1, "id": name, "version": "0.1.0", "base": base,
        "runtime": "llama.cpp", "runtime_revision": "b11669",
        "runtime_sha256": sha(root / "runtime/llama/llama-server.exe"), "format": "gguf-lora",
        "conversion": "llama.cpp/b11669/convert_lora_to_gguf.py", "source": "local:synthetic-phase2-v1/" + task,
        "license": "Apache-2.0", "sha256": sha(artifact), "artifact": "sha256:" + sha(artifact),
        "target_modules": training["config"]["target_modules"], "training_method": "qlora"}
    (out / "adapter.json").write_text(json.dumps(adapter, indent=2) + "\n")
    (out / "training.json").write_text(json.dumps(training, indent=2) + "\n")
    shutil.copyfile(root / "models/qwen3-source/LICENSE", out / "LICENSE")
    files = {p.relative_to(out).as_posix(): sha(p) for p in out.rglob("*") if p.is_file() and p.name != "package.json"}
    manifest = {"schema_version": 1, "id": name, "version": "0.1.0", "files": files}
    (out / "package.json").write_text(json.dumps(manifest, indent=2) + "\n")
    archive = root / ".local" / (name + ".zip")
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as target:
        for p in out.rglob("*"):
            if p.is_file():
                target.write(p, p.relative_to(out).as_posix())
    print(json.dumps({"archive": str(archive), "artifact": str(artifact), "sha256": adapter["sha256"]}))
