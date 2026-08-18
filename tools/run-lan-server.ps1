[CmdletBinding()]
param(
    [string]$ServerLanIp = '192.168.1.18',
    [string]$EnvFile = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$devScript = Join-Path $repoRoot 'tools\dev.ps1'
$exitCode = 0
$secure = $null
$passwordPtr = [IntPtr]::Zero

function Import-EnvFile([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw ('Environment file was not found: ' + $Path) }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { throw ('Invalid environment line in ' + $Path) }
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        Set-Item -Path ('Env:' + $Matches[1]) -Value $value
    }
}

function Get-PnpmPath {
    foreach ($name in @('pnpm.cmd', 'pnpm')) {
        $command = Get-Command $name -ErrorAction SilentlyContinue
        if ($null -ne $command -and (Test-Path -LiteralPath $command.Source -PathType Leaf)) { return $command.Source }
    }
    foreach ($candidate in @(
        (Join-Path $env:USERPROFILE '.cache\codex-runtimes\codex-primary-runtime\dependencies\bin\fallback\pnpm.cmd'),
        (Join-Path $env:USERPROFILE 'AppData\Local\pnpm\pnpm.cmd'),
        (Join-Path $env:USERPROFILE 'AppData\Roaming\npm\pnpm.cmd')
    )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return (Resolve-Path -LiteralPath $candidate).Path }
    }
    throw 'pnpm.cmd was not found. Rerun with pnpm installed or add it to PATH.'
}

function Get-ContainerEnvironment([string]$Service) {
    $containerId = (& docker ps -aq --filter ('label=com.docker.compose.service=' + $Service) 2>$null | Select-Object -First 1)
    if ([string]::IsNullOrWhiteSpace($containerId)) { throw ('Running Docker container for service ' + $Service + ' was not found.') }
    $inspect = (& docker inspect $containerId 2>$null | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0 -or $null -eq $inspect) { throw ('Could not inspect Docker service ' + $Service + '.') }
    $values = @{}
    foreach ($entry in @($inspect[0].Config.Env)) {
        $parts = $entry -split '=', 2
        if ($parts.Count -eq 2) { $values[$parts[0]] = $parts[1] }
    }
    return $values
}

function Set-InfraEnvironmentFromDocker {
    $postgres = Get-ContainerEnvironment 'postgres'
    $redis = Get-ContainerEnvironment 'redis'
    $minio = Get-ContainerEnvironment 'minio'

    $env:POSTGRES_DB = $postgres.POSTGRES_DB
    $env:POSTGRES_SUPERUSER = $postgres.POSTGRES_USER
    $env:POSTGRES_SUPERUSER_PASSWORD = $postgres.POSTGRES_PASSWORD
    $env:POSTGRES_APP_USER = $postgres.POSTGRES_APP_USER
    $env:POSTGRES_APP_PASSWORD = $postgres.POSTGRES_APP_PASSWORD
    $env:POSTGRES_MIGRATION_USER = $postgres.POSTGRES_MIGRATION_USER
    $env:POSTGRES_MIGRATION_PASSWORD = $postgres.POSTGRES_MIGRATION_PASSWORD
    $env:REDIS_PASSWORD = $redis.REDIS_PASSWORD
    $env:MINIO_ROOT_USER = $minio.MINIO_ROOT_USER
    $env:MINIO_ROOT_PASSWORD = $minio.MINIO_ROOT_PASSWORD
    if ([string]::IsNullOrWhiteSpace($env:MINIO_BUCKET)) { $env:MINIO_BUCKET = 'nh-media-artifacts' }
}

try {
    Set-Location -LiteralPath $repoRoot
    $pnpmPath = Get-PnpmPath
    $env:Path = (Split-Path -Parent $pnpmPath) + ';' + $env:Path

    if ([string]::IsNullOrWhiteSpace($EnvFile)) {
        Set-InfraEnvironmentFromDocker
    } else {
        Import-EnvFile (Resolve-Path -LiteralPath $EnvFile).Path
    }

    if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME)) { $env:NH_API_ADMIN_USERNAME = 'admin' }
    if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) {
        $secure = Read-Host 'NH-Media LocalAuth admin password' -AsSecureString
        $passwordPtr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
        $env:NH_API_ADMIN_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPtr)
    }
    $env:NH_API_BIND = '0.0.0.0:8080'
    $env:NH_API_ALLOWED_ORIGINS = 'http://' + $ServerLanIp + ':3000,http://tauri.localhost'
    $env:NEXT_PUBLIC_NH_MEDIA_API_URL = 'http://' + $ServerLanIp + ':8080'

    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $devScript -Profile lan -LanServerIp $ServerLanIp -Action restart
    if ($LASTEXITCODE -ne 0) { throw 'LAN server startup failed.' }
    Write-Output ('LAN server started at http://' + $ServerLanIp + ':8080 and http://' + $ServerLanIp + ':3000')
}
catch {
    $exitCode = 1
    Write-Host ('LAN server FAILED: ' + $_.Exception.Message) -ForegroundColor Red
}
finally {
    if ($passwordPtr -ne [IntPtr]::Zero) { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPtr) }
    if ($null -ne $secure) { $secure.Dispose() }
    foreach ($name in @('POSTGRES_DB','POSTGRES_SUPERUSER','POSTGRES_SUPERUSER_PASSWORD','POSTGRES_APP_USER','POSTGRES_APP_PASSWORD','POSTGRES_MIGRATION_USER','POSTGRES_MIGRATION_PASSWORD','REDIS_PASSWORD','MINIO_ROOT_USER','MINIO_ROOT_PASSWORD','MINIO_BUCKET','NH_API_ADMIN_USERNAME','NH_API_ADMIN_PASSWORD','NH_MEDIA_PROFILE','NH_API_BIND','NH_API_ALLOWED_ORIGINS','NH_MEDIA_DATABASE_URL','NH_STORAGE_ENDPOINT','NH_STORAGE_ACCESS_KEY','NH_STORAGE_SECRET_KEY','NH_STORAGE_BUCKET','NH_QUEUE_ENDPOINT','NH_MEDIA_WORKER_QUEUE_ENDPOINT','NH_QUEUE_PASSWORD','NEXT_PUBLIC_NH_MEDIA_API_URL')) {
        Remove-Item ('Env:' + $name) -ErrorAction SilentlyContinue
    }
}

if ($exitCode -ne 0) {
    Read-Host 'Press Enter to close this window'
    exit $exitCode
}
