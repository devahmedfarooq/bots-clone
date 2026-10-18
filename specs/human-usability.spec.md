# Human usability module

STATUS in-progress (HU-01); extend dashboard/autonomy, reuse existing Queue/Store and authentication

READ_RESULT(task_or_goal, viewer):
  require current authenticated session
  load persisted output + producing task/run + review state
  classify waiting | running | partial | awaiting_review | rejected | delivered | failed
  normalize known response envelope once; preserve original bytes for diagnostics/export
  unknown/malformed format -> readable escaped fallback, never hide output
  text -> preserve paragraphs/line breaks; structured data -> readable tree/table
  rich text -> allowlisted rendering, escape HTML, forbid active/remote content and unsafe links
  return shared reader model(title, content, status, provenance, revision, actions)
COPY_OR_EXPORT(result): use selected persisted revision; text download; surface failure

PLAN(goal):
  prefer smallest complete plan; simple writing -> one complete-deliverable task
  provide actual worker IDs/capabilities; research only if needed by requirements
  validate schema, allowed owners, bounded size and delivery coverage
  invalid -> save precise error + original output; bounded correction or explicit Blocked
  valid -> persist plan before dispatch; never invent owners or silently remap

FINALIZE(goal):
  single task -> candidate = reviewed worker output
  multiple tasks -> schedule explicit assembly with goal + reviewed sources within remaining budget
  verify candidate against original requirements; apply configured user approval
  persist candidate, source references, review, revision and state atomically
  mark delivered only after required verification/approval; goal complete only after delivery
  restart -> reconcile saved jobs, never duplicate assembly or uncertain effects

DISPLAY(workspace):
  shared navigation -> Overview | Result | Tasks | Activity; Diagnostics expandable
  current action + queue state + concrete next action; distinguish job completion from delivery
  connection errors -> explicit recovery state, replace connecting placeholder
  polling -> preserve reading position, selection, drafts, focus and expanded details
  missing result -> honest empty state; failures -> actionable explanation, retained evidence

ANSWER_OR_DECIDE(task, input):
  show contextual accessible form; retain input on error
  persist answer/decision before explicit resume; enforce existing gates and grants
REQUEST_REVISION(goal, feedback):
  preserve old result; create tracked revision under explicit user instruction
  reuse budgets/approval rules; never automatically replay uncertain tools

VERIFY:
  readable poetry/code/tables/JSON; exact text copy/export; hostile output escaped
  goal delivery separate from job success; invalid plans actionable; multi-task assembly required
  reload/restart, missing token, mobile, keyboard, polling stability and offline build
