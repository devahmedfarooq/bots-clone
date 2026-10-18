# Dashboard components

The dashboard uses Go html/template, HTMX 2.0.11 and compiled Tailwind CSS 4.3.3. Assets are embedded locally in the executable; runtime requires no CDN or Node.js.

| File | Responsibility |
| --- | --- |
| `internal/service/templates/components.html` | Shared button, badge, bot card, output and trace components |
| `internal/service/templates/page.html` | Page shell and local asset loading |
| `internal/service/templates/workspace.html` | Forms, bot picker/settings and panel composition |
| `internal/service/templates/activity.html` | Job and saved-run lists using the shared components |
| `frontend/dashboard.css` | Central design tokens and Tailwind component styles |
| `internal/service/dashboard.go` | HTML rendering, fragment handlers and shared service view helpers |
| `internal/service/web/app.js` | Token bootstrap and request error handling |

Change a shared component or style once; page templates compose it. Avoid rendering cards or polling in JavaScript. HTMX handles requests, form submission, fragment swaps and periodic updates. Completed output/trace components use stable IDs with hx-preserve so reading state survives refresh. Go templates escape dynamic content, and /ui routes retain the API's host/origin/bearer boundary.

To rebuild after editing templates or styling:

```powershell
npm.cmd ci --ignore-scripts
npm.cmd run build:dashboard
go build -buildvcs=false -o bin/bots.exe ./cmd/bots
```

Commit generated `internal/service/web/style.css`, `htmx.min.js` and the HTMX license with the sources so Go-only offline builds remain usable. Exact package versions/integrity are locked. The watcher override uses the patched 2.6.0 release; npm audit reports zero vulnerabilities.

References: [HTMX documentation](https://htmx.org/docs/), [preserved fragments](https://htmx.org/attributes/hx-preserve/), [Tailwind CLI](https://tailwindcss.com/docs/installation/tailwind-cli).
