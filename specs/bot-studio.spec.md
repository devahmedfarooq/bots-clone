# Bot Studio and Human Deliverables Module

STATUS in-progress; human-facing bot assistant studio, real deliverables, live previews, zero JSON exposure.

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

HUMAN_INTERFACE:
  CREATE_GOAL:
    input: plain natural language intent textarea
    assistant_pills: 1-click select (Landing Page, Code, Math, QA, Writer)
    advanced_parameters: collapsed by default (token budgets, approval rules, manager)
    submit: starts task without exposing technical schemas
  
  DELIVERABLE_READER:
    detect_html: if output contains HTML/CSS/DOCTYPE -> enable Live Preview tab
    live_preview: isolated sandbox iframe srcdoc rendering landing page
    code_view: syntax-styled code view with line numbers and copy
    download: one-click file download (.html, .py, .go, .md, .txt)
    hide_technical: raw JSON, token numbers and internal IDs hidden unless user clicks Diagnostics

  CARD_ACTIONS_AND_AUTONOMY:
    default_rules:
      before_work: false
      before_done: false
      tools: false
      note: manager bot handles planning and verification autonomously; human approval only when explicitly requested
    card_buttons:
      approve: direct POST /api/cards/{id}/approve (no blocking browser prompt dialog)
      reject: direct POST /api/cards/{id}/reject (no blocking browser prompt dialog)
      retry: direct POST /api/cards/{id}/retry
      visuals: distinctive green approve and red reject styling
    token_persistence:
      dual_storage: sync token between sessionStorage and localStorage to survive page refresh
