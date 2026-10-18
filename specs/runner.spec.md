# Run loop

```text
run(bot, input, options):
  validate(); create_owned_session_and_running_record(); start_deadline()
  acquire_model_slot_and_exact_adapter(); reload_completed_session_history()
  messages = instructions + owned_history + input + optional_untrusted_retrieval
  loop:
    reserve_prompt_context_and_total_tokens(); generate_with_bounded_retry()
    validate_and_record_response()
    if final_stop: complete()
    for call in response.calls: enforce_budget(); execute_granted_tool(); append_result()
  finally: persist_terminal_state(); release_model_slot_and_roots()
```

Validate bot/settings/input -> open confined tools -> validate grants -> create/resume owned session -> persist running record -> acquire single model slot (queue time counts toward deadline).
Build instructions + that session's completed history + input. For each turn: render/count prompt -> reserve remaining total/context tokens -> generate with bounded retry (one retry, no retry after emitted streaming text) -> validate envelope/usage -> persist messages.
Final stop completes. Fixed CLI/streaming runs still fail on length. Managed non-streaming runs grow the reply allowance, regenerate only the unfinished model response, and continue text in chunks at the context ceiling. Incomplete tool arguments are never executed. Tool calls require valid unique IDs; each counts toward budget, passes external grant checks and persists structured result before next generation. No bot-role branching.
Any error/cancellation/deadline -> persist terminal reason and duration -> release slot/roots. Failed-run messages remain auditable but are excluded from resumable history.

Before generation, reserve configured completion and tool headroom. Remove oldest complete prior turns; compact tool results without losing call pairing; omit oversized retrieval explicitly. Never truncate instructions/current user task. See context.spec.md.

MANAGED_OUTPUT:
  package max_tokens = initial hint, not permanent output ceiling
  manager may set task.output_tokens (0 = automatic); fit can reduce allocation for a large prompt
  before generation: ask durable manager allocator for prompt + reply + already spent tokens
  length -> increase allowance and retry this generation only; account all discarded tokens
  hardware ceiling -> retain plain text segment and request remainder with bounded UTF-8 tail
  assemble final answer; persist one complete assistant message; never mark partial output complete
  preserve completed tool messages; never replay tool execution during model retries
  stop on cancellation, deadline, exhausted shared goal budget, or invalid/incomplete tool response
  record generation_allocation, output_reallocated and output_continuation events
