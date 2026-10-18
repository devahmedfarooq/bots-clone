param([int]$Port = 8091, [switch]$KeepRunning, [string]$RuntimeRoot = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Set-Location $project
$testRoot = Join-Path $project ('.local/service-smoke-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$framework = Get-Content framework.json -Raw | ConvertFrom-Json
$framework.home = Join-Path $testRoot 'state'
if (!$RuntimeRoot) { $RuntimeRoot = $project }
$framework.engine.binary = Join-Path $RuntimeRoot $framework.engine.binary
$framework.engine.model_path = Join-Path $RuntimeRoot $framework.engine.model_path
$configFile = Join-Path $testRoot 'framework.json'
[System.IO.File]::WriteAllText($configFile, ($framework | ConvertTo-Json -Depth 10), [System.Text.UTF8Encoding]::new($false))
$exe = Join-Path $project 'bin/bots.exe'
$stdout = Join-Path $testRoot 'stdout.log'
$stderr = Join-Path $testRoot 'stderr.log'
$process = $null
$completed = $false
function Request($Path, $Data) {
    $params = @{ Uri = "http://127.0.0.1:$Port/api/$Path"; Headers = @{ Authorization = "Bearer $script:serviceToken" }; TimeoutSec = 240 }
    if ($null -ne $Data) { $params.Method = 'Post'; $params.ContentType = 'application/json'; $params.Body = $Data | ConvertTo-Json -Depth 20 -Compress }
    Invoke-RestMethod @params
}
function Await-Job($ID) {
    $deadline = (Get-Date).AddMinutes(4)
    do {
        $job = Request "jobs/$ID" $null
        if ($job.status -notin @('queued','running')) { return $job }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw 'Job did not terminate'
}
try {
    & $exe -config $configFile bot register (Join-Path $project 'bots/extract') | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'CLI registration failed' }
    $qaDir = Join-Path $testRoot 'workspace-qa'
    Copy-Item -LiteralPath (Join-Path $project 'bots/workspace-qa') -Destination $qaDir -Recurse
    Set-Content -LiteralPath (Join-Path $qaDir 'workspace/invoice-sample.txt') -Value 'Invoice INV-104 from Cedar Supplies. Total USD 149.50.' -Encoding UTF8
    $arguments = @('-config', ('"' + $configFile + '"'), 'serve', '-listen', "127.0.0.1:$Port", '-queue', '4')
    $process = Start-Process -FilePath $exe -ArgumentList $arguments -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $deadline = (Get-Date).AddSeconds(15)
    do {
        if ($process.HasExited) { throw ('Service exited: ' + (Get-Content $stderr -Raw)) }
        $log = Get-Content $stderr -Raw -ErrorAction SilentlyContinue
        if ($log -match '#token=([a-f0-9]{64})') { $script:serviceToken = $Matches[1]; break }
        Start-Sleep -Milliseconds 100
    } while ((Get-Date) -lt $deadline)
    if (!$script:serviceToken) { throw 'Service startup timed out' }
    $unauthorized = $false
    try { Invoke-RestMethod "http://127.0.0.1:$Port/api/bots" | Out-Null } catch { $unauthorized = [int]$_.Exception.Response.StatusCode -eq 401 }
    if (!$unauthorized) { throw 'Unauthenticated API access was accepted' }
    Request 'packages' @{ directory = $qaDir } | Out-Null
    $bots = Request 'bots' $null
    if ($bots.Count -ne 2) { throw 'Bot installation/inspection failed' }
    $job = Request 'jobs' @{ bot = 'extract'; input = 'Invoice INV-104 from Cedar Supplies. Total USD 149.50.' }
    $job = Await-Job $job.id
    if ($job.status -ne 'completed') { throw ('Extraction failed: ' + $job.error) }
    $invoice = $job.run.output | ConvertFrom-Json
    if ($invoice.invoice_number -ne 'INV-104' -or $invoice.total -ne 149.50 -or $invoice.currency -ne 'USD') { throw 'Unexpected invoice output' }
    $sse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/api/jobs/$($job.id)/events" -Headers @{ Authorization = "Bearer $script:serviceToken" } -TimeoutSec 10
    if ($sse.Content -notmatch 'event: job' -or $sse.Content -notmatch 'completed') { throw 'SSE terminal event missing' }
    $cancelled = Request 'jobs' @{ bot = 'workspace-qa'; input = 'Read project.txt and explain every detail at length.' }
    Request "jobs/$($cancelled.id)/cancel" @{} | Out-Null
    $cancelled = Await-Job $cancelled.id
    if ($cancelled.status -ne 'cancelled') { throw ('Cancellation failed: ' + $cancelled.status) }
    $definition = @{ schema_version = 1; id = 'invoice-workflow'; steps = @(
        @{ id = 'read'; bot = 'workspace-qa'; input = 'Read invoice-sample.txt. Quote its full contents.'; input_type = 'text'; output_type = 'text' },
        @{ id = 'extract'; bot = 'extract'; depends_on = @('read'); input_from = 'read'; input_type = 'text'; output_type = 'json' }
    ) }
    $flow = Request 'workflows' $definition
    $deadline = (Get-Date).AddMinutes(5)
    do {
        $flow = Request "workflows/$($flow.id)" $null
        if ($flow.status -ne 'running') { break }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    if ($flow.status -ne 'completed') { throw ('Workflow failed: ' + ($flow | ConvertTo-Json -Depth 20 -Compress)) }
    $result = $flow.steps.extract.output | ConvertFrom-Json
    if ($result.invoice_number -ne 'INV-104' -or $result.total -ne 149.50) { throw 'Workflow output did not preserve invoice facts' }
    $trace = Request "runs/$($job.run.id)/trace" $null
    if ($trace.Count -lt 2) { throw 'Run trace missing' }
    $stepMetrics = @{}
    foreach ($property in $flow.steps.PSObject.Properties) {
        $stepJob = Request "jobs/$($property.Value.job)" $null
        $stepMetrics[$property.Name] = @{ duration_ms = $stepJob.run.duration_ms; usage = $stepJob.run.usage; bot = $stepJob.run.bot; version = $stepJob.run.version }
    }
    $evidence = @{ date = (Get-Date).ToUniversalTime().ToString('o'); model = $framework.engine.model_id; checks = @('CLI registration','API local installation','bot settings/compatibility','unauthorized request rejected','real invoice extraction','SSE final event','active cancellation','two-bot workflow','attributable trace'); invoice = $invoice; invoice_metrics = @{ duration_ms = $job.run.duration_ms; usage = $job.run.usage }; workflow_invoice = $result; workflow_metrics = $stepMetrics; resources = $job.resources; adapter_checks = 'Pending Phase 2'; test_state = $testRoot }
    $evidence | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath evaluation/phase3-acceptance.json -Encoding UTF8
    $completed = $true
    if ($KeepRunning) { @{ process_id = $process.Id; root = $testRoot; port = $Port } | ConvertTo-Json | Set-Content -LiteralPath .local/service-smoke-session.json -Encoding UTF8 }
    Write-Output 'Live service acceptance passed. Evidence: evaluation/phase3-acceptance.json'
} finally {
    if ($process -and !($KeepRunning -and $completed)) {
        Get-CimInstance Win32_Process -Filter "ParentProcessId=$($process.Id)" -ErrorAction SilentlyContinue | Where-Object Name -eq 'llama-server.exe' | ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
        Stop-Process -Id $process.Id -ErrorAction SilentlyContinue
    }
}
