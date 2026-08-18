[CmdletBinding()]
param(
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'local',
    [string]$ApiBaseUrl = '',
    [string]$LanBaseUrl = '',
    [string]$LanWebBaseUrl = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) { $ApiBaseUrl = 'http://127.0.0.1:8080' }
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'
$results = [ordered]@{}

function Invoke-Api([string]$Method, [string]$Path, [string]$Token = '', [object]$Body = $null, [hashtable]$ExtraHeaders = @{}) {
    $headers = @{}
    if ($Token -ne '') { $headers.Authorization = 'Bearer ' + $Token }
    foreach ($key in $ExtraHeaders.Keys) { $headers[$key] = $ExtraHeaders[$key] }
    $params = @{ Method = $Method; Uri = $apiRoot + $Path; Headers = $headers; ErrorAction = 'Stop' }
    if ($null -ne $Body) { $params.ContentType = 'application/json'; $params.Body = ($Body | ConvertTo-Json -Depth 10 -Compress) }
    return Invoke-RestMethod @params
}

function Read-SseFrame([string]$Project, [string]$Job, [int64]$LastEventId, [string]$Token) {
    Add-Type -AssemblyName System.Net.Http -ErrorAction Stop
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $handler.UseProxy = $false
    $http = [System.Net.Http.HttpClient]::new($handler)
    $http.Timeout = [TimeSpan]::FromSeconds(15)
    $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Get, ($apiRoot + '/projects/' + $Project + '/jobs/' + $Job + '/events/stream'))
    $request.Headers.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer', $Token)
    $request.Headers.Add('Last-Event-ID', [string]$LastEventId)
    $request.Headers.Accept.Add([System.Net.Http.Headers.MediaTypeWithQualityHeaderValue]::new('text/event-stream'))
    $response = $null
    $reader = $null
    try {
        $response = $http.SendAsync($request, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
        if (-not $response.IsSuccessStatusCode) { throw ('SSE returned HTTP ' + [int]$response.StatusCode) }
        $reader = [IO.StreamReader]::new($response.Content.ReadAsStreamAsync().GetAwaiter().GetResult())
        $lines = [Collections.Generic.List[string]]::new()
        for ($index = 0; $index -lt 16; $index++) {
            $readTask = $reader.ReadLineAsync()
            if (-not $readTask.Wait(15000)) { throw 'SSE frame read timed out.' }
            $line = $readTask.GetAwaiter().GetResult()
            if ($null -eq $line) { break }
            [void]$lines.Add($line)
            if ($line -eq '') { break }
        }
        return ($lines -join "`n")
    } finally {
        if ($null -ne $reader) { $reader.Dispose() }
        if ($null -ne $response) { $response.Dispose() }
        if ($null -ne $request) { $request.Dispose() }
        $http.Dispose()
    }
}

try {
    $results.live = (Invoke-WebRequest -UseBasicParsing ($apiRoot + '/live')).StatusCode -eq 200
    $results.ready = (Invoke-WebRequest -UseBasicParsing ($apiRoot + '/ready')).StatusCode -eq 200
    $login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
    $token = $login.access_token
    if ([string]::IsNullOrWhiteSpace($token)) { throw 'LocalAuth did not return an access token.' }
    $results.local_auth = $true
    $session = Invoke-Api 'GET' '/auth/session' $token
    $results.default_workspace = $null -ne $session.memberships
    $project = Invoke-Api 'POST' '/projects' $token @{ name = 'local-acceptance-' + (Get-Date -Format 'yyyyMMddHHmmss'); workflow_key = 'movie_recap' } @{'Idempotency-Key' = 'accept-project-' + [guid]::NewGuid().ToString()}
    $results.project_created = $null -ne $project.id
    $ffmpeg = (Get-Command ffmpeg.exe -ErrorAction Stop).Source
    $ffprobe = (Get-Command ffprobe.exe -ErrorAction Stop).Source
    $fixture = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-acceptance-' + [guid]::NewGuid().ToString() + '.mp4')
    try {
        & $ffmpeg '-hide_banner' '-loglevel' 'error' '-f' 'lavfi' '-i' 'color=c=black:s=320x180:r=24' '-t' '1' '-y' $fixture
        if ($LASTEXITCODE -ne 0) { throw 'FFmpeg fixture generation failed.' }
        & $ffprobe '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $fixture | Out-Null
        $results.ffmpeg_ffprobe_path = $LASTEXITCODE -eq 0
        $fixtureInfo = Get-Item -LiteralPath $fixture
        $fixtureHash = (Get-FileHash -LiteralPath $fixture -Algorithm SHA256).Hash.ToLowerInvariant()
        $uploadEnvelope = Invoke-Api 'POST' ('/projects/' + $project.id + '/assets/upload-sessions') $token @{
            kind = 'video'; filename = $fixtureInfo.Name; content_type = 'video/mp4'; size_bytes = [int64]$fixtureInfo.Length; sha256 = $fixtureHash; multipart = $true
        } @{'Idempotency-Key' = 'accept-upload-' + [guid]::NewGuid().ToString()}
        $results.upload_session_created = $null -ne $uploadEnvelope.upload.id
        $part = @($uploadEnvelope.upload.parts)[0]
        if ($null -eq $part -or [string]::IsNullOrWhiteSpace([string]$part.url)) { throw 'Upload session did not return a signed part URL.' }
        $partHeaders = @{}
        if ($null -ne $part.headers) { foreach ($property in $part.headers.psobject.Properties) { $partHeaders[$property.Name] = [string]$property.Value } }
        $putResponse = Invoke-WebRequest -UseBasicParsing -Method Put -Uri ([string]$part.url) -Headers $partHeaders -InFile $fixture
        $etag = [string]$putResponse.Headers.ETag
        if ([string]::IsNullOrWhiteSpace($etag)) { throw 'Object storage upload did not return an ETag.' }
        $complete = Invoke-Api 'POST' ('/projects/' + $project.id + '/assets/' + $uploadEnvelope.asset.id + '/upload-sessions/' + $uploadEnvelope.upload.id + '/complete') $token @{
            sha256 = $fixtureHash; parts = @(@{ part_number = 1; etag = $etag })
        }
        $results.upload_completed = $complete.upload.status -eq 'completed'
        $results.upload_validation_job_created = $null -ne $complete.validation_job_id
        if ($complete.validation_job_id) {
            $null = Invoke-Api 'POST' ('/projects/' + $project.id + '/jobs/' + $complete.validation_job_id + '/start') $token @{} @{'Idempotency-Key' = 'accept-upload-start-' + [guid]::NewGuid().ToString()}
            $uploadTerminal = $null
            for ($attempt = 0; $attempt -lt 60; $attempt++) {
                Start-Sleep -Milliseconds 500
                $uploadCurrent = Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $complete.validation_job_id) $token
                if ($uploadCurrent.status -in @('completed', 'failed', 'dead_lettered', 'cancelled')) { $uploadTerminal = $uploadCurrent; break }
            }
            $results.upload_go_worker_completed = $null -ne $uploadTerminal -and $uploadTerminal.status -eq 'completed'
            if (-not $results.upload_go_worker_completed) { throw 'Upload validation Job did not reach the exact completed state.' }
        }
    } finally {
        if (Test-Path -LiteralPath $fixture) { Remove-Item -LiteralPath $fixture -Force }
    }
    $job = if ($null -ne $complete.validation_job_id) { Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $complete.validation_job_id) $token } else { $null }
    $results.durable_job_created = $null -ne $job.id
    $terminal = $job
    $results.go_worker_artifact_path = $null -ne $terminal -and $terminal.status -eq 'completed'
    if (-not $results.go_worker_artifact_path) { throw 'Go worker Job did not reach the exact completed state.' }
    $events = Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $job.id + '/events?after_sequence=0&limit=100') $token
    $results.sse_replay_source = $null -ne $events.items
    $lastSequence = [int64]0
    foreach ($event in @($events.items)) { if ([int64]$event.sequence -gt $lastSequence) { $lastSequence = [int64]$event.sequence } }
    $firstFrame = Read-SseFrame ([string]$project.id) ([string]$job.id) 0 $token
    $reconnectCursor = if ($lastSequence -gt 0) { $lastSequence - 1 } else { 0 }
    $secondFrame = Read-SseFrame ([string]$project.id) ([string]$job.id) $reconnectCursor $token
    $results.sse_snapshot = $firstFrame -match 'stream\.snapshot'
    $results.sse_live = $firstFrame -match 'job\.' -or $secondFrame -match 'job\.'
    $results.sse_reconnect = $secondFrame -match 'id: ' -and ($secondFrame -match 'event: ')
    if (-not $results.sse_snapshot -or -not $results.sse_live -or -not $results.sse_reconnect) { throw 'SSE snapshot/live/reconnect evidence was incomplete.' }

    # The Python hop must be created through Product API and consumed from
    # Redis Streams by the running ml-worker. Direct worker stdio is not an
    # acceptance path because it bypasses Job/Run/Step, leases and artifacts.
    $pythonProject = Invoke-Api 'POST' '/projects' $token @{ name = 'python-redis-acceptance-' + (Get-Date -Format 'yyyyMMddHHmmss'); workflow_key = 'acceptance_analysis' } @{'Idempotency-Key' = 'accept-python-project-' + [guid]::NewGuid().ToString()}
    $results.python_project_created = $null -ne $pythonProject.id
    $pythonJob = Invoke-Api 'POST' ('/projects/' + $pythonProject.id + '/jobs') $token @{ kind = 'analysis'; mode = 'automatic'; input = @{ mode = 'deterministic' }; auto_start = $true } @{'Idempotency-Key' = 'accept-python-job-' + [guid]::NewGuid().ToString()}
    $pythonTerminal = $null
    for ($attempt = 0; $attempt -lt 120; $attempt++) {
        Start-Sleep -Milliseconds 500
        $pythonCurrent = Invoke-Api 'GET' ('/projects/' + $pythonProject.id + '/jobs/' + $pythonJob.id) $token
        if ($pythonCurrent.status -in @('completed', 'failed', 'dead_lettered', 'cancelled')) { $pythonTerminal = $pythonCurrent; break }
    }
    $results.python_worker_terminal_status = if ($null -eq $pythonTerminal) { '' } else { [string]$pythonTerminal.status }
    $results.python_worker_redis_result = $null -ne $pythonTerminal -and $pythonTerminal.status -eq 'completed'
    if (-not $results.python_worker_redis_result) { throw 'Python Redis worker Job did not reach the exact completed state.' }

    # Recovery is part of T550, not an optional unit-test claim. Restart the
    # tracked API/Go/Python/web process set while preserving Docker volumes,
    # then re-authenticate and read the same canonical Job rows and SSE stream.
    $devScript = Join-Path $repoRoot 'tools\dev.ps1'
    $restartArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $devScript, '-Profile', $Profile, '-Action', 'restart')
    if ($Profile -eq 'lan') {
        $apiUri = [Uri]$ApiBaseUrl
        if ($apiUri.Host -eq 'localhost' -or $apiUri.Host -eq '127.0.0.1' -or $apiUri.Host -eq '::1') { throw 'LAN recovery requires the actual private-LAN API address.' }
        $restartArgs += @('-LanServerIp', $apiUri.Host)
    }
    & powershell.exe @restartArgs
    if ($LASTEXITCODE -ne 0) { throw 'Tracked application restart failed during T550 recovery acceptance.' }
    $relogin = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
    $token = $relogin.access_token
    if ([string]::IsNullOrWhiteSpace($token)) { throw 'LocalAuth did not recover after application restart.' }
    $recoveredGoJob = Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $job.id) $token
    $recoveredPythonJob = Invoke-Api 'GET' ('/projects/' + $pythonProject.id + '/jobs/' + $pythonJob.id) $token
    $results.recovery_go_job_completed = [string]$recoveredGoJob.status -eq 'completed'
    $results.recovery_python_job_completed = [string]$recoveredPythonJob.status -eq 'completed'
    $recoveryFrame = Read-SseFrame ([string]$project.id) ([string]$job.id) 0 $token
    $results.recovery_sse_snapshot = $recoveryFrame -match 'stream\.snapshot'
    if (-not $results.recovery_go_job_completed -or -not $results.recovery_python_job_completed -or -not $results.recovery_sse_snapshot) {
        throw 'Canonical Job or SSE state was not recovered after the tracked application restart.'
    }

    $required = @('live','ready','local_auth','default_workspace','project_created','ffmpeg_ffprobe_path','upload_session_created','upload_completed','upload_validation_job_created','upload_go_worker_completed','durable_job_created','go_worker_artifact_path','sse_replay_source','sse_snapshot','sse_live','sse_reconnect','python_project_created','python_worker_redis_result','recovery_go_job_completed','recovery_python_job_completed','recovery_sse_snapshot')
    foreach ($name in $required) { if ($results[$name] -ne $true) { throw ('Acceptance check failed: ' + $name) } }
    if ($Profile -eq 'lan') {
        if ([string]::IsNullOrWhiteSpace($LanBaseUrl)) { $LanBaseUrl = $env:NH_ACCEPT_LAN_BASE_URL }
        if ([string]::IsNullOrWhiteSpace($LanBaseUrl)) {
            $results.lan_server_path = 'missing_explicit_lan_base_url'
        } else {
            $results.lan_server_path = (Invoke-WebRequest -UseBasicParsing ($LanBaseUrl.TrimEnd('/') + '/api/v1/live')).StatusCode -eq 200
        }
        if ([string]::IsNullOrWhiteSpace($LanWebBaseUrl)) { $LanWebBaseUrl = $env:NH_ACCEPT_LAN_WEB_BASE_URL }
        if ([string]::IsNullOrWhiteSpace($LanWebBaseUrl)) {
            $results.lan_web_path = 'missing_explicit_lan_web_base_url'
        } else {
            $results.lan_web_path = (Invoke-WebRequest -UseBasicParsing $LanWebBaseUrl.TrimEnd('/')).StatusCode -eq 200
        }
        $results.physical_second_device = 'not_run_by_server_acceptance; use tools/accept-lan-client.ps1 on a different machine'
    }
    $results | ConvertTo-Json -Depth 8
} catch {
    $results.error = 'acceptance_failed'
    $results.details = $_.Exception.Message
    $results.error_line = $_.InvocationInfo.ScriptLineNumber
    $results.error_command = $_.InvocationInfo.Line.Trim()
    $results | ConvertTo-Json -Depth 8
    exit 1
}
