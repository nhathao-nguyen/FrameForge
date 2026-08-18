[CmdletBinding()]
param(
    [ValidateSet('local', 'lan')]
    [string]$Profile = 'local',
    [string]$ApiBaseUrl = '',
    [string]$LanBaseUrl = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ApiBaseUrl)) { $ApiBaseUrl = if ($Profile -eq 'lan') { 'http://127.0.0.1:8080' } else { 'http://127.0.0.1:8080' } }
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

function Invoke-JsonProcess([string]$FilePath, [string[]]$Arguments, [string]$InputText) {
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $FilePath
    $info.WorkingDirectory = $repoRoot
    $info.UseShellExecute = $false
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    if ($null -ne $info.ArgumentList) {
        foreach ($argument in $Arguments) { [void]$info.ArgumentList.Add($argument) }
    } else {
        $info.Arguments = $Arguments -join ' '
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    [void]$process.Start()
    $process.StandardInput.WriteLine($InputText)
    $process.StandardInput.Close()
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) { throw ($FilePath + ' failed: ' + $stderr) }
    return @{ output = $stdout; diagnostics = $stderr }
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
        }
    } finally {
        if (Test-Path -LiteralPath $fixture) { Remove-Item -LiteralPath $fixture -Force }
    }
    $job = Invoke-Api 'POST' ('/projects/' + $project.id + '/jobs') $token @{ kind = 'asset_probe'; mode = 'automatic'; input = @{ mode = 'deterministic' }; auto_start = $true } @{'Idempotency-Key' = 'accept-job-' + [guid]::NewGuid().ToString()}
    $results.durable_job_created = $null -ne $job.id
    $terminal = $null
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        Start-Sleep -Milliseconds 500
        $current = Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $job.id) $token
        if ($current.status -in @('completed', 'failed', 'dead_lettered', 'cancelled')) { $terminal = $current; break }
    }
    $results.go_worker_artifact_path = $null -ne $terminal -and $terminal.status -eq 'completed'
    $events = Invoke-Api 'GET' ('/projects/' + $project.id + '/jobs/' + $job.id + '/events?after_sequence=0&limit=100') $token
    $results.sse_replay_source = $null -ne $events.items
    $uv = (Get-Command uv.exe -ErrorAction Stop).Source
    $pythonCommand = '{"schema_version":"worker-command/v1","message_id":"msg_accept_python_001","capability":"analysis","project_id":"project_accept_python_001","job_id":"job_accept_python_001","pipeline_run_id":"run_accept_python_001","job_step_id":"step_accept_python_001","attempt":1,"input_refs":[],"config":{"mode":"deterministic"}}'
    $pythonResult = Invoke-JsonProcess $uv @('run', '--project', 'services/ml-worker', 'python', '-m', 'nh_media.worker') $pythonCommand
    $pythonText = [string]$pythonResult['output']
    if ([string]::IsNullOrWhiteSpace($pythonText)) { throw 'Python worker returned no protocol result.' }
    $parsedPython = $pythonText.Trim() | ConvertFrom-Json
    $results.python_worker_result = $parsedPython.status -eq 'completed' -and $null -ne $parsedPython.output_refs
    if ($Profile -eq 'lan') {
        if ([string]::IsNullOrWhiteSpace($LanBaseUrl)) { $LanBaseUrl = $env:NH_ACCEPT_LAN_BASE_URL }
        if ([string]::IsNullOrWhiteSpace($LanBaseUrl)) {
            $results.lan_server_path = 'missing_explicit_lan_base_url'
        } else {
            $results.lan_server_path = (Invoke-WebRequest -UseBasicParsing ($LanBaseUrl.TrimEnd('/') + '/api/v1/live')).StatusCode -eq 200
        }
        $results.physical_second_device = 'external_uncontrolled'
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
