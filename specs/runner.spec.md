# Run loop

Validate bot/settings/input -> open confined tools -> validate grants -> create/resume owned session -> persist running record -> acquire single model slot (queue time counts toward deadline).
Build instructions + that session's completed history + input. For each turn: render/count prompt -> reserve remaining total/context tokens -> generate with bounded retry (one retry, no retry after emitted streaming text) -> validate envelope/usage -> persist messages.
Final stop completes. Length output fails explicitly. Tool calls require valid unique IDs; each counts toward budget, passes external grant checks and persists structured result before next generation. No bot-role branching.
Any error/cancellation/deadline -> persist terminal reason and duration -> release slot/roots. Failed-run messages remain auditable but are excluded from resumable history.
