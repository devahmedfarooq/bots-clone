$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
$sessionPath = Join-Path $project '.local/service-smoke-session.json'
if (!(Test-Path -LiteralPath $sessionPath)) { return }
$session = Get-Content $sessionPath -Raw | ConvertFrom-Json
$localRoot = [System.IO.Path]::GetFullPath((Join-Path $project '.local')) + [System.IO.Path]::DirectorySeparatorChar
if (![System.IO.Path]::GetFullPath($session.root).StartsWith($localRoot, [System.StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected test state path' }
$process = Get-CimInstance Win32_Process -Filter "ProcessId=$($session.process_id)" -ErrorAction SilentlyContinue
if ($process) {
    if ($process.Name -ne 'bots.exe' -or !$process.CommandLine.Contains($session.root)) { throw 'Test process identity changed; refusing to stop it' }
    Get-CimInstance Win32_Process -Filter "ParentProcessId=$($process.ProcessId)" | Where-Object Name -eq 'llama-server.exe' | ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
    Stop-Process -Id $process.ProcessId -ErrorAction SilentlyContinue
}
Remove-Item -LiteralPath $sessionPath
Write-Output 'Isolated test service stopped.'
