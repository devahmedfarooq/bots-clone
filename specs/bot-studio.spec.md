# Bot Studio and Human Deliverables Module

STATUS in-progress; human-facing bot assistant studio, direct runs, real deliverables, live previews, zero JSON exposure.

BOTS:
  landing-page:
    role: frontend designer & web app builder
    tools: [file-read, file-search, file-write]
    workspace: bots/landing-page/workspace
    output: index.html, style.css, modern responsive components
  code-writer:
    role: full-stack software engineer
    tools: [file-read, file-search, file-write]
    workspace: bots/code-writer/workspace
    output: runnable scripts, programs, tests in Python/Go/JS
  qa-assistant:
    role: quality assurance & security tester
    tools: [file-read, file-search, file-write]
    workspace: bots/qa-assistant/workspace
    output: test suites, edge case audits, bug reports
  math-solver:
    role: quantitative mathematician
    tools: []
    output: step-by-step derivations, verified final answers
  writer:
    role: author & technical documentation specialist
    tools: [file-read, file-search, file-write]
    workspace: bots/writer/workspace
    output: articles, copy, guides, exported markdown

TOOLS_ENHANCEMENT:
  file-write:
    support optional overwrite=true flag (os.O_CREATE | os.O_WRONLY | os.O_TRUNC)
    maintain default strict non-overwrite behavior (os.O_EXCL) for contract tests
    auto-create parent directories under granted root

DIRECT_MODE:
  goal.direct (UI default): no manager plan, no manager review
  create -> one work card owned by chosen assistant, title = first request line
  run prompt = request + user follow-ups, asks for deliverable only (no JSON)
  output: JSON envelope {output} if present else raw text; empty -> Blocked
  finish -> Done (or Approval gate "done" when before_done rule set)
  retry allowed on Blocked card or Done direct card ("Send & redo" after follow-up)
  manager planned mode kept behind Advanced toggle (strict JSON plan/work/verify)

HUMAN_INTERFACE:
  LAYOUT: left = Assign a task (Assign to buttons, request box, Ctrl+Enter, Advanced) + My work list;
          right = Result for selected request; Board view (kanban) collapsed, optional
  STATUS_WORDS: Queued | Working | Checking | Needs your OK | Needs attention | Done
  RESULT: formatted text (line breaks kept), Copy, Download (ext by content), Live preview when HTML,
          Diagnostics = raw output; blocked reason + Try again; approvals as Approve/Reject buttons
  PREVIEW: iframe sandbox="allow-scripts" only (opaque origin, cannot reach token)
  CONNECTION: Connected | Offline, retrying (poll continues); 401 -> explain reopening bots serve link
  ERRORS: shown in notice region, never alert(); typed text retained on failure
  STATE: newest request auto-selected; selection + assistant remembered in localStorage (optional)
  POLLING: re-render only when board signature changes; chat/settings forms are outside re-rendered areas
  TOKEN: sessionStorage + localStorage
  CHAT_SUBMIT: capture submitting form before async mutation; reset only after success

MANAGER_DELEGATION:
  UI default "Manager decides": goal -> manager plan -> worker subtasks -> manager review -> final result
  workers = every installed prompt-only bot except manager; user may instead pick one bot (direct mode)
  PLAN(goal):
    prompt: fewest tasks (1 for simple writing), owner must be exactly one of allowed worker ids, no research/file task unless needed
    problem(plan): not JSON | 0 or >12 tasks | owner not allowed (name it, list allowed) | empty/oversized fields
    first problem -> store feedback, re-plan once with feedback in prompt (normal budgets)
    second problem -> Blocked with specific reason and original output; never remap owners silently
  FINAL_RESULT: when >=2 subtasks all Done -> tab "Final result" = outputs in task order under task titles (no model call)
  PROGRESS: "N of M subtasks done" from card statuses; no invented percentages

ACCESS:
  no login/token: server listens on loopback only
  kept checks: exact Host, same-origin Origin, reject Sec-Fetch-Site cross-site, JSON/form content type on POST
  consequence: any local process or user on this machine can use the API

WORKER_PROMPT (delegated and direct):
  plain-language task + requirements + acceptance + goal + prior results + user follow-ups; no JSON context dump
  reply = the deliverable itself; plain text accepted as result; JSON {"status":"question|blocked"} for genuine blockers
  files: if saving, overwrite:true and still include full content in reply
UI_DEFAULT_BUDGETS: tokens per run 24000 (bots allow 24000), total 120000; reservations are serial and refunded

REVIEW_PROMPT:
  reviewer sees: goal, task, the worker output once, and compact tool evidence
  tool evidence for file-write = path, status, byte count (never the file content, already in the output)
  other tools = name, path, status, first 200 chars of result
  reason: repeating a saved file in the review prompt doubled its size and exceeded the 8192 context

REVIEW_FALLBACK (review prompt too large even at the engine maximum):
  trigger: verify run fails with termination_reason context_budget and card.review_shrink < 3
  action: review_shrink += 1; card Ready again (same phase); history says the result is too long to review whole
  prompt at shrink n: output replaced by head(2/3) + "[... N characters omitted for review ...]" + tail(1/3)
    limit = max(1000, engine maximum * 2 >> n) characters; prompt carries a NOTE: only the beginning and end are shown
  result of an excerpt review: never auto-approved; card goes to "Needs your OK" (done gate) even when no sign-off rule is set
    tick must not auto-approve it when rules later change
  UI: notice says the manager could only review an excerpt; read the full result, then approve or reject
  still too large after shrink 3 -> Blocked with the real error; full output is never truncated or lost
  the stored/copy/download output is always the complete deliverable
