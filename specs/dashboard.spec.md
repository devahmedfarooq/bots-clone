# HTMX + Tailwind dashboard

Build: pinned npm lock -> copy HTMX distribution/license locally -> Tailwind CLI scans Go templates -> compile frontend/dashboard.css design tokens + shared @apply component classes -> embed checked-in CSS/JS and templates in Go binary. Runtime requires neither npm nor a CDN.
Layout: templates/page.html is the public shell; workspace/activity templates compose centralized button, badge, bot-card and run-output definitions in components.html. html/template escapes dynamic data; redaction applies before presenting strings.
Bootstrap: remove URL-fragment token -> sessionStorage -> HTMX configRequest bearer header. Disable eval, injected scripts, inline indicator styles and history caching for strict self-only CSP.
GET /ui/* -> authenticate exact Host/origin/bearer -> return HTML fragments. POST /ui/* -> same checks + bounded URL-encoded forms -> shared Catalog/Queue services. JSON /api/* stays JSON-only and uses the same bot views, package installation and sorted jobs helpers.
Install -> trigger botsChanged -> refresh bot cards/picker + out-of-band submit control (first install enables Run). Selection changes fetch effective settings. Submit/cancel -> trigger jobsChanged -> refresh activity/saved runs.
Poll every two seconds through declarative hx-trigger. Completed output/details/trace have stable prefixed IDs + hx-preserve; running placeholder is replaced on completion. Trace GET replaces its shared trace-section component.
Small app.js handles only authentication and request/connection errors; no client-side card rendering or polling loop. Tailwind tokens/component classes centralize styling; responsive layout, focus states, labels, live status and request disable/indicators remain.
Verify protected fragments, bounded forms, escaping/redaction, first install, selection after HTMX settle, live install/run/cancel, trace persistence and mobile overflow.
