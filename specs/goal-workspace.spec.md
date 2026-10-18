# Goal Workspace Module

STATUS step 1 of 4 done (per-goal folder, path-only tools, versioned writes); steps 2-4 planned
FILES internal/tools/files.go, internal/runner/runner.go, internal/service/{autonomy,queue,deliverables}.go

WORKSPACE(goal):
  dir = <home>/workspaces/<goal id>   (outside the repo, created at goal creation, stored on goal.workspace)
  only server-built; never accepted from API clients (Request.Workspace is json:"-")
  every run of this goal (plan, work, verify) receives the same dir
  runner without workspace dir configured -> legacy behavior (bot context_sources only)

TOOL_ROOTS(run):
  workspace set -> roots = [goal dir (read/write, versioned)] + bot.context_sources (unchanged)
  workspace unset -> roots = bot.context_sources
  path-only: "root" optional, default 0 = goal workspace; tool description says so
  confinement unchanged: relative paths only, no traversal/symlink escape (os.Root)

FILE_WRITE(path, content, overwrite):
  create missing parent directories under the root
  overwrite=true -> replace file
  root is versioned goal workspace and path exists and not overwrite:
    save as <stem>-v2<ext>, -v3 ... first free name (atomic exclusive create); result "created <actual path>"
  any other root -> strict exclusive create, error if exists (contract preserved)

DELIVERABLE_EVIDENCE:
  successful file-write = result starts "created "; key = requested (root, path); latest content wins
  versioned save still counts as the saved deliverable

PROMPT: worker told: files go to this goal's private workspace, use relative paths, same name saves a new version,
        still include full content in the reply

NEXT (not in this step): artifact table + Files tab, task inputs/outputs in plan, projects and attached folders
