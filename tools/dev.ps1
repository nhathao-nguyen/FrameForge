[CmdletBinding()]
param(
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'local',
    [ValidateSet('config', 'infra-up', 'infra-down', 'infra-restart', 'status', 'api', 'start', 'stop', 'restart')]
    [string]$Action = 'status',
    [string]$EnvFile = '',
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
        $env:NH_API_ALLOWED_ORIGINS = if ($Profile -eq 'lan') { 'http://localhost:3000' } else { 'http://127.0.0.1:3000,http://localhost:3000' }
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
    if ([string]::IsNullOrWhiteSpace($env:NEXT_PUBLIC_NH_MEDIA_API_URL)) { $env:NEXT_PUBLIC_NH_MEDIA_API_URL = 'http://127.0.0.1:8080' }
    if ($Profile -eq 'lan' -and [string]::IsNullOrWhiteSpace($env:NH_API_BIND)) { throw 'LAN profile requires an explicit NH_API_BIND.' }
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
    $existing = Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($FilePath)) -ErrorAction SilentlyContinue | Where-Object { $_.Id -in @(Read-ProcessState | ForEach-Object { $_.pid }) }
    if ($existing) { return [pscustomobject]@{ name = $Name; pid = $existing[0].Id; log = '' } }
    New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null
    $stdout = Join-Path $stateRoot ($Name + '.out.log')
    $stderr = Join-Path $stateRoot ($Name + '.err.log')
    $startParams = @{ FilePath = $FilePath; WorkingDirectory = $WorkingDirectory; RedirectStandardOutput = $stdout; RedirectStandardError = $stderr; WindowStyle = 'Hidden'; PassThru = $true }
    if ($Arguments.Count -gt 0) { $startParams.ArgumentList = $Arguments }
    $process = Start-Process @startParams
    return [pscustomobject]@{ name = $Name; pid = $process.Id; log = $stdout }
}

function Stop-NHProcesses {
    foreach ($record in (Read-ProcessState)) {
        $process = Get-Process -Id ([int]$record.pid) -ErrorAction SilentlyContinue
        if ($process) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
    }
    if (Test-Path -LiteralPath $stateFile) { Remove-Item -LiteralPath $stateFile -Force }
}

function Start-NHStack {
    Set-AppDefaults
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
    $records += Start-NHProcess 'api' $apiBinary @() $repoRoot
    $env:NH_MEDIA_REDIS_WORKER = '1'
    $env:NH_MEDIA_QUEUE_CAPABILITY = 'probe'
    $records += Start-NHProcess 'media-worker' $mediaBinary @() $repoRoot
    $env:NH_MEDIA_QUEUE_CAPABILITY = 'analysis'
    $records += Start-NHProcess 'ml-worker' $uv @('run', '--project', 'services/ml-worker', 'python', '-m', 'nh_media.worker') $repoRoot
    $webHost = if ($Profile -eq 'lan') { '0.0.0.0' } else { '127.0.0.1' }
    $webRoot = Join-Path $repoRoot 'apps/web'
    $records += Start-NHProcess 'web' $pnpm @('dev', '--hostname', $webHost) $webRoot
    if ($IncludeTauri) {
        $tauri = Get-Command cargo-tauri.exe -ErrorAction SilentlyContinue
        if ($tauri) {
            $tauriArgs = @('dev', '--config', 'apps/desktop/src-tauri/tauri.conf.json')
        } else {
            $tauri = Get-Command cargo -ErrorAction Stop
            $tauriArgs = @('tauri', 'dev', '--config', 'apps/desktop/src-tauri/tauri.conf.json')
        }
        $records += Start-NHProcess 'tauri' $tauri.Source $tauriArgs $repoRoot
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
