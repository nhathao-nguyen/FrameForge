[CmdletBinding()]
param(
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'local',
    [ValidateSet('config', 'infra-up', 'infra-down', 'infra-restart', 'status', 'api')]
    [string]$Action = 'status'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'infrastructure/compose/docker-compose.yml'

function Require-Env([string[]]$Names) {
    foreach ($name in $Names) {
        $value = [Environment]::GetEnvironmentVariable($name)
        if ([string]::IsNullOrWhiteSpace($value) -or $value -like 'CHANGE_ME*') {
            throw "$name must be set to a non-default value before starting NH-Media infrastructure. See .env.example."
        }
    }
}

$required = @(
    'POSTGRES_DB', 'POSTGRES_SUPERUSER', 'POSTGRES_SUPERUSER_PASSWORD', 'POSTGRES_APP_USER',
    'POSTGRES_APP_PASSWORD', 'POSTGRES_MIGRATION_USER', 'POSTGRES_MIGRATION_PASSWORD',
    'REDIS_PASSWORD', 'MINIO_ROOT_USER', 'MINIO_ROOT_PASSWORD', 'MINIO_BUCKET'
)
if ($Action -in @('infra-up', 'infra-restart')) { Require-Env $required }
if ($Profile -eq 'lan' -and $Action -eq 'api' -and [string]::IsNullOrWhiteSpace($env:NH_API_BIND)) {
    throw 'LAN API startup requires an explicit NH_API_BIND; do not hard-code a LAN address.'
}

$env:NH_MEDIA_PROFILE = $Profile
$env:COMPOSE_PROFILES = $Profile
$composeArgs = @('--file', $composeFile, '--profile', $Profile)

switch ($Action) {
    'config' { & docker compose @composeArgs config --quiet }
    'infra-up' { & docker compose @composeArgs up -d; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; & docker compose @composeArgs ps }
    'infra-down' { & docker compose @composeArgs down }
    'infra-restart' { & docker compose @composeArgs restart; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; & docker compose @composeArgs ps }
    'status' { & docker compose @composeArgs ps }
    'api' {
        Push-Location $repoRoot
        try { & go run .\services\api\cmd\api; exit $LASTEXITCODE } finally { Pop-Location }
    }
}
