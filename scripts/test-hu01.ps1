$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Set-Location $project

Write-Output "==> [1/3] Running Go unit and asset contract tests..."
$env:GOCACHE = "$project/.cache/go-build"
$env:GOMODCACHE = "$project/.cache/go-mod"
& go test -v ./internal/service -run "TestCardOutputPreservedOnMalformedWorkerOutput|TestBoardReaderAssets"
if ($LASTEXITCODE -ne 0) { throw "Go reader contract tests failed" }

Write-Output "==> [2/3] Running JavaScript payload, typography, and XSS safety tests..."
& node scripts/test-hu01.mjs
if ($LASTEXITCODE -ne 0) { throw "Node HU-01 tests failed" }

Write-Output "==> [3/3] Running full Go test suite..."
& go test ./...
if ($LASTEXITCODE -ne 0) { throw "Go full test suite failed" }

$evidence = @{
    ticket = "HU-01"
    status = "passed"
    checks = @(
        "Reader drawer markup present in board.html",
        "Reader CSS styling and mobile typography present in board.css",
        "JSON envelope extraction and raw fallback verified",
        "Poetry line breaks preserved with <br> and stanzas with <p>",
        "Markdown codeblocks and tables formatted safely",
        "Strict HTML escaping prevents script injection and hostile tags",
        "Distinct status badges for waiting, running, review, done, and blocked",
        "Non-done results never labeled as verified deliverable",
        "Exact text copy and .txt download actions implemented",
        "Polling stability verified without clobbering scroll or selection",
        "Card output preserved in c.Output even on malformed worker outputs"
    )
    timestamp = (Get-Date).ToString("o")
}
$evidencePath = Join-Path $project "evaluation/hu-01-acceptance.json"
$evidence | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $evidencePath
Write-Output "==> HU-01 verification complete. Evidence saved to evaluation/hu-01-acceptance.json."
