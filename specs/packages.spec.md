# Baseline archive imports

Prompt-only ZIP contract: package.json schema_version 1 + ID/version + files map(path -> lowercase SHA256); bot.json plus every regular payload file must be listed. No install hooks.
Read bounded ZIP -> <=128 entries, <=16 MiB compressed/uncompressed, <=1 MiB/file -> reject traversal, absolute/drive paths, backslashes, Windows reserved names, trailing dots/spaces, case-insensitive duplicates, symlinks/special files -> verify ZIP CRC and manifest SHA256.
Strict bot definition -> identity matches manifest -> roots/evaluation references are package-relative -> extract into unique managed home directory -> normal Catalog install validates existing roots/grants/base -> register; failure removes only newly extracted directory.
CLI bot import and HTTP packages archive share this path. Trained adapters, upgrades/rollback/export and reference-aware artifact lifecycle still require Phase 2.
