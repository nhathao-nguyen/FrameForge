[CmdletBinding()]
param([switch]$SkipDesktop)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    function Assert-NativeSuccess([string]$Name) {
        if ($LASTEXITCODE -ne 0) { throw ($Name + ' failed with exit code ' + $LASTEXITCODE) }
    }

    $env:POSTGRES_DB = 'nh_media'
    $env:POSTGRES_SUPERUSER = 'nh_media_admin'
    $env:POSTGRES_SUPERUSER_PASSWORD = 'GateB_Test_Postgres_Admin_2026'
    $env:POSTGRES_APP_USER = 'nh_media_app'
    $env:POSTGRES_APP_PASSWORD = 'GateB_Test_Postgres_App_2026'
    $env:POSTGRES_MIGRATION_USER = 'nh_media_migrator'
    $env:POSTGRES_MIGRATION_PASSWORD = 'GateB_Test_Postgres_Migrator_2026'
    $env:REDIS_PASSWORD = 'GateB_Test_Redis_2026'
    $env:MINIO_ROOT_USER = 'nh_media_storage'
    $env:MINIO_ROOT_PASSWORD = 'GateB_Test_MinIO_2026'
    $env:MINIO_BUCKET = 'nh-media-artifacts'

    gofmt -l packages services | ForEach-Object { if ($_ ) { throw "gofmt required: $_" } }
    go vet ./...
    Assert-NativeSuccess 'go vet'
    go test ./...
    Assert-NativeSuccess 'go test'
    uv lock --project services/ml-worker --check
    Assert-NativeSuccess 'uv lock --check'
    uv run --project services/ml-worker ruff check services/ml-worker/nh_media services/ml-worker/tests
    Assert-NativeSuccess 'Ruff'
    uv run --project services/ml-worker mypy services/ml-worker/nh_media services/ml-worker/tests
    Assert-NativeSuccess 'mypy'
    uv run --project services/ml-worker bandit -q -r services/ml-worker/nh_media
    Assert-NativeSuccess 'Bandit'
    uv run --project services/ml-worker pytest
    Assert-NativeSuccess 'Python pytest'
    python tools/validate_contracts.py
    Assert-NativeSuccess 'contract validation'
    pnpm typecheck
    Assert-NativeSuccess 'pnpm typecheck'
    if (-not $SkipDesktop) {
        cargo fmt --manifest-path apps/desktop/src-tauri/Cargo.toml -- --check
        Assert-NativeSuccess 'cargo fmt --check'
        cargo check --manifest-path apps/desktop/src-tauri/Cargo.toml
        Assert-NativeSuccess 'cargo check'
    }
    & (Join-Path $PSScriptRoot 'verify-independence.ps1')
    & (Join-Path $PSScriptRoot 'verify-secrets.ps1')
    & (Join-Path $PSScriptRoot 'check-supply-chain.ps1')
    docker compose --file infrastructure/compose/docker-compose.yml --profile local config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'compose config failed' }
    git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }
    Write-Output 'NH-Media Gate B local verification baseline: pass'
} finally { Pop-Location }
