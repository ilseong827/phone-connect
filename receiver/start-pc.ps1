$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $project
$sessionFile = Join-Path $project '.tools/pc-session.json'
if (Test-Path -LiteralPath $sessionFile) {
    $previous = Get-Content -LiteralPath $sessionFile -Raw | ConvertFrom-Json
    foreach ($processId in @($previous.receiverPid, $previous.viewerPid, $previous.tunnelPid)) {
        if ($processId -and (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
            throw 'An existing test session may still be running. Use stop-pc.ps1 first.'
        }
    }
}
$randomBytes = New-Object byte[] 24
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($randomBytes)
$rng.Dispose()
$token = [Convert]::ToBase64String($randomBytes).Replace('+','-').Replace('/','_').TrimEnd('=')
$env:RELAY_TOKEN = $token
$env:RELAY_DATA = Join-Path $project 'receiver-data'
$python = Join-Path $project '.tools/python/python.exe'
$receiver = Start-Process -FilePath $python -ArgumentList 'receiver/receiver.py' -WorkingDirectory $project -WindowStyle Hidden -PassThru -RedirectStandardOutput '.tools/receiver.log' -RedirectStandardError '.tools/receiver-error.log'
$viewer = Start-Process -FilePath $python -ArgumentList 'receiver/chat_viewer.py' -WorkingDirectory $project -WindowStyle Hidden -PassThru -RedirectStandardOutput '.tools/viewer.log' -RedirectStandardError '.tools/viewer-error.log'
$tunnel = Start-Process -FilePath (Join-Path $project '.tools/cloudflared.exe') -ArgumentList 'tunnel --no-autoupdate --protocol http2 --url http://127.0.0.1:8787' -WorkingDirectory $project -WindowStyle Hidden -PassThru -RedirectStandardOutput '.tools/tunnel-out.log' -RedirectStandardError '.tools/tunnel.log'
$session = @{token=$token;url='';receiverPid=$receiver.Id;viewerPid=$viewer.Id;tunnelPid=$tunnel.Id;startedAt=(Get-Date).ToString('o')}
$session | ConvertTo-Json | Set-Content -LiteralPath $sessionFile -Encoding UTF8
Remove-Item Env:RELAY_TOKEN
for ($attempt=0; $attempt -lt 30; $attempt++) {
    $log = Get-Content -LiteralPath '.tools/tunnel.log' -Raw -ErrorAction SilentlyContinue
    $match = [regex]::Match([string]$log, 'https://[a-z0-9-]+\.trycloudflare\.com')
    if ($match.Success) {
        $session.url=$match.Value
        $session | ConvertTo-Json | Set-Content -LiteralPath $sessionFile -Encoding UTF8
        Write-Output ('Receiver URL: '+$session.url)
        Write-Output 'PC viewer: http://127.0.0.1:8788'
        exit
    }
    Start-Sleep -Seconds 1
}
Write-Output 'Tunnel URL not ready. Check .tools/tunnel.log.'
