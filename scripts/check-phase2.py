"""Reproduce installed-package, quality, switching and memory checks without training."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def package(task, target, *, identifier=None, version=None, first_fixture=False, adapter=True):
    source = ROOT / "examples" / ("qlora-" + task)
    files = {p.relative_to(source).as_posix(): p.read_bytes() for p in source.rglob("*") if p.is_file()}
    bot = json.loads(files["bot.json"])
    if identifier:
        bot["id"] = identifier
    if version:
        bot["version"] = version
    if not adapter:
        bot.pop("adapter", None)
    if first_fixture:
        files["fixtures.json"] = json.dumps(json.loads(files["fixtures.json"])[:1]).encode()
    files["bot.json"] = json.dumps(bot).encode()
    manifest = {"schema_version": 1, "id": bot["id"], "version": bot["version"],
                "files": {name: hashlib.sha256(data).hexdigest() for name, data in files.items()
                          if name != "package.json"}}
    files["package.json"] = json.dumps(manifest).encode()
    with zipfile.ZipFile(target, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, data in files.items():
            archive.writestr(name, data)
    return bot["id"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cli", type=Path, default=ROOT / "bin/bots-phase2.exe")
    parser.add_argument("--config", type=Path, default=ROOT / "framework.adapters.json")
    parser.add_argument("--output", type=Path, default=ROOT / "evaluation/phase2-recheck")
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="phase2-check-", dir=ROOT / ".local"))
    base = json.loads(args.config.read_text())
    for key in ["binary", "model_path", "base_manifest"]:
        base["engine"][key] = str((args.config.resolve().parent / base["engine"][key]).resolve())
    base["plugins"] = [str((args.config.resolve().parent / p).resolve()) for p in base.get("plugins", [])]

    def config(name, cache="f16"):
        value = dict(base)
        value["engine"] = dict(base["engine"], cache_type=cache)
        value["home"] = str(work / name)
        path = work / (name + "-" + cache + ".json")
        path.write_text(json.dumps(value))
        return path

    def cli(cfg, *command, require=True):
        result = subprocess.run([str(args.cli.resolve()), "-config", str(cfg), *map(str, command)],
                                cwd=ROOT, capture_output=True, text=True)
        if require and result.returncode:
            raise RuntimeError(result.stderr)
        return result.returncode

    installed = config("installed")
    for task in ["extract", "workspace"]:
        archive = work / (task + ".zip")
        package(task, archive)
        manifest = json.loads((ROOT / "examples" / ("qlora-" + task) / "adapter.json").read_text())
        cli(installed, "artifact", "import", ROOT / "models/adapters" / (task + ".gguf"), manifest["sha256"])
        cli(installed, "package", "install", archive)
    upgrade = work / "upgrade.zip"
    package("extract", upgrade, version="0.1.1")
    cli(installed, "package", "upgrade", upgrade)
    cli(installed, "package", "rollback", "qlora-extract", "0.1.0")
    exported = work / "export.zip"
    cli(installed, "package", "export", "qlora-extract", exported)
    cli(installed, "package", "uninstall", "qlora-extract")
    cli(installed, "package", "install", exported)
    print(json.dumps({"work": str(work), "config": str(installed), "package_lifecycle": "passed"}), flush=True)
    if args.prepare_only:
        return
    comparison_exit = cli(installed, "compare", "-bots", "qlora-extract,qlora-workspace", "-contexts", "2048",
                          "-output", output / "comparison.json", require=False)
    cli(installed, "compare", "-bots", "qlora-extract,qlora-workspace", "-switch-cycles", "3",
        "-output", output / "switching.json")
    memory = config("memory")
    control = config("control")
    for task in ["extract", "workspace"]:
        archive = work / ("memory-" + task + ".zip")
        package(task, archive, identifier="memory-qlora-" + task, first_fixture=True)
        manifest = json.loads((ROOT / "examples" / ("qlora-" + task) / "adapter.json").read_text())
        cli(memory, "artifact", "import", ROOT / "models/adapters" / (task + ".gguf"), manifest["sha256"])
        cli(memory, "package", "install", archive)
    archive = work / "control.zip"
    package("extract", archive, identifier="memory-base", first_fixture=True, adapter=False)
    cli(control, "package", "install", archive)
    exits = {}
    for name, cfg in [("base-f16", control), ("adapters-f16", memory),
                      ("adapters-q8", config("memory", "q8_0"))]:
        exits[name] = cli(cfg, "eval", "-contexts", "4096", "-output", output / (name + ".json"), require=False)
    (output / "memory-exits.json").write_text(json.dumps(exits, indent=2))
    print(json.dumps({"output": str(output), "comparison_exit": comparison_exit, "memory_exits": exits}), flush=True)
    raise SystemExit(1 if comparison_exit or any(exits.values()) else 0)


if __name__ == "__main__":
    main()
