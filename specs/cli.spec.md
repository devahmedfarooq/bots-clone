# CLI and process ownership

```text
main(args):
  context = cancel_on_interrupt(); config = resolve_local_framework_paths()
  route diagnostics / registry / settings / stored_result to shared_services
  for run_or_eval: acquire_OS_lock(); open_SQLite(); construct_shared_runner()
  interactive: run_each_line_with_same_model_and_owned_session()
  finally: unload_model(); close_store(); release_OS_lock()
  exit 0 on success, 1 on error, 130 on interrupt
```

Global -config resolves framework paths relative to its file. Strict config selects one local model and maximum server context capacity.
diagnostics: JSON hardware and missing dependencies. bot register/list/inspect/validate/remove: declarative package lifecycle. settings: inspect merged generation overrides.
run ID [-input task | stdin | -interactive] [-session ID] [-stream] [-json=false]: shared Runner, default machine-readable result, optional stderr stream, isolated session reuse. run show ID: stored result.
eval: registered fixture suites and explicit context sweep -> JSON evidence; failing cases return nonzero. Ctrl+C returns 130; errors return 1; success/help returns 0.
OS file lock serializes CLI process ownership of the model; it releases automatically on exit/crash. Within a process the Manager queues callers. On normal shutdown always unload/reap the owned engine.
