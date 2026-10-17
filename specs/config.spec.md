# Configuration

Decode one strict JSON object, reject unknown fields -> validate schema 1, safe bot ID, semantic version, required model/instructions, grants, bounded budgets/settings.
Resolve defaults < bot generation < explicit run overrides. Reject invalid effective settings.
Resolve context directories and evaluation fixture relative to definition, require existing references, canonicalize roots before registration. Phase 1 rejects adapters.
Generation token limit is per request; run token budget covers prompt plus completion across all turns. Context limit is checked before every request.
