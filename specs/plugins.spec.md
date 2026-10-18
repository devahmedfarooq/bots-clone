# Trusted tool plugins

Administrator opts in through framework.plugins manifest paths -> strict bounded manifest -> protocol 1 + existing executable + flat primitive object schema -> register alongside scoped file tools.
Bot must grant plugin name; registry checks grant and validates required/unknown/type constraints BEFORE subprocess.
Call -> new executable process (no shell) -> protocol-versioned bounded JSON stdin/stdout -> context timeout + bounded wait -> structured result/error; stderr discarded and inherited environment minimized.
Go authors use public plugin.Serve(in,out,callback); sample cmd/plugin-echo builds without core edits.
Executables are trusted code with host account access, not a sandbox. No shell/code execution tool is provided. Packages cannot add plugins or install hooks. Network/mount sandboxing must be implemented before offering arbitrary shell/code tools.
Default registry tests reject shell/code/exec/Python/PowerShell grants and calls. P3-10 constrained arbitrary execution remains deferred; time/output/process cancellation bounds on trusted plugins do not imply filesystem/network isolation.
Credential forwarding is disabled. External tools inherit only SystemRoot/WINDIR/TEMP/TMP, never the full environment. Recognized credential assignments and bearer/key patterns are redacted before tool results enter model context, returned run output, HTTP responses, and persistence. Redaction is pattern-based, not a universal secret detector.
