[CmdletBinding()]
param(
    [string]$FfmpegPath = '',
    [string]$FfprobePath = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $toolchainRoot = Join-Path $env:LOCALAPPDATA 'NH-Media\toolchains'
    $uv = Join-Path $toolchainRoot 'uv-0.11.28\uv.exe'
    $node = Join-Path $toolchainRoot 'node-v24.14.1-win-x64\node.exe'
    $pnpmShim = (Get-Command pnpm.cmd -ErrorAction SilentlyContinue).Source
    if (-not (Test-Path -LiteralPath $uv)) { throw 'Pinned uv 0.11.28 is not installed; run the dependency preflight first.' }
    if (-not (Test-Path -LiteralPath $node)) { throw 'Pinned Node 24.14.1 is not installed; run the dependency preflight first.' }
    if ([string]::IsNullOrWhiteSpace($pnpmShim)) { throw 'pnpm.cmd is required.' }
    $pnpmScript = (Resolve-Path (Join-Path (Split-Path $pnpmShim) '..\..\node\node_modules\pnpm\bin\pnpm.mjs')).Path
    if ([string]::IsNullOrWhiteSpace($FfmpegPath)) { $FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH }
    if ([string]::IsNullOrWhiteSpace($FfprobePath)) { $FfprobePath = $env:NH_MEDIA_FFPROBE_PATH }
    if ([string]::IsNullOrWhiteSpace($FfmpegPath) -or [string]::IsNullOrWhiteSpace($FfprobePath)) { throw 'Reviewed FFmpeg/ffprobe paths are required.' }
    $expectedFfmpeg = 'AF9E7AF850346AE908745F6191CDCF9581889E915F8FE2DD6AB8CABEF21161D9'
    $expectedFfprobe = '6D2B7AC8CD07DA82F066994BE8BCF0C74AB1ABA7F248BB75025FCC543711181A'
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $FfmpegPath).Hash -ne $expectedFfmpeg) { throw 'ffmpeg SHA-256 does not match the reviewed T002 pin.' }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $FfprobePath).Hash -ne $expectedFfprobe) { throw 'ffprobe SHA-256 does not match the reviewed T002 pin.' }
    $env:NH_MEDIA_FFMPEG_PATH = $FfmpegPath
    $env:NH_MEDIA_FFPROBE_PATH = $FfprobePath

    function Invoke-Checked([string]$FilePath, [string[]]$Arguments) {
        & $FilePath @Arguments
        if ($LASTEXITCODE -ne 0) { throw "$FilePath exited with code $LASTEXITCODE" }
    }

    Write-Output 'Gate G: Python lock/tests/lint/type/security'
    Invoke-Checked $uv @('lock', '--project', 'services/ml-worker', '--check')
    Invoke-Checked $uv @('sync', '--frozen', '--project', 'services/ml-worker')
    Invoke-Checked $uv @('run', '--project', 'services/ml-worker', 'pytest')
    Invoke-Checked $uv @('run', '--project', 'services/ml-worker', 'ruff', 'check', 'services/ml-worker/nh_media', 'services/ml-worker/tests')
    Invoke-Checked $uv @('run', '--project', 'services/ml-worker', 'mypy', 'services/ml-worker/nh_media', 'services/ml-worker/tests')
    Invoke-Checked $uv @('run', '--project', 'services/ml-worker', 'bandit', '-q', '-r', 'services/ml-worker/nh_media')

    Write-Output 'Gate G: Go contracts/render/regression'
    $gofmtOutput = @(gofmt -l services/media-worker/internal/render)
    if ($gofmtOutput.Count -gt 0) { throw ('gofmt required: ' + ($gofmtOutput -join ', ')) }
    Invoke-Checked go.exe @('vet', './...')
    Invoke-Checked go.exe @('test', './...')
    Invoke-Checked go.exe @('test', '-run', 'TestRenderRealMediaAndReuseThreeProfiles|TestRenderRealMultiClipMovieWithAudioAndSubtitles|TestInspectRejectsMissingVideo', './services/media-worker/internal/render', '-count=1')

    Write-Output 'Gate G: exact Node/pnpm client checks'
    Invoke-Checked $node @($pnpmScript, '--recursive', '--if-present', 'run', 'typecheck')
    Invoke-Checked $uv @('run', '--project', 'services/ml-worker', 'python', 'tools/validate_contracts.py')

    Write-Output 'Gate G: independence/secrets/supply-chain/compose'
    Invoke-Checked powershell.exe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'verify-independence.ps1'))
    Invoke-Checked powershell.exe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'verify-secrets.ps1'))
    Invoke-Checked powershell.exe @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'check-supply-chain.ps1'))
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
    Invoke-Checked docker.exe @('compose', '--file', 'infrastructure/compose/docker-compose.yml', '--profile', 'local', 'config', '--quiet')
    git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }
    Write-Output 'GATE G local acceptance checks: PASS'
} finally {
    Pop-Location
}
