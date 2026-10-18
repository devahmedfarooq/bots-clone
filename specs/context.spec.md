# Context retrieval and budgeting

RETRIEVE(configured sources, input):
  opt-in root index + relative file/directory; require file-read grant
  open confined os.Root; reject escaping paths; never follow symlink entries
  bound visited entries, input file size, excerpts, total output bytes
  rank literal query words; return root/path/text attribution
  present JSON as untrusted user data, never system authority
FIT(request):
  tokenize exact chat template; reserve configured completion + tool headroom
  remove oldest complete prior conversation turns, preserving current input/tool-call pairs
  compact older tool results into labelled previews; shrink retrieved data
  never truncate instructions or silently widen roots/tools; fail if irreducible
VERIFY: traversal/injection, attribution, bounds, history/tool-pair preservation

COMPACT: require each replacement to shrink encoded content; otherwise use a fixed marker
  honor cancellation; drop only framework-added retrieval immediately after current input
  never infer retrieval authority from user-authored JSON

AUTONOMOUS_FIT:
  optional adaptive context growth bounded by engine maximum and remaining run token budget
  fit error is typed ErrContextBudget; only this error allows context expansion
  record allocation in run trace; never repeat completed tool calls while fitting larger context
