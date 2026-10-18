# Human usability implementation plan

Status: planned, not implemented. Keep existing runtime, permissions and local single-model execution. Each implementation ticket gets its own feature branch and updates its module spec.

## Observed problems

- Worker output is hidden in a card details panel as preformatted text; job IDs are plain text.
- There is no primary final-result view for a goal. Completed model inference can still produce a rejected plan or result.
- The manager invented worker IDs in two observed runs. A later poem plan used 12 tasks, unnecessary workspace research, and no final assembly task.
- Connection failures leave a connecting placeholder beside an authentication message.
- Creating a goal exposes bot selection, token budgets and approval internals before the user gets a result.

## Target journey

Connect -> describe desired outcome -> start with suitable defaults -> see meaningful progress -> answer or approve when necessary -> read the final result -> copy/download or request a revision.

Use one goal workspace with Overview, Result, Tasks and Activity. Reuse it for direct runs where possible. Keep technical details available under Diagnostics. Kanban is an optional task view.

## Ordered tickets

- [x] HU-01: Readable worker results (first delivery).
  Add an obvious View result action to every task. Show output in a wide reading panel, with task title, worker name, review status and last update. Preserve paragraphs, poetry line breaks, lists, headings, code and tables where supported. JSON envelopes show their output field; structured data gets a readable tree/table with raw JSON under Diagnostics. Offer Copy and Download as text; preserve exact output on export. Empty, running, failed, malformed and rejected results have distinct states. Never label partial/rejected output as final.
  Acceptance: a poem is readable without opening history or decoding JSON; mobile text wraps; copy/download match stored content; polling preserves reading position and selection; malicious output cannot execute scripts or load remote resources.

- [ ] HU-02: One final deliverable per goal. Depends on HU-01.
  Add a persisted goal result with source task/run references, revision, review decision and status. A single-task goal uses its reviewed output. Multi-task goals require an explicit assembly step and review against the original goal before completion. Assembly consumes normal budgets and survives restart. Show Result prominently and retain earlier versions.
  Acceptance: completing stanza tasks alone cannot mark a poem delivered; the full poem survives reload/restart; missing assembly, failed review and budget exhaustion remain visibly incomplete.

- [ ] HU-03: Plans proportional to the goal.
  Prefer one task for simple writing; split only when needed. Use actual allowed worker IDs and capabilities; never require files/research unless the goal needs evidence. Validate names, required fields and delivery coverage before materializing cards. Return specific errors showing the offending task and allowed workers. Bound any plan correction attempt by existing budgets; preserve original output and correction history.
  Acceptance: the observed simple poem scenarios produce a complete writing task without invented workers or workspace research; invalid plans never dispatch; retry exhaustion explains the next action. No claim that prompt changes guarantee model quality.

- [ ] HU-04: Clear startup and goal creation.
  Distinguish connecting, connected, offline, expired/missing access and storage failure; give a recovery action without exposing tokens. Show an outcome field and capability-based worker defaults; explain missing packages. Put budgets and approval settings in Advanced, with readable summaries. Explain prompt-only limitations and serial GPU queuing where relevant.
  Acceptance: a new user can connect and start a text task without entering IDs or token counts; defaults keep current permission/approval boundaries; expired authentication never leaves an indefinite connecting message.

- [ ] HU-05: Progress and actionable recovery.
  Show current action, queued/running distinction, elapsed time, completed tasks and what is needed next. Separate model response received, plan accepted, worker finished, review passed and goal delivered. Link jobs to readable run details and expandable diagnostics. Show blockers in plain language with specific actions for missing files, invalid plans, exhausted budget and interrupted work.
  Acceptance: the two invalid-worker jobs explain the rejected assignment; a finished inference job is never presented as a delivered goal; retry explains possible repeated tool effects; avoid invented ETA/progress percentages.

- [ ] HU-06: Approvals, questions and revisions.
  Replace browser prompts with accessible dialogs showing the proposed action/output and relevant tool access. Put questions next to the task with an answer field. Save answers before explicit resume. Offer Request changes with a retained prior result and a new revision; do not silently reopen completed goals or replay uncertain actions.
  Acceptance: approve, reject, answer and request changes have clear consequences; errors retain typed text; double clicks cannot duplicate actions; refresh preserves pending decisions.

- [ ] HU-07: Consistent navigation and accessible layouts.
  Share navigation, status badges, result reader, empty/error states and form controls across dashboard and board. Provide keyboard navigation, visible focus, named controls, sufficient contrast and appropriate live announcements. Collapse advanced activity on narrow screens. Save filters and avoid polling that resets active controls.
  Acceptance: keyboard-only goal-to-result journey; usable at 360px and desktop without page overflow; screen readers receive status changes without the entire board being reread.

- [ ] HU-08: Validate the complete human journey.
  Run browser acceptance with real local inference for a poem and a file-grounded answer. Cover authentication failure, invalid worker, approval, blocker/answer, cancellation, restart, rejected output, final assembly, revision and copy/download. Record outcomes and remaining model failures; mark tickets complete only when their checks pass.
  Acceptance: a user reaches a readable, reviewed deliverable without inspecting JSON or job IDs; diagnostics retain evidence for every failure; tests and offline build still pass.

## Delivery boundaries

Implement HU-01 first so existing worker results become useful immediately. HU-02 and HU-03 follow before claiming end-to-end delivery. Subsequent tickets complete the wider platform journey. This plan does not require trained adapters, external services or simultaneous GPU workers. Existing adapter, sandbox and reliability gaps remain tracked in todo.md.

Pseudocode contract: [human-usability.spec.md](../specs/human-usability.spec.md).
