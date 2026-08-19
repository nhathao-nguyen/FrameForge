[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$EvidenceInput = 'docs/evidence/real-local-gate-h-live-20260819-r21.json',
    [string]$EvidenceOutput = 'docs/evidence/real-local-render-profile-smoke-20260819-r24.json',
    [ValidateSet('youtube_16_9', 'shorts_9_16', 'square_1_1')]
    [string]$RenderProfileKey = 'square_1_1',
    [string]$EnvFile = '',
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH,
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = 'Stop'
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'

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
    if ($null -ne $Body) { $params.ContentType = 'application/json'; $params.Body = ($Body | ConvertTo-Json -Depth 20 -Compress) }
    Invoke-RestMethod @params
}

function Items([object]$Value) { if ($null -ne $Value.items) { return @($Value.items) }; return @($Value) }
function ArtifactUuid([string]$Value) { if ($Value -match '^artifact_([0-9a-fA-F_]{36})$') { return ($Matches[1] -replace '_', '-') }; return $Value }
function Wait-Terminal([string]$ProjectId, [string]$JobId, [string]$Token) {
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $job = Invoke-Api 'GET' ('/projects/' + $ProjectId + '/jobs/' + $JobId) $Token
        if ([string]$job.status -in @('completed', 'failed', 'cancelled', 'dead_lettered')) { return $job }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)
    throw 'Render profile smoke timed out.'
}

Import-EnvFile $EnvFile
if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME) -or [string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) { throw 'LocalAuth environment is required.' }
if (-not (Test-Path -LiteralPath $EvidenceInput -PathType Leaf)) { throw 'Input evidence was not found.' }
$input = Get-Content -Raw -LiteralPath $EvidenceInput | ConvertFrom-Json
$projectId = [string]$input.project_id
$timelineVersionId = [string]$input.timeline_version_id
$login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
$token = [string]$login.access_token
$render = Invoke-Api 'POST' ('/projects/' + $projectId + '/renders') $token @{ timeline_version_id = $timelineVersionId; render_profile = @{ profile_key = $RenderProfileKey; version = 1 }; preview = $false; overrides = @{} }
$jobId = [string]$render.job_id
$null = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs/' + $jobId + '/start') $token @{}
$terminal = Wait-Terminal $projectId $jobId $token
if ([string]$terminal.status -ne 'completed') { throw ('Render profile smoke job status was ' + [string]$terminal.status) }
$steps = @(Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $jobId + '/steps') $token))
$refs = @($steps | ForEach-Object { @($_.output_refs) } | Where-Object { [string]$_.role -in @('render_video', 'render_metadata') } | Select-Object -Last 2)
if (@($refs | Where-Object role -eq 'render_video').Count -eq 0 -or @($refs | Where-Object role -eq 'render_metadata').Count -eq 0) { throw 'Render profile smoke did not return render artifacts.' }
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-render-smoke-' + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
try {
    $downloaded = [ordered]@{}
    foreach ($ref in $refs) {
        $role = [string]$ref.role
        $path = Join-Path $tempRoot ($role + '.bin')
        $url = [string](Invoke-Api 'GET' ('/projects/' + $projectId + '/artifacts/' + (ArtifactUuid ([string]$ref.artifact_id)) + '/download') $token).url
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $path -TimeoutSec 60 | Out-Null
        $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -ne [string]$ref.sha256) { throw "Render artifact checksum mismatch for $role." }
        $downloaded[$role] = $path
    }
    $metadata = Get-Content -Raw -LiteralPath $downloaded['render_metadata'] | ConvertFrom-Json
    $probe = & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $downloaded['render_video'] | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'ffprobe failed for render profile smoke.' }
    $video = @($probe.streams | Where-Object codec_type -eq 'video' | Select-Object -First 1)
    $audio = @($probe.streams | Where-Object codec_type -eq 'audio')
    $expected = @{ youtube_16_9 = '640x360'; shorts_9_16 = '360x640'; square_1_1 = '480x480' }[$RenderProfileKey]
    $actual = ([int]$video.width).ToString() + 'x' + ([int]$video.height).ToString()
    $report = [ordered]@{ schema_version = 'nh-media/render-profile-smoke/v1'; project_id = $projectId; timeline_version_id = $timelineVersionId; render_id = [string]$render.id; job_id = $jobId; requested_profile_key = $RenderProfileKey; worker_profile_key = [string]$metadata.profile_key; expected_size = $expected; actual_size = $actual; duration_sec = [double]$probe.format.duration; has_video = ($video.Count -gt 0); has_audio = ($audio.Count -gt 0); render_metadata = $metadata; profile_dimensions_pass = ($actual -eq $expected); status = if ([string]$metadata.profile_key -eq $RenderProfileKey -and $actual -eq $expected -and $video.Count -gt 0 -and $audio.Count -gt 0) { 'PASS' } else { 'NOT_PASS' }; finished_at = (Get-Date).ToUniversalTime().ToString('o') }
    $parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    $report | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
    Get-Content -Raw -LiteralPath $EvidenceOutput
} finally { if (Test-Path -LiteralPath $tempRoot) { Remove-Item -LiteralPath $tempRoot -Recurse -Force } }
