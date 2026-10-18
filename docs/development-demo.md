# Development demo and release status

Use branch `development`; each Phase 3 feature has its own branch. `main` remains the Phase 1 baseline. This is a tested prompt-only development product, not the completed trained-adapter release.

1. Run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup.ps1 -Offline` with existing artifacts.
2. Run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/start.ps1` and open its printed token-bearing link.
3. Install `bots/extract` and `bots/workspace-qa` if not already registered. Review their grants.
4. Select extract; submit `Invoice INV-104 from Cedar Supplies. Total USD 149.50.` Expected: vendor Cedar Supplies, invoice_number INV-104, currency USD, total 149.50.
5. Inspect the saved run and its trace. Select workspace-qa, request `Read project.txt. Who owns Project Cedar? Cite the file.`, then try cancelling a longer task.
6. Demonstrate typed workflow routing using the isolated API smoke script; it has workspace-qa read an invoice file and extract consume only that step's text. Both sessions remain separate.
7. To reproduce API/browser acceptance, run `scripts/test-service.ps1 -KeepRunning`, `node scripts/test-dashboard.mjs`, and `scripts/stop-test-service.ps1`. Test state is isolated under `.local`; normal bot registrations are preserved.

Exact artifact versions are pinned by `artifacts/baseline.lock.json`. API output/latency/usage/resource evidence lives in `evaluation/phase3-acceptance.json`; browser checks and images live beside it. `evaluation/development-release.json` pins the backend source and executable checksum used for the final check.

Remaining acceptance: constrained arbitrary executable sandbox, authorized external credential forwarding, unused-artifact reference cleanup, actual GPU OOM/adapter-switch reliability, and the trained adapter demo. See `todo.md`; those tickets remain unchecked. Adapter-dependent checks require Phase 2. Credential forwarding was rejected by automatic approval review because credential sources and recipient plugins were not explicitly authorized.
