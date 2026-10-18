# Trusted tool plugins

Administrator opts in through framework.plugins manifest paths -> strict bounded manifest -> protocol 1 + existing executable + flat primitive object schema -> register alongside scoped file tools.
Bot must grant plugin name; registry checks grant and validates required/unknown/type constraints BEFORE subprocess.
Call -> new executable process (no shell) -> protocol-versioned bounded JSON stdin/stdout -> context timeout + bounded wait -> structured result/error; stderr discarded and inherited environment minimized.
Go authors use public plugin.Serve(in,out,callback); sample cmd/plugin-echo builds without core edits.
Executables are trusted code with host account access, not a sandbox. No shell/code execution tool is provided. Packages cannot add plugins or install hooks. Network/mount sandboxing must be implemented before offering arbitrary shell/code tools.
