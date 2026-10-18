# Configuration

```text
load_bot(path):
  bot = strict_json(read(path)); validate_schema_and_bounds(bot)
  bot.sources = canonical_existing_directories(relative_to(path, bot.sources))
  require_existing_fixture_reference(bot)
  return bot
resolve(bot, run): return validate(merge(DEFAULTS, bot.generation, run.overrides))
```

Decode one strict JSON object, reject unknown fields -> validate schema 1, safe bot ID, semantic version, required model/instructions, grants, bounded budgets/settings.
Resolve defaults < bot generation < explicit run overrides. Reject invalid effective settings.
Resolve context directories and evaluation fixture relative to definition, require existing references, canonicalize roots before registration. Adapters remain rejected until Phase 2 compatibility/provider support exists.
Generation token limit is per request; run token budget covers prompt plus completion across all turns. Context limit is checked before every request.
