# Autonomous operation module

MODULE service/autonomy (uses SQLite checkpoints, Queue and installed bot registry)
STATE board = goals[], cards[], goal chat[], append-only card events[]
STATES Inbox -> Ready -> In Progress -> Review -> Done; Blocked; Needs Your Approval

CREATE goal(requirements, manager, worker allowlist, priority, token/run/time/tool budgets, approval rules):
  validate all bot versions, grants, inputs and limits
  persist goal and manager planning card in Inbox atomically before returning
TICK (single scheduler; higher priority first, creation time breaks ties):
  reconcile persisted dispatches before starting another action
  Inbox -> Ready
  Ready -> approval gate when rule requires it, otherwise dispatch
  reserve run and token allowance and persist unique dispatch key BEFORE Queue submission
  Queue deduplicates key across restarts; manager planning/review have no tool grants
  enforce per-run token/time/tool caps at Queue -> Runner boundary
  model output -> remove an optional complete outer JSON code fence, then strictly decode (no text extraction or repair)
  completed plan -> strictly decode tasks envelope or task array; bound tasks, require acceptance and allowlisted owners
  materialize tasks atomically, pin owner versions, record manager decisions
  worker receives goal, prior outputs, instructions, acceptance and card history
  worker structured status = result | progress | question | blocked
  progress -> Ready (bounded by remaining goal budget); question/blocked -> Blocked
  result -> Review; retain all job IDs; manager verifies output and saved worker tool receipts against acceptance and records evidence/reason
  approved -> Done automatically or Needs Your Approval per configured rule
  rejected/invalid/failed -> Blocked with reason; never claim completion
  all cards Done -> manager confirms goal completion in saved chat
APPROVE card: accept only pending execution/review gate; record user decision; resume correct phase
REJECT card: Blocked; record reason
RESUME card: require explicit user action, clear dispatch, preserve history and spent budget
CHAT: persist user instructions/questions and manager acknowledgement; include context in next dispatch
UPDATE goal settings: validate limits >= already reserved usage; apply priority and approval rules
SAVE: clone -> redact -> atomic SQLite checkpoint -> publish memory; errors never advance state
RECOVER: restore board; reconcile dispatch key against saved jobs; completed -> consume once;
  interrupted/uncertain action -> Blocked, preserve reservation and outputs, require explicit retry
  undispatched Ready/Inbox work continues automatically; never replay uncertain tools
CLOSE: stop scheduler before Queue/Store; keep all waiting work persisted
RETENTION: exclude dispatch-key job runs from plans and direct deletion to retain evidence and deduplication

INTERFACE /board: seven Kanban columns, goal creation/settings, manager chat, full task history/output,
  approval/rejection/retry actions, polling, authenticated API, escaped content, visible network errors
HOST: Windows supervisor restarts crashes with backoff; optional login scheduled task; model inference
  remains serial for RTX 5050; host must remain powered on, plugged in and awake for continuous work.
TEST: plan->workers->verification->completion, approval/rejection, budget exhaustion, priorities,
  malformed plans/results, ownership, persistent chat/history, restart reconciliation, key dedup,
  capped runner budgets, API validation and authentication, dashboard render/build.

DELIVERABLES:
  on worker result (direct or managed): load this run's durable tool trace
  successful file-write receipts -> use exact recorded content; latest write per root/path wins
  one file -> its complete text; multiple -> labeled complete contents
  failed, truncated or unfinished writes are never evidence of a saved deliverable
  receipt/filename without nonempty successful write -> Blocked, never Review/Done
  manager verifies actual content rather than only a filename
  startup: repair prior receipt-only cards from completed worker evidence without replaying tools
    missing evidence -> Blocked; keep original runs/history for diagnostics

ADAPTIVE_CONTEXT (autonomous plan/work/review, including direct tasks):
  begin at bot context; reserve its configured output plus tool headroom
  if context cannot fit after safe compaction:
    grow context toward min(engine configured maximum, remaining reserved run tokens)
    release/reacquire single model slot; reload serially; retry fitting only, never replay tools
    record context_allocated event; preserve full current task, tool pairs and all budget caps
  engine maximum for this laptop = 8192; normal bot context stays 4096
  never silently increase goal/per-run token limits or tool/file grants
  if capped, report current context, engine maximum and remaining run tokens with recovery action
PROMPT_CONTEXT:
  send goal requirements, task, relevant output and user instructions
  include worker capabilities for planning; omit operational history, IDs and dispatch telemetry
  keep complete audit history in SQLite, outside the inference prompt
