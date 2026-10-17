# Run loop

```text
run(bot, input, options):
  validate(); create_owned_session_and_running_record(); start_deadline()
  acquire_model_slot(); reload_completed_session_history()
  messages = instructions + owned_history + input
  loop:
    reserve_prompt_context_and_total_tokens(); generate_with_bounded_retry()
    validate_and_record_response()
    if final_stop: complete()
    for call in response.calls: enforce_budget(); execute_granted_tool(); append_result()
  finally: persist_terminal_state(); release_model_slot_and_roots()
```

Validate bot/settings/input -> open confined tools -> validate grants -> create/resume owned session -> persist running record -> acquire single model slot (queue time counts toward deadline).
Build instructions + that session's completed history + input. For each turn: render/count prompt -> reserve remaining total/context tokens -> generate with bounded retry (one retry, no retry after emitted streaming text) -> validate envelope/usage -> persist messages.
Final stop completes. Length output fails explicitly. Tool calls require valid unique IDs; each counts toward budget, passes external grant checks and persists structured result before next generation. No bot-role branching.
Any error/cancellation/deadline -> persist terminal reason and duration -> release slot/roots. Failed-run messages remain auditable but are excluded from resumable history.
