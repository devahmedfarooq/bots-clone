# Isolated persistent state

SQLite WAL stores immutable session ownership (bot ID + version), ordered messages, run snapshots and structured tool events. All queries use bound parameters.
New session uses random ID. Resume requires exact bot/version; never query another session's history. Persist status before execution and terminal status even after failure/cancellation.
Known credential assignments, bearer tokens and common secret-key forms are redacted in stored JSON copies; full tool data remains only in run memory. Do not provide unrelated credentials in tasks. No prompt/body debug logging.
