[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$EvidenceInput = 'docs/evidence/real-local-gate-h-live-20260819-r37.json',
    [string]$EvidenceOutput = 'docs/evidence/real-local-service-restart-matrix-20260819.json',
    [string]$EnvFile = '',
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'lan',
    [string]$LanServerIp = '192.168.1.19'
)

$ErrorActionPreference = 'Stop'
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'
$composeFile = Join-Path $PSScriptRoot '..\infrastructure\compose\docker-compose.yml'

function Import-EnvFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { throw 'Invalid environment line.' }
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) { $value = $value.Substring(1, $value.Length - 2) }
        Set-Item -Path ('Env:' + $Matches[1]) -Value $value
    }
}

function Invoke-Api([string]$Method, [string]$Path, [string]$Token = '') {
    $headers = @{}
    if ($Token -ne '') { $headers.Authorization = 'Bearer ' + $Token }
    Invoke-RestMethod -Method $Method -Uri ($apiRoot + $Path) -Headers $headers -TimeoutSec 30
}

function Wait-Ready([int]$TimeoutSeconds = 60) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            if ((Invoke-WebRequest -UseBasicParsing -Uri ($apiRoot + '/ready') -TimeoutSec 5).StatusCode -eq 200) { return $true }
        } catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Get-DurableState([string]$ProjectId, [string]$JobId, [string]$Token) {
    $job = Invoke-Api 'GET' ('/projects/' + $ProjectId + '/jobs/' + $JobId) $Token
    return [ordered]@{ job_id = $JobId; status = [string]$job.status; completed = ([string]$job.status -eq 'completed') }
}

function Wait-ContainerHealthy([string]$Service, [int]$TimeoutSeconds = 90) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $container = (& docker compose --file $composeFile --profile local ps -q $Service 2>$null | Select-Object -First 1).Trim()
        if ($container) {
            $state = (& docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' $container 2>$null | Select-Object -First 1).Trim()
            if ($state -match '^running\|(healthy|none)$') { return $state }
        }
        Start-Sleep -Milliseconds 500
    }
    throw ($Service + ' did not become healthy in time.')
}

Import-EnvFile $EnvFile
if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME) -or [string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) { throw 'LocalAuth environment is required.' }
if (-not (Test-Path -LiteralPath $EvidenceInput -PathType Leaf)) { throw 'Input evidence was not found.' }
$input = Get-Content -Raw -LiteralPath $EvidenceInput | ConvertFrom-Json
$projectId = [string]$input.project_id
$login = Invoke-RestMethod -Method Post -Uri ($apiRoot + '/auth/local/login') -ContentType 'application/json' -Body (@{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD } | ConvertTo-Json -Compress)
$token = [string]$login.access_token
if ([string]::IsNullOrWhiteSpace($token)) { throw 'LocalAuth token was not returned.' }

$observations = [System.Collections.Generic.List[object]]::new()
$jobIds = @([string]$input.validation_job_id, [string]$input.movie_job_id, [string]$input.render_job_id) | Where-Object { $_ -ne '' }
foreach ($service in @('postgres', 'redis', 'minio')) {
    & docker compose --file $composeFile --profile local restart $service
    if ($LASTEXITCODE -ne 0) { throw ('Docker restart failed for ' + $service) }
    $containerState = Wait-ContainerHealthy $service
    $ready = Wait-Ready
    $rows = @($jobIds | ForEach-Object { Get-DurableState $projectId $_ $token })
    $observations.Add([ordered]@{ service = $service; restart_command = 'docker compose restart ' + $service; container_state = $containerState; api_ready = $ready; durable_jobs = $rows; all_jobs_completed = (@($rows | Where-Object { -not $_.completed }).Count -eq 0) })
    if (-not $ready -or @($rows | Where-Object { -not $_.completed }).Count -gt 0) { throw ('Durable state was not healthy after ' + $service + ' restart.') }
}

$devScript = Join-Path $PSScriptRoot 'dev.ps1'
$restartArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $devScript, '-Profile', $Profile, '-Action', 'restart')
if ($EnvFile) { $restartArgs += @('-EnvFile', $EnvFile) }
if ($Profile -eq 'lan') { $restartArgs += @('-LanServerIp', $LanServerIp) }
& powershell.exe @restartArgs
if ($LASTEXITCODE -ne 0) { throw 'Tracked application process restart failed.' }
$appReady = Wait-Ready
$login = Invoke-RestMethod -Method Post -Uri ($apiRoot + '/auth/local/login') -ContentType 'application/json' -Body (@{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD } | ConvertTo-Json -Compress)
$token = [string]$login.access_token
$appRows = @($jobIds | ForEach-Object { Get-DurableState $projectId $_ $token })
$observations.Add([ordered]@{ service = 'api-go-worker-python-worker-web'; restart_command = 'tools/dev.ps1 -Action restart'; container_state = ''; api_ready = $appReady; durable_jobs = $appRows; all_jobs_completed = (@($appRows | Where-Object { -not $_.completed }).Count -eq 0) })

$report = [ordered]@{
    schema_version = 'nh-media/real-local-service-restart-matrix/v1'
    evidence_input = [IO.Path]::GetFullPath($EvidenceInput)
    project_id = $projectId
    services = @($observations)
    status = if (@($observations | Where-Object { -not $_.api_ready -or -not $_.all_jobs_completed }).Count -eq 0) { 'PASS' } else { 'NOT_PASS' }
    limitations = @('Runs restarts between completed durable jobs; it proves readiness and state preservation, not power-loss chaos or in-flight full-AI interruption.', 'Physical second-device LAN workflow is separate evidence.')
    finished_at = (Get-Date).ToUniversalTime().ToString('o')
}
$parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$report | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
Get-Content -Raw -LiteralPath $EvidenceOutput
