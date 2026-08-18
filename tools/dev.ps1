[CmdletBinding()]
param(
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'local',
    [ValidateSet('config', 'infra-up', 'infra-down', 'infra-restart', 'status', 'api', 'start', 'stop', 'restart')]
    [string]$Action = 'status',
    [string]$EnvFile = '',
    [string]$LanServerIp = '',
    [switch]$IncludeTauri
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'infrastructure/compose/docker-compose.yml'
$stateRoot = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'NH-Media\dev'
$stateFile = Join-Path $stateRoot 'processes.json'

function Import-EnvFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    if (-not (Test-Path -LiteralPath $Path)) { throw "Environment file was not found: $Path" }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { throw "Invalid environment line in $Path" }
        $name = $Matches[1]
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        Set-Item -Path ("Env:" + $name) -Value $value
    }
}

if ($EnvFile -ne '') { Import-EnvFile $EnvFile }
$env:NH_MEDIA_PROFILE = $Profile
$env:COMPOSE_PROFILES = $Profile
$composeArgs = @('--file', $composeFile, '--profile', $Profile)

function Resolve-LanServerIp {
    if ($Profile -ne 'lan') { return '' }
    if (-not [string]::IsNullOrWhiteSpace($LanServerIp)) {
        $parsed = $null
        if (-not [Net.IPAddress]::TryParse($LanServerIp, [ref]$parsed) -or $parsed.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork -or [Net.IPAddress]::IsLoopback($parsed)) {
            throw 'LanServerIp must be an RFC1918 private-LAN IPv4 address.'
        }
        $octets = $parsed.GetAddressBytes()
        $privateLan = $octets[0] -eq 10 -or
            ($octets[0] -eq 172 -and $octets[1] -ge 16 -and $octets[1] -le 31) -or
            ($octets[0] -eq 192 -and $octets[1] -eq 168)
        if (-not $privateLan) { throw 'LanServerIp must be an RFC1918 private-LAN IPv4 address.' }
        return $parsed.IPAddressToString
    }
    $candidates = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
        Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' -and $_.PrefixOrigin -ne 'WellKnown' } |
        Select-Object -ExpandProperty IPAddress -Unique)
    if ($candidates.Count -ne 1) {
        throw ('LAN profile needs an explicit -LanServerIp because the host has multiple or no usable IPv4 addresses: ' + ($candidates -join ', '))
    }
    return [string]$candidates[0]
}

$resolvedLanServerIp = Resolve-LanServerIp

function Require-Env([string[]]$Names) {
    foreach ($name in $Names) {
        $value = [Environment]::GetEnvironmentVariable($name)
        if ([string]::IsNullOrWhiteSpace($value) -or $value -like 'CHANGE_ME*') {
            throw "$name must be set to a non-default value before starting NH-Media. See .env.example."
        }
    }
}

function Set-AppDefaults {
    Require-Env @(
        'POSTGRES_DB', 'POSTGRES_SUPERUSER', 'POSTGRES_SUPERUSER_PASSWORD', 'POSTGRES_APP_USER',
        'POSTGRES_APP_PASSWORD', 'POSTGRES_MIGRATION_USER', 'POSTGRES_MIGRATION_PASSWORD',
        'REDIS_PASSWORD', 'MINIO_ROOT_USER', 'MINIO_ROOT_PASSWORD', 'MINIO_BUCKET',
        'NH_API_ADMIN_USERNAME', 'NH_API_ADMIN_PASSWORD'
    )
    if ([string]::IsNullOrWhiteSpace($env:NH_API_BIND)) {
        $env:NH_API_BIND = if ($Profile -eq 'lan') { '0.0.0.0:8080' } else { '127.0.0.1:8080' }
    }
    if ([string]::IsNullOrWhiteSpace($env:NH_API_ALLOWED_ORIGINS)) {
        $env:NH_API_ALLOWED_ORIGINS = if ($Profile -eq 'lan') { 'http://' + $resolvedLanServerIp + ':3000,http://tauri.localhost' } else { 'http://127.0.0.1:3000,http://localhost:3000' }
    }
    if ([string]::IsNullOrWhiteSpace($env:NH_MEDIA_DATABASE_URL)) {
        $env:NH_MEDIA_DATABASE_URL = "postgresql://$($env:POSTGRES_APP_USER):$($env:POSTGRES_APP_PASSWORD)@127.0.0.1:5432/$($env:POSTGRES_DB)?sslmode=disable"
    }
    if ([string]::IsNullOrWhiteSpace($env:NH_STORAGE_ENDPOINT)) { $env:NH_STORAGE_ENDPOINT = 'http://127.0.0.1:9000' }
    if ([string]::IsNullOrWhiteSpace($env:NH_STORAGE_ACCESS_KEY)) { $env:NH_STORAGE_ACCESS_KEY = $env:MINIO_ROOT_USER }
    if ([string]::IsNullOrWhiteSpace($env:NH_STORAGE_SECRET_KEY)) { $env:NH_STORAGE_SECRET_KEY = $env:MINIO_ROOT_PASSWORD }
    if ([string]::IsNullOrWhiteSpace($env:NH_STORAGE_BUCKET)) { $env:NH_STORAGE_BUCKET = $env:MINIO_BUCKET }
    if ([string]::IsNullOrWhiteSpace($env:NH_QUEUE_ENDPOINT)) { $env:NH_QUEUE_ENDPOINT = 'redis://127.0.0.1:6379' }
    if ([string]::IsNullOrWhiteSpace($env:NH_MEDIA_WORKER_QUEUE_ENDPOINT)) { $env:NH_MEDIA_WORKER_QUEUE_ENDPOINT = '127.0.0.1:6379' }
    if ([string]::IsNullOrWhiteSpace($env:NH_QUEUE_PASSWORD)) { $env:NH_QUEUE_PASSWORD = $env:REDIS_PASSWORD }
    if ([string]::IsNullOrWhiteSpace($env:NEXT_PUBLIC_NH_MEDIA_API_URL)) {
        $env:NEXT_PUBLIC_NH_MEDIA_API_URL = if ($Profile -eq 'lan') { 'http://' + $resolvedLanServerIp + ':8080' } else { 'http://127.0.0.1:8080' }
    }
    if ($Profile -eq 'lan') {
        if ($env:NH_API_BIND -like '127.*') { throw 'LAN profile cannot bind the Product API to loopback.' }
        $expectedOrigin = 'http://' + $resolvedLanServerIp + ':3000'
        $expectedDesktopOrigin = 'http://tauri.localhost'
        $origins = @($env:NH_API_ALLOWED_ORIGINS -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
        if ($origins -contains '*') { throw 'LAN profile does not allow a wildcard CORS origin.' }
        if ($origins -notcontains $expectedOrigin) { throw ('LAN profile must allow the exact web origin ' + $expectedOrigin + ' in NH_API_ALLOWED_ORIGINS.') }
        if ($origins -notcontains $expectedDesktopOrigin) { throw ('LAN profile must allow the packaged Windows desktop origin ' + $expectedDesktopOrigin + ' in NH_API_ALLOWED_ORIGINS.') }
        if ($env:NEXT_PUBLIC_NH_MEDIA_API_URL -ne ('http://' + $resolvedLanServerIp + ':8080')) { throw ('LAN profile must use NEXT_PUBLIC_NH_MEDIA_API_URL=http://' + $resolvedLanServerIp + ':8080.') }
    }
}

function Write-ProcessState($values) {
    New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
    $values | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $stateFile -Encoding UTF8
}

function Read-ProcessState {
    if (-not (Test-Path -LiteralPath $stateFile)) { return @() }
    try { return @(Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json) } catch { return @() }
}

function Start-NHProcess([string]$Name, [string]$FilePath, [string[]]$Arguments, [string]$WorkingDirectory) {
    New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
    $stdout = Join-Path $stateRoot ($Name + '.out.log')
    $stderr = Join-Path $stateRoot ($Name + '.err.log')
    $startParams = @{ FilePath = $FilePath; WorkingDirectory = $WorkingDirectory; RedirectStandardOutput = $stdout; RedirectStandardError = $stderr; WindowStyle = 'Hidden'; PassThru = $true }
    if ($Arguments.Count -gt 0) { $startParams.ArgumentList = $Arguments }
    $process = Start-Process @startParams
    return [pscustomobject]@{ name = $Name; pid = $process.Id; listener_pid = $null; port = $null; log = $stdout; error_log = $stderr }
}

function Get-NHListeningProcessIds([int]$Port) {
    return @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique)
}

function Assert-NHPortFree([int]$Port, [string]$ServiceName) {
    $owners = @(Get-NHListeningProcessIds $Port)
    if ($owners.Count -eq 0) { return }
    $descriptions = foreach ($ownerId in $owners) {
        $owner = Get-Process -Id ([int]$ownerId) -ErrorAction SilentlyContinue
        if ($owner) { $owner.ProcessName + ' PID=' + $owner.Id } else { 'PID=' + $ownerId }
    }
    throw ($ServiceName + ' cannot start because port ' + $Port + ' is already owned by ' + ($descriptions -join ', ') + '. Run -Action restart or stop that process explicitly.')
}

function Wait-NHListener([string]$Name, [int]$Port, [string]$ErrorLog, [int]$TimeoutSeconds = 30) {
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        $owners = @(Get-NHListeningProcessIds $Port)
        if ($owners.Count -eq 1) { return [int]$owners[0] }
        Start-Sleep -Milliseconds 250
    }
    $detail = ''
    if (Test-Path -LiteralPath $ErrorLog) {
        $detail = ((Get-Content -LiteralPath $ErrorLog -Tail 12 -ErrorAction SilentlyContinue) -join ' ')
    }
    throw ($Name + ' did not listen on port ' + $Port + ' within ' + $TimeoutSeconds + ' seconds. ' + $detail)
}

function Wait-NHHttp([string]$Name, [string]$Uri, [hashtable]$Headers = @{}, [int]$TimeoutSeconds = 60) {
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $lastError = ''
    while ($timer.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Uri -Headers $Headers -TimeoutSec 5
            if ([int]$response.StatusCode -eq 200) { return $response }
            $lastError = 'HTTP ' + [int]$response.StatusCode
        }
        catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Milliseconds 500
    }
    throw ($Name + ' did not become healthy at ' + $Uri + ' within ' + $TimeoutSeconds + ' seconds. Last error: ' + $lastError)
}

function Stop-NHProcessTree([int]$RootProcessId, [object[]]$ProcessTable) {
    $childrenByParent = @{}
    foreach ($item in $ProcessTable) {
        $parentId = [int]$item.ParentProcessId
        if (-not $childrenByParent.ContainsKey($parentId)) { $childrenByParent[$parentId] = @() }
        $childrenByParent[$parentId] += [int]$item.ProcessId
    }
    $pending = [Collections.Generic.Queue[int]]::new()
    $processIds = [Collections.Generic.List[int]]::new()
    $pending.Enqueue($RootProcessId)
    while ($pending.Count -gt 0) {
        $currentId = $pending.Dequeue()
        if ($processIds.Contains($currentId)) { continue }
        $processIds.Add($currentId)
        if ($childrenByParent.ContainsKey($currentId)) {
            foreach ($childId in $childrenByParent[$currentId]) { $pending.Enqueue($childId) }
        }
    }
    for ($index = $processIds.Count - 1; $index -ge 0; $index--) {
        Stop-Process -Id $processIds[$index] -Force -ErrorAction SilentlyContinue
    }
}

function Stop-NHProcesses {
    $processTable = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue)
    foreach ($record in (Read-ProcessState)) {
        $recordedIds = @($record.pid, $record.listener_pid) | Where-Object { $null -ne $_ -and [int]$_ -gt 0 } | Select-Object -Unique
        foreach ($recordedId in $recordedIds) {
            $process = Get-Process -Id ([int]$recordedId) -ErrorAction SilentlyContinue
            if ($process) { Stop-NHProcessTree ([int]$process.Id) $processTable }
        }
    }
    $webEntrypoints = @($processTable | Where-Object {
        $_.CommandLine -and
        $_.CommandLine -match [regex]::Escape((Join-Path $repoRoot 'apps\web')) -and
        $_.CommandLine -match '\bnext\b.*\bdev\b'
    })
    foreach ($entrypoint in $webEntrypoints) {
        Stop-NHProcessTree ([int]$entrypoint.ProcessId) $processTable
    }
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while ($timer.Elapsed.TotalSeconds -lt 5) {
        $ownedPorts = @(Read-ProcessState | ForEach-Object { $_.port } | Where-Object { $null -ne $_ })
        $stillListening = @($ownedPorts | Where-Object { @(Get-NHListeningProcessIds ([int]$_)).Count -gt 0 })
        if ($stillListening.Count -eq 0) { break }
        Start-Sleep -Milliseconds 250
    }
    if (Test-Path -LiteralPath $stateFile) { Remove-Item -LiteralPath $stateFile -Force }
}

function Start-NHStack {
    Set-AppDefaults
    Assert-NHPortFree 8080 'Product API'
    Assert-NHPortFree 3000 'Web application'
    & docker compose @composeArgs up -d
    if ($LASTEXITCODE -ne 0) { throw 'Infrastructure startup failed.' }
    $records = @()
    $go = (Get-Command go.exe -ErrorAction Stop).Source
    $uv = (Get-Command uv.exe -ErrorAction Stop).Source
    $pnpm = (Get-Command pnpm.cmd -ErrorAction Stop).Source
    $binaryRoot = Join-Path $stateRoot 'bin'
    New-Item -ItemType Directory -Force -Path $binaryRoot | Out-Null
    $apiBinary = Join-Path $binaryRoot 'nh-media-api.exe'
    $mediaBinary = Join-Path $binaryRoot 'nh-media-worker.exe'
    & $go build -o $apiBinary '.\services\api\cmd\api'
    if ($LASTEXITCODE -ne 0) { throw 'Product API build failed.' }
    & $go build -o $mediaBinary '.\services\media-worker\cmd\worker'
    if ($LASTEXITCODE -ne 0) { throw 'Go media worker build failed.' }
    try {
        $apiRecord = Start-NHProcess 'api' $apiBinary @() $repoRoot
        $records += $apiRecord
        Write-ProcessState $records
        $apiRecord.listener_pid = Wait-NHListener 'Product API' 8080 $apiRecord.error_log 30
        $apiRecord.port = 8080
        Write-ProcessState $records
        Wait-NHHttp 'Product API live check' 'http://127.0.0.1:8080/api/v1/live' @{} 30 | Out-Null
        Wait-NHHttp 'Product API readiness check' 'http://127.0.0.1:8080/api/v1/ready' @{} 30 | Out-Null
        if ($Profile -eq 'lan') {
            $corsResponse = Wait-NHHttp 'Packaged desktop CORS check' 'http://127.0.0.1:8080/api/v1/live' @{ Origin = 'http://tauri.localhost' } 30
            if ([string]$corsResponse.Headers['Access-Control-Allow-Origin'] -ne 'http://tauri.localhost') {
                throw 'Product API did not return the exact packaged desktop CORS origin.'
            }
        }

        $env:NH_MEDIA_REDIS_WORKER = '1'
        $env:NH_MEDIA_QUEUE_CAPABILITY = 'probe'
        $records += Start-NHProcess 'media-worker' $mediaBinary @() $repoRoot
        Write-ProcessState $records
        $env:NH_MEDIA_QUEUE_CAPABILITY = 'analysis'
        $records += Start-NHProcess 'ml-worker' $uv @('run', '--project', 'services/ml-worker', 'python', '-m', 'nh_media.worker') $repoRoot
        Write-ProcessState $records

        $webHost = if ($Profile -eq 'lan') { '0.0.0.0' } else { '127.0.0.1' }
        $webRoot = Join-Path $repoRoot 'apps/web'
        $nextCache = Join-Path $webRoot '.next'
        if (Test-Path -LiteralPath $nextCache) { Remove-Item -LiteralPath $nextCache -Recurse -Force }
        $webRecord = Start-NHProcess 'web' $pnpm @('dev', '--hostname', $webHost, '--port', '3000') $webRoot
        $records += $webRecord
        Write-ProcessState $records
        $webRecord.listener_pid = Wait-NHListener 'Web application' 3000 $webRecord.error_log 30
        $webRecord.port = 3000
        Write-ProcessState $records
        Wait-NHHttp 'Web application' 'http://127.0.0.1:3000' @{} 60 | Out-Null

        if ($IncludeTauri) {
            $tauri = Get-Command cargo-tauri.exe -ErrorAction SilentlyContinue
            if ($tauri) {
                $tauriArgs = @('dev', '--config', 'apps/desktop/src-tauri/tauri.conf.json')
            } else {
                $tauri = Get-Command cargo -ErrorAction Stop
                $tauriArgs = @('tauri', 'dev', '--config', 'apps/desktop/src-tauri/tauri.conf.json')
            }
            $records += Start-NHProcess 'tauri' $tauri.Source $tauriArgs $repoRoot
            Write-ProcessState $records
        }
    }
    catch {
        if ($records.Count -gt 0) { Write-ProcessState $records }
        Stop-NHProcesses
        throw
    }
    Write-ProcessState $records
    Write-Output ('NH-Media ' + $Profile + ' stack started. Process logs: ' + $stateRoot)
}

switch ($Action) {
    'config' { & docker compose @composeArgs config --quiet; exit $LASTEXITCODE }
    'infra-up' {
        Require-Env @('POSTGRES_DB', 'POSTGRES_SUPERUSER', 'POSTGRES_SUPERUSER_PASSWORD', 'POSTGRES_APP_USER', 'POSTGRES_APP_PASSWORD', 'POSTGRES_MIGRATION_USER', 'POSTGRES_MIGRATION_PASSWORD', 'REDIS_PASSWORD', 'MINIO_ROOT_USER', 'MINIO_ROOT_PASSWORD', 'MINIO_BUCKET')
        & docker compose @composeArgs up -d
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        & docker compose @composeArgs ps
    }
    'infra-down' { & docker compose @composeArgs down; exit $LASTEXITCODE }
    'infra-restart' { & docker compose @composeArgs restart; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; & docker compose @composeArgs ps }
    'status' {
        & docker compose @composeArgs ps
        foreach ($record in (Read-ProcessState)) {
            $running = $null -ne (Get-Process -Id ([int]$record.pid) -ErrorAction SilentlyContinue)
            Write-Output ($record.name + [char]9 + 'PID=' + $record.pid + [char]9 + 'Running=' + $running + [char]9 + 'Log=' + $record.log)
        }
    }
    'api' {
        Set-AppDefaults
        Push-Location $repoRoot
        try { & go run .\services\api\cmd\api; exit $LASTEXITCODE } finally { Pop-Location }
    }
    'start' { Start-NHStack }
    'stop' { Stop-NHProcesses; Write-Output 'NH-Media application processes stopped; infrastructure volumes were preserved.' }
    'restart' { Stop-NHProcesses; Start-NHStack }
}
