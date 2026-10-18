# Reproducible local setup

```text
setup(offline):
  lock = read_pinned_artifact_metadata()
  for artifact in lock: require_or_download(artifact); verify_SHA256(artifact)
  expand_engine(); configure_project_local_Go_cache()
  if offline: disable_proxy_checksum_and_toolchain_downloads()
  install_pinned_modules(); test(); build(); run_diagnostics()
```

Read artifact lock -> download missing binaries/model only when online -> verify pinned SHA-256 -> expand engine -> download pinned Go modules -> tests -> CLI build -> diagnostics.
Offline mode disables Go proxy/sum DB/toolchain downloads and requires cached modules plus local artifacts. Runtime always starts engine offline, uses local loopback API, and has no Python dependency.
Store weights, binaries, build/module cache and SQLite outside Git through ignore rules. Persist evaluation JSON and decisions in Git.

REGISTER_STUDIO_BOTS (scripts/register-studio-bots.ps1): for each id in [manager, writer, landing-page, code-writer, math-solver, qa-assistant] or -Bots; skip when bot list already has it; else bot register bots/<id>; fail on first registration error; safe to re-run
