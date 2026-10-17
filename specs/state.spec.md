# Isolated persistent state

```text
session(id, bot, version):
  if no id: insert(random_id(), bot, version)
  else: require(existing.owner == (bot, version))
  return messages JOIN completed_runs WHERE session == id ORDER BY sequence
persist(value): bound_SQL(encode_json(redact_strings(value)))
finish(run): update_status_reason_usage_duration(run)
```

SQLite WAL stores immutable session ownership (bot ID + version), ordered messages, run snapshots and structured tool events. All queries use bound parameters.
New session uses random ID. Resume requires exact bot/version; never query another session's history. Persist status before execution and terminal status even after failure/cancellation.
Known credential assignments, bearer tokens and common secret-key forms are redacted in stored JSON copies; full tool data remains only in run memory. Do not provide unrelated credentials in tasks. No prompt/body debug logging.
