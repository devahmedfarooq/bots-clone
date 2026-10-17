# Diagnostics

```text
diagnostics():
  collect(OS, architecture, Go_version, CPU_threads)
  GPU = bounded_nvidia_smi(name, total_VRAM, used_VRAM, driver)
  RAM, disk = native_system_queries()
  report_available_paths_and_missing_probes_without_inventing_values()
```

Collect OS/architecture/threads, bounded nvidia-smi GPU/VRAM/driver report, installed executables.
Windows: query physical RAM and current-volume free disk through kernel32 APIs, avoiding WMI permissions.
Report missing probes explicitly; never invent hardware capacities. Non-Windows fallback reports proc memory and disk inspection command.
