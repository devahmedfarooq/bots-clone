param([int]$Port = 8098)
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Set-Location $project
$testRoot = Join-Path $project ('.local/supervisor-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$framework = Get-Content framework.json -Raw | ConvertFrom-Json
$framework.home = Join-Path $testRoot 'state'
$framework.engine.binary = Join-Path $project $framework.engine.binary
$framework.engine.model_path = Join-Path $project $framework.engine.model_path
$configPath = Join-Path $testRoot 'framework.json'
[System.IO.File]::WriteAllText($configPath, ($framework | ConvertTo-Json -Depth 10), [System.Text.UTF8Encoding]::new($false))
$supervisor = $null
$ownedChildren = @()
function Await-Service($ParentID, $ExcludeID = 0) {
    $deadline = (Get-Date).AddSeconds(20)
    do {
        $child = Get-CimInstance Win32_Process -Filter "ParentProcessId=$ParentID" | Where-Object { $_.Name -eq 'bots.exe' -and $_.ProcessId -ne $ExcludeID } | Select-Object -First 1
        if ($child) { return $child }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw 'Supervisor service startup/restart timed out'
}
try {
    $arguments = @('-NoProfile','-ExecutionPolicy','Bypass','-File',('"' + (Join-Path $PSScriptRoot 'supervise.ps1') + '"'),'-Config',('"' + $configPath + '"'),'-Port',$Port)
    $supervisor = Start-Process powershell.exe -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $testRoot 'stdout.log') -RedirectStandardError (Join-Path $testRoot 'stderr.log')
    $first = Await-Service $supervisor.Id
    $ownedChildren += $first.ProcessId
    Start-Sleep -Milliseconds 800
    Stop-Process -Id $first.ProcessId
    $second = Await-Service $supervisor.Id $first.ProcessId
    $ownedChildren += $second.ProcessId
    Start-Sleep -Milliseconds 800
    if (!(Get-Process -Id $second.ProcessId -ErrorAction SilentlyContinue)) { throw 'Restarted service did not stay running' }
    @{checks=@('fresh state directory startup','hidden supervisor startup','automatic restart after forced service crash'); passed=$true} | ConvertTo-Json | Set-Content -Encoding UTF8 evaluation/autonomy-supervisor.json
    Write-Output 'Supervisor crash/restart acceptance passed.'
} finally {
    if ($supervisor) {
        $ownedChildren += @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$($supervisor.Id)" -ErrorAction SilentlyContinue | Where-Object Name -eq 'bots.exe' | Select-Object -ExpandProperty ProcessId)
        Stop-Process -Id $supervisor.Id -ErrorAction SilentlyContinue
    }
    foreach ($childId in ($ownedChildren | Select-Object -Unique)) { Stop-Process -Id $childId -ErrorAction SilentlyContinue }
}
