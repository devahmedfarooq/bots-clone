# Autonomous bot operation

Build with `go build -buildvcs=false -o bin/bots.exe ./cmd/bots`. Register the provided packages once:

```powershell
bin/bots.exe bot register bots/manager
bin/bots.exe bot register bots/task-worker
scripts/start.ps1
```

Open the printed dashboard link, then choose **Bot Operations** (`/board`). Authentication carries across in the browser session. Create a goal, choose `manager` and one or more workers, and set its priority (5 highest), total token/run budgets, per-run time/token/tool caps, and approval rules. Manager planning and verification count toward the budget. All inference remains serial for the local RTX 5050 and shared base model. These packages use the configured Qwen baseline; adjust their base model if using another engine.

Tasks move through Inbox, Ready, In Progress, Review, Done, Blocked and Needs Your Approval. The manager generates tasks, pins worker versions, assigns ownership, verifies results and confirms completion. Each card saves requirements, acceptance criteria, owner, output, job ID, decisions and dated history. The supplied worker reads the example workspace and drafts text; install workers with suitable grants for other tasks. The manager cannot grant itself or workers new tools.

Expand a card for evidence and history. Chat saves instructions and answers for the next dispatch. Worker progress, questions and blockers appear on the card. Answer a question in chat, then Retry the card. Retry review reruns verification; retry a worker task can repeat its tool actions. Inspect the job and `/api/runs/{run-id}/trace` first after an interruption. Rejected results and malformed model responses stay Blocked; they are never silently marked complete. For revised work after failed review, create a corrected goal. Chat on a completed goal does not reopen it.

Default approval rules request permission for workers with tools. You can also require approval for all worker runs and/or verified results, or allow routine work automatically. Approval covers one worker dispatch. Settings changes apply to subsequent dispatches; already running work keeps its admitted limits. Changing installed bot versions blocks existing goals; create a new goal to use the new version.

The manager starts with each bot's normal context and automatically grows it when a task cannot fit, up to the engine maximum and the run's remaining reserved tokens. The laptop configuration permits up to 8192 context tokens; normal bots still start at 4096. Allocation is recorded in the run trace, and completed tool calls are not replayed. This does not increase your total or per-run token caps. Increase Tokens per run and, if needed, engine.context_size when the explicit maximum is reached.

Total tokens include prompt and completion usage. Before dispatch, the controller reserves its maximum token allowance and one run, so concurrent requests cannot overspend. Completed runs release unused tokens; uncertain interrupted work retains its reservation. Increase a depleted budget in goal settings and Retry the blocked card. Limits cannot be set below used/reserved amounts. Local inference has no metered dollar charge; token, run, time and tool limits bound resource use.

SQLite `.local/metadata.db` stores the board, chat, histories, job checkpoints and run traces. Dispatch keys connect tasks and jobs durably, including a crash between submission and card update. Restart resumes undispatched waiting work and reconciles completed jobs. Interrupted queued/running actions become Blocked for explicit review, preventing duplicate side effects. Storage errors are shown on the board; no state advances unless its checkpoint succeeds. Keep the database and its WAL/SHM files together; stop the service before copying a backup.

For crash recovery, run `scripts/supervise.ps1`. It restarts abnormal service exits with a 2–60 second backoff, cleaning up its crashed service's inference child; a normal shutdown exits the supervisor. For login startup, run `scripts/install-autostart.ps1`, then `Start-ScheduledTask -TaskName 'Local Bots'`. The task runs hidden as your logged-in user without elevation. Restart history is in `.local/supervisor.log`; current service output and the dashboard link with its per-start token are in `.local/service.stderr.log`. Use `Stop-ScheduledTask -TaskName 'Local Bots'` to stop it and `scripts/install-autostart.ps1 -Remove` to remove future login startup. Registration is optional and is never performed by the app.

**Continuous operation requires the laptop to remain powered on and awake.** Plug it in, provide ventilation and configure Windows sleep/lid settings appropriately. Sleep pauses work; shutdown and logout stop a login task. The app cannot operate while the host is off. GPU inference is configured through `framework.json`; retain the measured single-model configuration for this laptop.

API: GET `/api/board`; POST `/api/goals`; POST `/api/goals/{id}/settings`; POST `/api/goals/{id}/chat` with `{"text":"..."}`; POST `/api/cards/{id}/approve`, `/reject`, `/retry` with `{"note":"..."}`. All mutations require JSON and the existing same-origin checks (no login). Goal creation accepts `requirements`, `manager`, `workers`, `priority`, `token_limit`, `run_limit`, `run_tokens`, `seconds`, `tool_calls`, and `rules` (`before_work`, `before_done`, `tools`).

Module pseudocode: `specs/autonomy.spec.md`. Contract coverage: `go test ./...`. Live model and browser acceptance: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-autonomy.ps1`. Cleanup preserves autonomous dispatch runs and traces, including their deduplication records.
