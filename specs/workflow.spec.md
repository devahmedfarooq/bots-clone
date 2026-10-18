# Workflows

Definition schema 1 -> unique step IDs -> existing bots -> text/json input and output contracts -> literal XOR dependency output.
Dependency refs exist, unique, no self edges; input_from is a dependency and types match -> stable topological sort -> reject cycles before scheduling.
Retries 0..2 -> only bots with no tool grants; tool side effects are never retried by workflow engine.
Each step executes with a new Runner session; route only declared dependency output, no shared history.
Start -> pin bot versions -> checkpoint all pending -> topological worker -> checkpoint running BEFORE submit -> shared queue -> checkpoint job ID -> poll terminal -> validate output type -> checkpoint result.
Failed dependency -> mark blocked; independent steps may continue. Declared retries apply only to bots without tools. Workflow concurrency limited to eight; model still has one queue slot.
Cancel -> cancel only current job -> mark step uncertain -> stop. Restart running flow -> interrupted; in-flight step -> uncertain; never replay automatically.
Recovery may promote an in-flight step from its already completed job checkpoint; resume validates output contract and pinned bot versions -> only completed/pending steps allowed -> skip completed steps -> continue pending. Uncertain actions require trace review and a new explicit definition.
Runner Started callback -> durably link job to run ID BEFORE model acquisition/tool actions, so interrupted jobs have attributable traces.
