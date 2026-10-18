# Registry

```text
register(directory):
  bot = load_bot(directory / "bot.json")
  create_exclusive(registry / safe_id(bot.id), canonical_json(bot))
get(id): return load_bot(registry / safe_id(id))
list(): return sort_by_id(map(get, installed_ids))
remove(id): delete_definition_only(safe_id(id))
```

Register directory/bot.json -> validate and canonicalize references -> write exclusive ID.json in local registry.
IDs never become unchecked paths. Existing IDs require an explicit upgrade. Upgrade serializes with an OS lifecycle lock, preserves immutable version snapshots and atomically replaces the registration; rollback revalidates a retained version. List sorted definitions, inspect/revalidate on reads, remove definition only; run history remains.

REINSTALL: preserve an existing immutable version snapshot when package relocation changes paths
UPGRADE: reject a conflicting target-version snapshot; keep atomic replacement and rollback validation
