# Registry

Register directory/bot.json -> validate and canonicalize references -> write exclusive ID.json in local registry.
IDs never become unchecked paths. Existing IDs require explicit removal before replacement. List sorted definitions, inspect/revalidate on reads, remove definition only; run history remains.
