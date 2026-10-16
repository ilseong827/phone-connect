$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$sessionFile = Join-Path $project '.tools/pc-session.json'
if (!(Test-Path -LiteralPath $sessionFile)) { Write-Output 'No session found.'; exit }
$session = Get-Content -LiteralPath $sessionFile -Raw | ConvertFrom-Json
foreach ($processId in @($session.receiverPid,$session.viewerPid,$session.tunnelPid)) {
    if (!$processId) { continue }
    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if ($process -and $process.Path -and $process.Path.StartsWith((Join-Path $project '.tools'),[StringComparison]::OrdinalIgnoreCase) -and $process.StartTime.ToUniversalTime() -ge ([DateTimeOffset]::Parse($session.startedAt).UtcDateTime.AddMinutes(-1))) {
        Stop-Process -Id $processId
    }
}
Write-Output 'Stopped this test session. Received files are preserved.'
