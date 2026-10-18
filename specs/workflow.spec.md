# Workflows

Definition schema 1 -> unique step IDs -> existing bots -> text/json input and output contracts -> literal XOR dependency output.
Dependency refs exist, unique, no self edges; input_from is a dependency and types match -> stable topological sort -> reject cycles before scheduling.
Retries 0..2 -> only bots with no tool grants; tool side effects are never retried by workflow engine.
Each step executes with a new Runner session; route only declared dependency output, no shared history.
