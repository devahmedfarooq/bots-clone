# Diagnostics

Collect OS/architecture/threads, bounded nvidia-smi GPU/VRAM/driver report, installed executables.
Windows: query physical RAM and current-volume free disk through kernel32 APIs, avoiding WMI permissions.
Report missing probes explicitly; never invent hardware capacities. Non-Windows fallback reports proc memory and disk inspection command.
