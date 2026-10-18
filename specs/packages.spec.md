# Portable bot packages

Prompt-only ZIP contract: package.json schema_version 1 + ID/version + files map(path -> lowercase SHA256); bot.json plus every regular payload file must be listed. No install hooks.
Read bounded ZIP -> <=128 entries, <=16 MiB compressed/uncompressed, <=1 MiB/file -> reject traversal, absolute/drive paths, backslashes, Windows reserved names, trailing dots/spaces, case-insensitive duplicates, symlinks/special files -> verify ZIP CRC and manifest SHA256.
Strict bot definition -> identity matches manifest -> roots/evaluation references are package-relative -> extract into unique managed home directory -> normal Catalog install validates existing roots/grants/base -> register; failure removes only newly extracted directory.
CLI bot import and HTTP packages archive share this path. Adapter archives carry a strict adapter manifest with sha256:<digest> artifact references; weights are imported separately into content-addressed storage. Never resolve arbitrary host artifact paths from ZIP content.

INSTALL: extract bounded ZIP -> Catalog validation -> materialize verified local adapter manifest -> exclusive register
UPGRADE: same prepare path -> lifecycle lock -> preserve current/new immutable version snapshots -> atomic registry replace
ROLLBACK: load selected retained version -> current compatibility/tool checks -> same atomic replace
EXPORT: bounded bot snapshot + confined context/evaluation + digest-only adapter manifest -> checksummed ZIP
UNINSTALL: remove registration; retain histories, rollback snapshots and all shared weights
No package hooks or implicit dependency downloads. New adapters require existing pinned runtime/base identity.
