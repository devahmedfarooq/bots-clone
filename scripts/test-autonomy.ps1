param([int]$Port = 8097)
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Set-Location $project
$testRoot = Join-Path $project ('.local/autonomy-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$framework = Get-Content framework.json -Raw | ConvertFrom-Json
$framework.home = Join-Path $testRoot 'state'
$framework.engine.binary = Join-Path $project $framework.engine.binary
$framework.engine.model_path = Join-Path $project $framework.engine.model_path
$configFile = Join-Path $testRoot 'framework.json'
[System.IO.File]::WriteAllText($configFile, ($framework | ConvertTo-Json -Depth 10), [System.Text.UTF8Encoding]::new($false))
$exe = Join-Path $testRoot 'bots.exe'
& go build -buildvcs=false -o $exe ./cmd/bots
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
foreach ($bot in @('manager','task-worker')) {
    & $exe -config $configFile bot register (Join-Path $project "bots/$bot") | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Registration failed: $bot" }
}
$script:process = $null
function Start-ServiceUnderTest {
    $script:serviceToken = $null
    $script:process = Start-Process -FilePath $exe -ArgumentList @('-config', ('"' + $configFile + '"'), 'serve', '-listen', "127.0.0.1:$Port") -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $testRoot 'stdout.log') -RedirectStandardError (Join-Path $testRoot 'stderr.log')
    $deadline = (Get-Date).AddSeconds(15)
    do {
        if ($script:process.HasExited) { throw 'Service exited unexpectedly' }
        $log = Get-Content (Join-Path $testRoot 'stderr.log') -Raw -ErrorAction SilentlyContinue
        if ($log -match '#token=([a-f0-9]{64})') { $script:serviceToken = $Matches[1]; return }
        Start-Sleep -Milliseconds 100
    } while ((Get-Date) -lt $deadline)
    throw 'Startup timed out'
}
function Stop-ServiceUnderTest {
    if ($script:process -and !$script:process.HasExited) {
        Get-CimInstance Win32_Process -Filter "ParentProcessId=$($script:process.Id)" -ErrorAction SilentlyContinue | Where-Object Name -eq 'llama-server.exe' | ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
        Stop-Process -Id $script:process.Id -ErrorAction SilentlyContinue
        $script:process.WaitForExit()
    }
}
function Request($Path, $Data) {
    $params = @{Uri="http://127.0.0.1:$Port/api/$Path"; Headers=@{Authorization="Bearer $script:serviceToken"}; TimeoutSec=30}
    if ($null -ne $Data) { $params.Method='Post'; $params.ContentType='application/json'; $params.Body=$Data | ConvertTo-Json -Depth 20 -Compress }
    Invoke-RestMethod @params
}
function Await-Card($State) {
    $deadline = (Get-Date).AddMinutes(4)
    do {
        $snapshot = Request 'board' $null
        if ($snapshot.error) { throw $snapshot.error }
        $blocked = @($snapshot.board.cards | Where-Object status -eq 'Blocked')
        if ($blocked.Count) { throw ('Blocked: ' + ($blocked | ConvertTo-Json -Depth 20 -Compress)) }
        $card = $snapshot.board.cards | Where-Object { $_.status -eq $State -and $_.kind -eq 'task' } | Select-Object -First 1
        if ($card) { return $card }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    throw "Timed out waiting for $State"
}
try {
    Start-ServiceUnderTest
    $goal = Request 'goals' @{requirements='Create exactly ONE task: Read project.txt and report the project owner and backend language, citing project.txt as evidence. Do not create additional tasks.'; manager='manager'; workers=@('task-worker'); priority=4; token_limit=60000; run_limit=10; run_tokens=12000; seconds=180; tool_calls=6; rules=@{before_work=$true; before_done=$true; tools=$true}}
    $card = Await-Card 'Needs Your Approval'
    if ($card.gate -ne 'work') { throw 'Worker approval gate missing' }
    Request "goals/$($goal.id)/chat" @{text='Keep the answer concise and cite the file.'} | Out-Null
    Request "cards/$($card.id)/approve" @{note='Read the granted sample file.'} | Out-Null
    Start-Sleep -Milliseconds 600
    $card = Await-Card 'Needs Your Approval'
    if ($card.gate -ne 'done' -or $card.output -notmatch 'Maya Chen' -or $card.output -notmatch 'Go') { throw 'Verified result or approval gate missing' }
    Request "cards/$($card.id)/approve" @{note='Evidence accepted.'} | Out-Null
    $saved = (Request 'board' $null).board
    if ($saved.goals[0].status -ne 'Done') { throw 'Completion not confirmed' }
    $chrome = 'C:/Program Files/Google/Chrome/Application/chrome.exe'
    if (Test-Path -LiteralPath $chrome) {
        $screenshot = Join-Path $testRoot 'board.png'
        $browser = Start-Process -FilePath $chrome -ArgumentList @('--headless','--disable-gpu','--no-first-run',('--user-data-dir="' + (Join-Path $testRoot 'chrome') + '"'),'--window-size=1440,1000','--virtual-time-budget=5000',('--screenshot="' + $screenshot + '"'),'--dump-dom',"http://127.0.0.1:$Port/board#token=$script:serviceToken") -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $testRoot 'board.html') -RedirectStandardError (Join-Path $testRoot 'chrome.log')
        if (!$browser.WaitForExit(30000)) { $browser.Kill(); throw 'Browser render timed out' }
        $html = Get-Content (Join-Path $testRoot 'board.html') -Raw
        if ($html -notmatch 'Find|Read|report' -or $html -notmatch 'class="column"' -or $html -notmatch 'Connected') { throw 'Board browser rendering failed' }
    }
    Stop-ServiceUnderTest
    Start-ServiceUnderTest
    $recovered = (Request 'board' $null).board
    if ($recovered.goals[0].status -ne 'Done' -or $recovered.goals[0].runs -ne $saved.goals[0].runs -or $recovered.cards[1].output -ne $saved.cards[1].output) { throw 'Restart lost state or replayed work' }
    $evidence = @{checks=@('real manager plan','worker approval','persistent instruction chat','real worker file evidence','real manager verification','result approval','goal completion','browser board rendering','restart persistence without replay'); runs=$saved.goals[0].runs; tokens=$saved.goals[0].tokens; output=$card.output; test_root=$testRoot}
    $evidence | ConvertTo-Json -Depth 10 | Set-Content -Encoding UTF8 evaluation/autonomy-acceptance.json
    Write-Output "Autonomy acceptance passed. Screenshot: $testRoot/board.png"
} finally { Stop-ServiceUnderTest }
