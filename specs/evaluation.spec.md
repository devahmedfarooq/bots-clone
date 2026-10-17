# Evaluation

```text
evaluate(bots, context_sizes):
  start_bounded_memory_sampler()
  for size in context_sizes:
    for bot in bots:
      for fixture in strict_load(bot.evaluation_suite):
        run = shared_runner(bot, fixture.input, context=size)
        check(expected_JSON, answer_terms, successful_tool_count)
        record(run, pass, latency, tokens)
  stop_sampler(); save_samples_peaks_hardware_and_process_starts()
```

For each explicit context size and registered baseline bot, load strict fixtures -> run through normal Runner -> check exact JSON or required answer terms AND required successful tool-event count -> save run, latency and usage. Changing context reloads the engine; bots at the same size reuse one process. Record actual process starts.
Monitor device-wide VRAM and system-wide used RAM every second (bounded hardware command); record samples/peaks plus exact hardware. Report sample errors, synthetic-fixture limitations and cold-load behavior.
Do not claim process-isolated memory, long-context quality, or adapter improvement. Context-size sweeps measure allocations and fixture behavior; long-input sweeps are separately identified in evidence.
