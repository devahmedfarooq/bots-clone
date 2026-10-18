# Routing

Explicit ID -> registry Get -> return that ID.
Otherwise tokenize lowercase input -> exact @bot-ID mentions -> one match selects; multiple/no matches return needs_choice + candidate IDs.
Route never executes, changes settings, expands grants, or invokes a model. CLI route and API route share Catalog.Route.
