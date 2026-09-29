param(
    [string]$ExpectedTag,
    [string]$Output = (Join-Path $PSScriptRoot 'output/grsai-video'),
    [switch]$HealthOnly
)

$ErrorActionPreference = 'Stop'
if (-not $ExpectedTag) {
    $commit = (& git -C (Split-Path $PSScriptRoot -Parent) rev-parse --short=7 HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $commit -notmatch '^[0-9a-f]{7}$') { throw 'Cannot resolve local candidate commit' }
    $ExpectedTag = "sha-$commit"
}
$state = (& ssh tenxunyun.guigu '/opt/sub2api/scripts/gray-status.sh') -join "`n"
if ($LASTEXITCODE -ne 0 -or $state -notmatch '(?m)^candidate_slot=green$' -or
    $state -notmatch '(?m)^candidate_percent=0$' -or
    $state -notmatch "(?m)^green_image=ghcr[.]io/eternity714/sub2api:$([regex]::Escape($ExpectedTag))$") {
    throw "The 0% green candidate does not match $ExpectedTag"
}

$tunnel = $null
$port = 18082
try {
    $listener = Get-NetTCPConnection -LocalAddress '127.0.0.1' -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    if ($listener) {
        $process = Get-CimInstance Win32_Process -Filter "ProcessId = $($listener.OwningProcess)"
        $expectedForward = "127.0.0.1:${port}:127.0.0.1:${port}"
        if (-not $process -or $process.Name -ne 'ssh.exe' -or
            $process.CommandLine -notlike "*$expectedForward*tenxunyun.guigu*") {
            throw "Local port $port is not the expected green SSH tunnel"
        }
    } else {
        $tunnel = Start-Process -FilePath (Get-Command ssh).Source -ArgumentList @(
            '-N', '-o', 'ExitOnForwardFailure=yes', '-o', 'ServerAliveInterval=30',
            '-L', "127.0.0.1:${port}:127.0.0.1:${port}", 'tenxunyun.guigu'
        ) -WindowStyle Hidden -PassThru
    }

    $healthy = $false
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try {
            $response = Invoke-RestMethod -Uri "http://127.0.0.1:${port}/health" -TimeoutSec 3
            if ($response.status -eq 'ok') { $healthy = $true; break }
        } catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $healthy) { throw 'Green health check through the local tunnel failed' }
    Write-Output "Green tunnel healthy on http://127.0.0.1:${port} ($ExpectedTag)"
    if ($HealthOnly) { return }
    if (-not $env:SUB2API_KEY) { throw 'Set SUB2API_KEY to the GRSAI test group key' }

    & python (Join-Path $PSScriptRoot 'grsai_sub2api_video.py') `
        --base "http://127.0.0.1:${port}" --output $Output
    if ($LASTEXITCODE -ne 0) { throw "Video test failed (exit $LASTEXITCODE); use task.json to resume without another POST" }
} finally {
    if ($tunnel) { Stop-Process -Id $tunnel.Id -ErrorAction SilentlyContinue }
}
