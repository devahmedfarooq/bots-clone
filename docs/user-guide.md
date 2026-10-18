# Local Bots | User Guide
Windows + RTX 5050 laptop | October 2026

A practical guide to the Local Adapter Bot Framework in this repository. Examples assume PowerShell opened at the repository root. This guide follows the current source; some older documents under docs/ describe earlier interfaces.

## 1. Start here
The app runs specialized bots through one local llama.cpp model. Use the main dashboard for individual runs and installation, or Bot Studio for assignments, revisions and manager-led tasks. After setup, inference works offline without an API key. One inference slot is shared by the bots.

### First-time setup
Requirements: Go 1.26+, Git, PowerShell, curl, and a Vulkan-capable GPU driver. Python is not required to use the app. Setup downloads pinned engine/model artifacts; the model is about 1.84 GB, with extra space needed for caches and the engine.

```powershell
Set-Location <path-to-this-repository>
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1
.\bin\bots.exe diagnostics
.\bin\bots.exe bot list
```

Register only missing bots. A duplicate ID deliberately fails. For a simple writing session and optional manager delegation:

```powershell
.\bin\bots.exe bot register bots/writer
.\bin\bots.exe bot register bots/manager
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/start.ps1
```

Open the URL printed by the service (http://127.0.0.1:8080/board). There is no login. Keep the PowerShell window running. Stop a foreground service with Ctrl+C. To use another port, append -Port 8081 to the start command.

### Your first result
Click Bot Operations in the main dashboard header. In Bot Studio, select Writer, enter "Write a friendly 80-word announcement for a local coffee shop. Return only the announcement.", then press Assign task (or Ctrl+Enter). Wait for Done, read the result, and use Copy or Download.

The first request may take longer while the model loads. Keep the laptop powered on and awake.

---PAGE---
## 2. Use Bot Studio
Open Bot Operations from the main dashboard. The Studio is at /board. Only installed, compatible prompt-only bots appear in its assistant picker.

### Pick how work is assigned
Choose a named assistant for a direct task. This creates one worker card and skips manager planning and verification. Choose Manager decides for planning, assignment and review; this choice appears when manager is installed, and uses all installed eligible workers.

Available repository packages include:

| Assistant | Use it for |
| Writer | Articles, copy, poems and documentation |
| Landing page | HTML/CSS page drafts |
| Code | Scripts and program source |
| Math | Step-by-step calculations |
| QA | Review findings and test suggestions |
| workspace-qa / task-worker | Questions about their granted example files |
| extract | Invoice text converted to JSON |

Install a missing assistant from its corresponding bots/<id> folder. Generated code is a draft to inspect and run yourself; the shipped file tools do not execute programs.

### Read, export and revise
My work reopens earlier assignments. The result pane shows progress and output; multi-task goals have subtask tabs. When multiple subtasks are Done, Final result combines their outputs in task order. It is not an additional model-written synthesis.

Copy copies the selected result. Download saves it through your browser, choosing a text/code extension from the content. These controls do not prove that a worker wrote a file into its workspace. Live preview appears for recognizable HTML and renders it inside a sandboxed iframe. Diagnostics shows the raw model output; Formatted returns to the readable view.

For a completed direct assignment, enter a revision in Ask for changes and click Send & redo. A blocked task also retries after this action. On a completed manager-led goal, chat is saved but does not reopen finished work: create a new assignment for revised requirements.

### Main dashboard alternative
At /, choose a Bot under Start a task, enter the Task, then click Run task. Follow Task activity, inspect Saved runs and traces, or cancel an active job. Cancel does not undo file actions already completed. Use Install a local bot to install a directory or checksummed ZIP by local path.

---PAGE---
## 3. Write tasks that work
Give the bot a clear input, desired output and a measurable finish condition. Keep tasks small for the local 1.7B model; split large documents or programs into stages.

### Copy-ready examples
Writer: "Write a 120-word LinkedIn post announcing my local AI bot tool. Audience: developers. Mention offline inference and a shared GPU. End with one question. Do not invent performance claims."

Landing page: "Return one complete HTML document with embedded CSS for a coffee shop. Include a hero, three product cards and a contact section. Use placeholder contact details."

Code: "Write a Python function that reads a CSV containing price and date columns and returns a 3-row moving average. Include a short usage example. Do not run it."

QA: "Review the following function for invalid inputs and boundary cases. Return a numbered issue list and suggested test cases: [paste the function]."

Workspace QA: "Read project.txt. Who owns Project Cedar? Cite the file and the exact supporting sentence."

Manager decides: "Write a short coffee-shop announcement and produce a matching landing-page HTML draft. Finish when both deliverables are present. Use the available writer and landing-page workers."

### File access and saved output
Bots can only use the tools granted in their bot.json, and a file path in a prompt grants nothing. Every goal gets its own private workspace folder under .local/workspaces/<goal id>; the Studio shows its path under History and limits. File tools work there by relative path (the root argument is optional), and a bot's own context_sources stay available as extra read-only-by-convention roots.

To request a saved file: "Save a 100-word product announcement to announcement.md. Confirm the saved relative path." Verify the actual file in the goal workspace and the trace. Downloading a result and saving through a bot tool are separate operations.

Saving to a name that already exists in that goal's workspace creates a new version (name-v2.ext, name-v3.ext); overwrite:true replaces the file instead. File reads/writes are bounded to 64 KiB; search/output and time budgets also apply. Tools cannot browse arbitrary folders or run shell commands.

### Read the status
Queued = waiting. Working = inference/tool work. Checking = manager review. Needs your OK = approval required. Needs attention = blocked; inspect its message. Done = the app finished the task, so review the actual result before using it.

Malformed responses or unsupported work can block a task. The recorded baseline passes 9 of 12 fixtures, with a known search-to-read failure. Small local model output still needs review; adapter examples remain experimental.

---PAGE---
## 4. Budgets, approvals and recovery
Expand Advanced before assigning a task. For an existing goal, open History and limits and click Save limits after editing.

| Control | Meaning |
| Priority | 1 to 5; 5 is highest |
| Total token budget | Prompt plus generated tokens across the goal |
| Max runs | Total worker, planning and review dispatch allowance |
| Tokens per run | Upper token allowance reserved before each dispatch |
| Seconds per run | Time allowance for each dispatch |
| Tool calls per run | Maximum permitted tool calls per dispatch |

The Studio initially shows 120,000 total tokens, 30 runs, 24,000 tokens/run, 180 seconds/run and 6 tool calls/run. Bot and engine limits still apply. The manager starts at the bot's 4,096-token context and can grow it toward the configured 16,384-token engine maximum, bounded by the remaining reserved run tokens. Allocation is recorded in the run trace. Goal and per-run token caps still apply.

### Choose approval rules
Advanced offers Ask me before any assistant runs, Ask me to sign off on results, and Ask me before file/tool use. The current Studio initially leaves these boxes unchecked; enable the rules you want before submission. When a card needs approval, inspect it and choose Approve or Reject. Approval covers one worker dispatch.

### Unblock work deliberately
Open the affected assignment and read its notice, Diagnostics, and History and limits. Answer missing requirements through Ask for changes. Increase exhausted limits when appropriate, then retry. Try again reruns worker work; Retry review reruns verification. Retrying a worker can repeat its file actions, so inspect the output and saved files first.

The controller reserves one run and its maximum token allowance before dispatch. Completed runs release unused tokens; uncertain interrupted work can retain reservations. Limits cannot be reduced below used/reserved amounts. New settings affect later dispatches, while admitted work keeps its limits.

### Restart behavior
The board and traces persist in .local/metadata.db. Restart reconciles completed jobs and resumes eligible waiting work. Interrupted queued/running work becomes blocked for explicit review; uncertain tool actions are not automatically replayed. After an interruption, inspect both traces and actual files before retrying.

Changing an installed bot version blocks existing goals. Create a new goal for the new version. A manager-led goal that fails review may need a fresh corrected assignment rather than chat on a finished goal.

---PAGE---
## 5. CLI reference
Run commands from the repository root. Put global -config before the command, and run flags after the bot ID. Stop a separate CLI inference process before trying to use the service: one OS lock prevents multiple model copies.

### Inspect and register
```powershell
.\bin\bots.exe diagnostics
.\bin\bots.exe bot list
.\bin\bots.exe bot inspect writer
.\bin\bots.exe bot validate bots/extract
.\bin\bots.exe bot register bots/extract
```

### Run a task
```powershell
.\bin\bots.exe run extract -input 'Invoice INV-104 from Cedar Supplies. Total USD 149.50.'
.\bin\bots.exe run writer -input 'Write a four-line poem about rain.' -json=false
```

The invoice example should yield vendor Cedar Supplies, invoice_number INV-104, currency USD and total 149.50. Verify the actual response. By default the CLI prints a JSON run record; -json=false prints the answer.

### Conversation and saved runs
```powershell
.\bin\bots.exe run workspace-qa -interactive -stream
.\bin\bots.exe run workspace-qa -session SESSION_ID -input 'What database does it use?'
.\bin\bots.exe run show RUN_ID
```

Replace SESSION_ID and RUN_ID with real returned IDs. Interactive mode preserves a session and keeps weights resident. Enter /quit to exit; Ctrl+C cancels. Ordinary runs create fresh isolated sessions unless -session is explicit. A session resumes only under the same bot ID and version. Stream text goes to stderr; JSON stays on stdout.

### Generation settings
```powershell
.\bin\bots.exe settings writer -temperature 0.3 -max-tokens 1024
.\bin\bots.exe run writer -context 2048 -max-tokens 400 -input 'Explain local inference in 80 words.'
```

Precedence is framework defaults, bot settings, then run flags. Context must stay within engine.context_size and leave room for output and tool schemas. Independent CLI invocations unload on exit. Prefer the resident service or interactive mode for repeated use.

---PAGE---
## 6. Add and extend bots
A bot package defines instructions, model identity, generation settings, resource budgets, file roots and tool grants. No framework code changes are needed for a prompt-only bot.

### Create your own package
Copy examples/package-template into a new directory such as bots/my-helper. Edit bot.json: choose a unique lowercase id, version and description; write focused instructions; match base_model to the configured model_id; grant only required tools and context folders. Update fixture expectations if using the sample evaluation suite.

```powershell
Copy-Item examples/package-template bots/my-helper -Recurse
# Edit bots/my-helper/bot.json before continuing.
.\bin\bots.exe bot validate bots/my-helper
.\bin\bots.exe bot register bots/my-helper
.\bin\bots.exe run my-helper -input 'Reply with only READY.'
```

Directory registration retains absolute references to the package. Keep the directory in place. Refresh/reopen Studio after installing another assistant. Validation checks schema and references, not output quality.

### Move a package
```powershell
.\bin\bots.exe package export my-helper .local/my-helper.zip
.\bin\bots.exe package install .local/my-helper.zip
```

The install line is for a target registry where the ID is not installed. Use package upgrade for a different version of an existing package and package rollback ID VERSION for a retained version. ZIP manifests require checksums and matching ID/version; exported archives simplify portability. Shared weights and histories remain after package uninstall ID.

### Workflows and trained adapters
For a predefined pipeline rather than manager planning, submit a schema-version-1 workflow to POST /api/workflows. examples/workflow.json shows step IDs, bot IDs, literal input, input_from and depends_on. Text/JSON contracts must match, graphs must be acyclic, and steps use separate sessions. The API needs no login (loopback only). See docs/local-service.md for endpoints and recovery rules; some adapter-status prose in that older document is outdated.

Adapters are an advanced optional path. Follow training/README.md and docs/bot-authoring.md, and select framework.adapters.json with global -config. Import referenced GGUF weights separately using artifact import PATH DIGEST. Exact base/tokenizer/runtime compatibility is checked. The example adapters are experimental and are not required for ordinary dashboard use.

---PAGE---
## 7. Keep it running and fix problems
### Background operation (optional)
These commands change Windows login startup; use them only when you want automatic background operation.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-autostart.ps1
Start-ScheduledTask -TaskName 'Local Bots'
Stop-ScheduledTask -TaskName 'Local Bots'
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-autostart.ps1 -Remove
```

Start starts the registered task, Stop stops it, and -Remove removes future login startup. Avoid running a second foreground server alongside it. For a manually supervised foreground service, run scripts/supervise.ps1 through PowerShell instead. The supervisor restarts abnormal service exits with backoff.

The laptop must stay on and awake. Sleep pauses work; shutdown/logout stops a login task. The background dashboard link is in .local/service.stderr.log; restart details are in .local/supervisor.log.

### Troubleshooting
| Symptom | What to do |
| Missing/expired access key | Reopen the latest full service URL after a restart. |
| No assistants in Studio | Install compatible prompt-only bots, then reload. |
| Busy inference lock | Finish or stop the other CLI/service using the model. |
| Missing engine/model | Run setup online again; inspect diagnostics. |
| GPU memory/context error | Close GPU-heavy apps, shorten input/context/output, inspect .local/engine.log. |
| No usable GPU | Verify --list-devices on the configured llama-server; engine.gpu_layers=0 selects CPU mode. |
| Task blocked or budget depleted | Inspect history, fix the cause, adjust limits and retry. |
| Queue full | Wait for earlier work to finish and submit again. |
| Output truncated | Use a smaller task or adjust output within the context limit. |

### Backups and offline use
Stop the service before copying .local/metadata.db and any accompanying WAL/SHM files. Keep framework configuration, bot directories and required engine/model artifacts. Histories and retrieved text are local persisted data. Restore only while stopped. After artifacts and dependencies exist, normal inference works offline; scripts/setup.ps1 -Offline rebuilds using cached dependencies.

### Source map
Current behavior was checked against README.md; cmd/bots/main.go; scripts/start.ps1 and install-autostart.ps1; internal/service/templates/workspace.html; internal/service/web/board.html and board.js; internal/tools/files.go; bot definitions; docs/autonomous-operation.md and bot-authoring.md. This guide documents source behavior, not a new live inference acceptance test. Older development documents can lag behind the current implementation.
