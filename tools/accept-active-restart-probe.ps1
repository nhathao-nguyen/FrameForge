[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$EvidenceInput = 'docs/evidence/real-local-gate-h-live-20260819-r38.json',
    [string]$EvidenceOutput = 'docs/evidence/real-local-active-restart-probe-20260819.json',
    [string]$ProviderPolicyPath = 'tools/provider-policy-real-local.json',
    [string]$EnvFile = '',
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'lan',
    [string]$LanServerIp = '192.168.1.19',
    [int]$TimeoutSeconds = 420
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

function Invoke-Api([string]$Method, [string]$Path, [string]$Token = '', [object]$Body = $null) {
    $headers = @{}
    if ($Token -ne '') { $headers.Authorization = 'Bearer ' + $Token }
    $params = @{ Method = $Method; Uri = $apiRoot + $Path; Headers = $headers; ErrorAction = 'Stop'; TimeoutSec = 30 }
    if ($null -ne $Body) {
        $params.ContentType = 'application/json'
        $params.Body = ($Body | ConvertTo-Json -Depth 30 -Compress)
    }
    Invoke-RestMethod @params
}

function Wait-Ready([int]$Timeout = 90) {
    $deadline = [DateTime]::UtcNow.AddSeconds($Timeout)
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            if ((Invoke-WebRequest -UseBasicParsing -Uri ($apiRoot + '/ready') -TimeoutSec 5).StatusCode -eq 200) { return $true }
        } catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Wait-ContainerHealthy([string]$Service, [int]$Timeout = 90) {
    $deadline = [DateTime]::UtcNow.AddSeconds($Timeout)
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

function Get-JobStatus([string]$ProjectId, [string]$JobId, [string]$Token) {
    return [string](Invoke-Api 'GET' ('/projects/' + $ProjectId + '/jobs/' + $JobId) $Token).status
}

function Is-Active([string]$Status) { return $Status -in @('created', 'queued', 'running', 'retrying') }

Import-EnvFile $EnvFile
if (-not (Test-Path -LiteralPath $EvidenceInput -PathType Leaf)) { throw 'Input evidence was not found.' }
if (-not (Test-Path -LiteralPath $ProviderPolicyPath -PathType Leaf)) { throw 'Provider policy was not found.' }
$input = Get-Content -Raw -LiteralPath $EvidenceInput | ConvertFrom-Json
$policy = Get-Content -Raw -LiteralPath $ProviderPolicyPath | ConvertFrom-Json
$projectId = [string]$input.project_id
$source = @($input.artifact_refs | Where-Object { [string]$_.role -eq 'source_original' } | Select-Object -First 1)
if ($source.Count -ne 1) { throw 'Input evidence did not contain one source_original Artifact ref.' }

$login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
$token = [string]$login.access_token
if ([string]::IsNullOrWhiteSpace($token)) { throw 'LocalAuth token was not returned.' }
$job = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs') $token @{
    kind = 'pipeline'
    workflow_key = 'movie_recap'
    mode = 'automatic'
    auto_start = $true
    input = @{ artifacts = @(@{ artifact_id = [string]$source.artifact_id; role = 'source_original'; sha256 = [string]$source.sha256 }) }
    params = @{ duration_sec = 3; language = 'en'; provider_policy = $policy }
}
$jobId = [string]$job.id
if ([string]::IsNullOrWhiteSpace($jobId)) { throw 'Active restart probe did not create a Job.' }

$observations = [System.Collections.Generic.List[object]]::new()
$activeDeadline = [DateTime]::UtcNow.AddSeconds(45)
$status = Get-JobStatus $projectId $jobId $token
while (-not (Is-Active $status) -and [DateTime]::UtcNow -lt $activeDeadline) {
    Start-Sleep -Milliseconds 250
    $status = Get-JobStatus $projectId $jobId $token
}

foreach ($service in @('postgres', 'redis', 'minio')) {
    $before = Get-JobStatus $projectId $jobId $token
    & docker compose --file $composeFile --profile local restart $service | Out-Null
    if ($LASTEXITCODE -ne 0) { throw ('Docker restart failed for ' + $service) }
    $containerState = Wait-ContainerHealthy $service
    $apiReady = Wait-Ready
    $after = Get-JobStatus $projectId $jobId $token
    $observations.Add([ordered]@{ service = $service; restart_command = 'docker compose restart ' + $service; job_status_before = $before; active_before_restart = (Is-Active $before); container_state = $containerState; api_ready = $apiReady; job_status_after = $after })
}

$beforeApp = Get-JobStatus $projectId $jobId $token
$devScript = Join-Path $PSScriptRoot 'dev.ps1'
$restartArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $devScript, '-Profile', $Profile, '-Action', 'restart')
if ($EnvFile) { $restartArgs += @('-EnvFile', $EnvFile) }
if ($Profile -eq 'lan') { $restartArgs += @('-LanServerIp', $LanServerIp) }
$restartLogBase = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-active-restart-' + [guid]::NewGuid().ToString())
$restartProcess = Start-Process -FilePath ((Get-Command powershell.exe -ErrorAction Stop).Source) -ArgumentList $restartArgs -WindowStyle Hidden -RedirectStandardOutput ($restartLogBase + '.out.log') -RedirectStandardError ($restartLogBase + '.err.log') -PassThru
if (-not $restartProcess.WaitForExit(180000)) {
    Stop-Process -Id $restartProcess.Id -Force -ErrorAction SilentlyContinue
    throw 'Tracked application process restart did not exit within three minutes.'
}
$restartProcess.Refresh()
$apiReadyApp = Wait-Ready
$restartExitCode = [int]$restartProcess.ExitCode
if ($restartExitCode -ne 0 -and -not $apiReadyApp) { throw ('Tracked application process restart failed with exit code ' + $restartExitCode + ' and API readiness was not restored.') }
$login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
$token = [string]$login.access_token
$afterApp = Get-JobStatus $projectId $jobId $token
$observations.Add([ordered]@{ service = 'api-go-worker-python-worker-web'; restart_command = 'tools/dev.ps1 -Action restart'; restart_process_exit_code = $restartExitCode; job_status_before = $beforeApp; active_before_restart = (Is-Active $beforeApp); container_state = ''; api_ready = $apiReadyApp; job_status_after = $afterApp })

$deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$terminal = Get-JobStatus $projectId $jobId $token
while ($terminal -notin @('completed', 'failed', 'dead_lettered', 'cancelled') -and [DateTime]::UtcNow -lt $deadline) {
    Start-Sleep -Seconds 1
    $terminal = Get-JobStatus $projectId $jobId $token
}
$activeObserved = @($observations | Where-Object { $_.active_before_restart }).Count
$report = [ordered]@{
    schema_version = 'nh-media/real-local-active-restart-probe/v1'
    evidence_input = [IO.Path]::GetFullPath($EvidenceInput)
    project_id = $projectId
    job_id = $jobId
    services = @($observations)
    active_restart_count = $activeObserved
    terminal_status = $terminal
    all_restarts_observed_active = ($activeObserved -eq @($observations).Count)
    status = if ($activeObserved -eq @($observations).Count -and $terminal -eq 'completed') { 'PASS' } else { 'NOT_PASS' }
    limitations = @('The application process entry is restarted as one tracked API/Go/Python/web set; individual process isolation is not claimed.', 'This probe does not simulate power loss or a worker crash after staged output before commit.')
    finished_at = (Get-Date).ToUniversalTime().ToString('o')
}
$parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$report | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
Get-Content -Raw -LiteralPath $EvidenceOutput
if ($report.status -ne 'PASS') { exit 1 }
