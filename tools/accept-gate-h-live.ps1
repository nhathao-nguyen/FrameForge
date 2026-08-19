[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH,
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH,
    [string]$EvidencePath = '',
    [string]$CorpusPath = '',
    [string]$ProviderPolicyPath = '',
    [int]$TimeoutSeconds = 360
)

$ErrorActionPreference = 'Stop'
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'
$expectedFfmpeg = 'AF9E7AF850346AE908745F6191CDCF9581889E915F8FE2DD6AB8CABEF21161D9'
$expectedFfprobe = '6D2B7AC8CD07DA82F066994BE8BCF0C74AB1ABA7F248BB75025FCC543711181A'

function Require-Value([string]$Name, [string]$Value) {
    if ([string]::IsNullOrWhiteSpace($Value)) { throw "$Name is required." }
}

function Invoke-Api([string]$Method, [string]$Path, [string]$Token = '', [object]$Body = $null, [hashtable]$ExtraHeaders = @{}) {
    $headers = @{}
    if ($Token -ne '') { $headers.Authorization = 'Bearer ' + $Token }
    foreach ($key in $ExtraHeaders.Keys) { $headers[$key] = $ExtraHeaders[$key] }
    $params = @{ Method = $Method; Uri = $apiRoot + $Path; Headers = $headers; ErrorAction = 'Stop'; TimeoutSec = 30 }
    if ($null -ne $Body) {
        $params.ContentType = 'application/json'
        $params.Body = ($Body | ConvertTo-Json -Depth 30 -Compress)
    }
    Invoke-RestMethod @params
}

function Wait-JobTerminal([string]$ProjectId, [string]$JobId, [string]$Token, [int]$Timeout) {
    $deadline = [DateTime]::UtcNow.AddSeconds($Timeout)
    while ([DateTime]::UtcNow -lt $deadline) {
        $value = Invoke-Api 'GET' ('/projects/' + $ProjectId + '/jobs/' + $JobId) $Token
        if ([string]$value.status -in @('completed', 'failed', 'dead_lettered', 'cancelled')) { return $value }
        Start-Sleep -Milliseconds 500
    }
    throw "Job $JobId did not reach a terminal state within $Timeout seconds."
}

function Collection-Items([object]$Value) {
    if ($null -ne $Value.items) { return @($Value.items) }
    return @($Value)
}

function Convert-ContractArtifactIdToUuid([string]$ArtifactId) {
    if ($ArtifactId -match '^artifact_([0-9a-fA-F_]{36})$') {
        return ($Matches[1] -replace '_', '-')
    }
    return $ArtifactId
}

Require-Value 'NH_API_ADMIN_USERNAME' $env:NH_API_ADMIN_USERNAME
Require-Value 'NH_API_ADMIN_PASSWORD' $env:NH_API_ADMIN_PASSWORD
Require-Value 'FfmpegPath' $FfmpegPath
Require-Value 'FfprobePath' $FfprobePath
if (-not (Test-Path -LiteralPath $FfmpegPath -PathType Leaf)) { throw "FFmpeg was not found: $FfmpegPath" }
if (-not (Test-Path -LiteralPath $FfprobePath -PathType Leaf)) { throw "ffprobe was not found: $FfprobePath" }
if ((Get-FileHash -LiteralPath $FfmpegPath -Algorithm SHA256).Hash -ne $expectedFfmpeg) { throw 'FFmpeg hash does not match the reviewed T002 pin.' }
if ((Get-FileHash -LiteralPath $FfprobePath -Algorithm SHA256).Hash -ne $expectedFfprobe) { throw 'ffprobe hash does not match the reviewed T002 pin.' }

if ([string]::IsNullOrWhiteSpace($EvidencePath)) {
    $EvidencePath = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-gate-h-live-' + [guid]::NewGuid().ToString() + '.json')
}
$providerPolicy = $null
if (-not [string]::IsNullOrWhiteSpace($ProviderPolicyPath)) {
    if (-not (Test-Path -LiteralPath $ProviderPolicyPath -PathType Leaf)) { throw "Provider policy was not found: $ProviderPolicyPath" }
    $providerPolicy = Get-Content -Raw -LiteralPath $ProviderPolicyPath | ConvertFrom-Json
}
$evidenceDirectory = Split-Path -Parent ([IO.Path]::GetFullPath($EvidencePath))
New-Item -ItemType Directory -Force -Path $evidenceDirectory | Out-Null

$report = [ordered]@{
    schema_version = 'nh-media/gate-h-live/v1'
    started_at = (Get-Date).ToUniversalTime().ToString('o')
    api_base_url = $ApiBaseUrl
    ffmpeg_sha256 = (Get-FileHash -LiteralPath $FfmpegPath -Algorithm SHA256).Hash.ToLowerInvariant()
    ffprobe_sha256 = (Get-FileHash -LiteralPath $FfprobePath -Algorithm SHA256).Hash.ToLowerInvariant()
    provider_policy_path = if ($ProviderPolicyPath) { [IO.Path]::GetFullPath($ProviderPolicyPath) } else { '' }
    corpus_path = if ($CorpusPath) { [IO.Path]::GetFullPath($CorpusPath) } else { '' }
}
$fixture = if ($CorpusPath) { [IO.Path]::GetFullPath($CorpusPath) } else { Join-Path ([IO.Path]::GetTempPath()) ('nh-media-gate-h-source-' + [guid]::NewGuid().ToString() + '.mp4') }
$removeFixture = [string]::IsNullOrWhiteSpace($CorpusPath)
try {
    $login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
    $token = [string]$login.access_token
    Require-Value 'LocalAuth access token' $token

    $project = Invoke-Api 'POST' '/projects' $token @{ name = 'gate-h-restore-trace-' + (Get-Date -Format 'yyyyMMddHHmmss'); workflow_key = 'movie_recap' } @{'Idempotency-Key' = 'gate-h-live-project-' + [guid]::NewGuid().ToString() }
    $projectId = [string]$project.id
    $report.project_id = $projectId

    if ($removeFixture) {
        & $FfmpegPath '-hide_banner' '-loglevel' 'error' '-f' 'lavfi' '-i' 'testsrc=size=640x360:rate=24' '-f' 'lavfi' '-i' 'sine=frequency=440:sample_rate=48000' '-t' '3' '-c:v' 'libx264' '-pix_fmt' 'yuv420p' '-c:a' 'aac' '-shortest' '-y' $fixture
        if ($LASTEXITCODE -ne 0) { throw 'FFmpeg fixture generation failed.' }
    } elseif (-not (Test-Path -LiteralPath $fixture -PathType Leaf)) {
        throw "Corpus media was not found: $fixture"
    }
    & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $fixture | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'ffprobe fixture validation failed.' }

    $fixtureInfo = Get-Item -LiteralPath $fixture
    $fixtureHash = (Get-FileHash -LiteralPath $fixture -Algorithm SHA256).Hash.ToLowerInvariant()
    $upload = Invoke-Api 'POST' ('/projects/' + $projectId + '/assets/upload-sessions') $token @{ kind = 'video'; filename = $fixtureInfo.Name; content_type = 'video/mp4'; size_bytes = [int64]$fixtureInfo.Length; sha256 = $fixtureHash; multipart = $true } @{'Idempotency-Key' = 'gate-h-live-upload-' + [guid]::NewGuid().ToString() }
    $part = @($upload.upload.parts)[0]
    if ($null -eq $part) { throw 'Upload session did not provide a part.' }
    $partHeaders = @{}
    if ($null -ne $part.headers) { foreach ($property in $part.headers.psobject.Properties) { $partHeaders[$property.Name] = [string]$property.Value } }
    $putResponse = Invoke-WebRequest -UseBasicParsing -Method Put -Uri ([string]$part.url) -Headers $partHeaders -InFile $fixture -TimeoutSec 60
    $etag = [string]$putResponse.Headers.ETag
    Require-Value 'MinIO upload ETag' $etag
    $complete = Invoke-Api 'POST' ('/projects/' + $projectId + '/assets/' + $upload.asset.id + '/upload-sessions/' + $upload.upload.id + '/complete') $token @{ sha256 = $fixtureHash; parts = @(@{ part_number = 1; etag = $etag }) }
    $validationJobId = [string]$complete.validation_job_id
    $report.validation_job_id = $validationJobId
    $null = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs/' + $validationJobId + '/start') $token @{} @{'Idempotency-Key' = 'gate-h-live-validation-start-' + [guid]::NewGuid().ToString() }
    $validation = Wait-JobTerminal $projectId $validationJobId $token $TimeoutSeconds
    $report.validation_status = [string]$validation.status
    if ([string]$validation.status -ne 'completed') { throw 'Asset validation Job did not complete.' }

    $asset = Invoke-Api 'GET' ('/projects/' + $projectId + '/assets/' + $upload.asset.id) $token
    $sourceArtifactId = [string]$asset.original_artifact_id
    Require-Value 'Committed source Artifact ID' $sourceArtifactId
    $report.source_artifact_id = $sourceArtifactId
    $report.source_sha256 = $fixtureHash

    $jobParams = @{ duration_sec = 3; language = 'en' }
    if ($null -ne $providerPolicy) { $jobParams.provider_policy = $providerPolicy }
    $job = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs') $token @{ kind = 'pipeline'; workflow_key = 'movie_recap'; mode = 'automatic'; auto_start = $true; input = @{ artifacts = @(@{ artifact_id = $sourceArtifactId; role = 'source_original'; sha256 = $fixtureHash }) }; params = $jobParams } @{'Idempotency-Key' = 'gate-h-live-movie-' + [guid]::NewGuid().ToString() }
    $jobId = [string]$job.id
    $report.movie_job_id = $jobId
    $terminal = Wait-JobTerminal $projectId $jobId $token $TimeoutSeconds
    $report.movie_job_status = [string]$terminal.status
    if ([string]$terminal.status -ne 'completed') { throw 'movie_recap Job did not complete.' }

    $timelineDocument = @{
        schema_version = '1.0'
        timeline_id = 'client-timeline'
        timeline_version_id = 'client-timeline-version'
        project_id = $projectId
        version = 1
        duration_sec = 3
        tracks = @(@{
            id = 'video-1'
            kind = 'video'
            name = 'Source'
            order = 1
            clips = @(@{
                id = 'source-clip-1'
                timeline_in_sec = 0
                timeline_out_sec = 3
                source_in_sec = 0
                source_out_sec = 3
                source = @{ type = 'artifact'; artifact_id = $sourceArtifactId }
                origin = 'user'
            })
        })
    }
    $timelineEnvelope = Invoke-Api 'POST' ('/projects/' + $projectId + '/timelines') $token @{ origin = 'user'; document = $timelineDocument }
    $timelineId = [string]$timelineEnvelope.timeline.id
    $timelineVersionId = [string]$timelineEnvelope.version.id
    Require-Value 'Timeline ID' $timelineId
    Require-Value 'TimelineVersion ID' $timelineVersionId
    $timelineValidation = Invoke-Api 'POST' ('/projects/' + $projectId + '/timelines/' + $timelineId + '/versions/' + $timelineVersionId + '/validate') $token @{}
    if (-not [bool]$timelineValidation.valid) { throw 'TimelineVersion validation did not pass.' }
    $approvedTimeline = Invoke-Api 'POST' ('/projects/' + $projectId + '/timelines/' + $timelineId + '/versions/' + $timelineVersionId + '/approve') $token @{}
    $report.timeline_id = $timelineId
    $report.timeline_version_id = $timelineVersionId
    $report.timeline_validation_valid = [bool]$timelineValidation.valid
    $report.timeline_approval_status = [string]$approvedTimeline.status

    $renderRequest = Invoke-Api 'POST' ('/projects/' + $projectId + '/renders') $token @{
        timeline_version_id = $timelineVersionId
        render_profile = @{ profile_key = 'youtube_16_9'; version = 1 }
        preview = $false
        overrides = @{}
    }
    $renderId = [string]$renderRequest.id
    $renderJobId = [string]$renderRequest.job_id
    Require-Value 'Render ID' $renderId
    Require-Value 'Render Job ID' $renderJobId
    $report.render_id = $renderId
    $report.render_job_id = $renderJobId
    $null = Invoke-Api 'POST' ('/projects/' + $projectId + '/jobs/' + $renderJobId + '/start') $token @{} @{'Idempotency-Key' = 'gate-h-live-render-start-' + [guid]::NewGuid().ToString() }
    $renderTerminal = Wait-JobTerminal $projectId $renderJobId $token $TimeoutSeconds
    $report.render_job_status = [string]$renderTerminal.status
    if ([string]$renderTerminal.status -ne 'completed') { throw 'Render Job did not complete.' }

    $runs = @(Collection-Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $jobId + '/runs') $token))
    $steps = @(Collection-Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $jobId + '/steps') $token))
    $renderSteps = @(Collection-Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + $renderJobId + '/steps') $token))
    $timelineList = @(Collection-Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/timelines') $token))
    $renderList = @(Collection-Items (Invoke-Api 'GET' ('/projects/' + $projectId + '/renders') $token))
    $outputRefs = @($steps + $renderSteps | ForEach-Object { @($_.output_refs) })
    $artifactRefs = @($outputRefs | Where-Object { $_ -is [pscustomobject] -and -not [string]::IsNullOrWhiteSpace([string]$_.artifact_id) } | ForEach-Object { [ordered]@{ artifact_id = [string]$_.artifact_id; role = [string]$_.role; sha256 = [string]$_.sha256 } })
    $artifactRefs = @([ordered]@{ artifact_id = $sourceArtifactId; role = 'source_original'; sha256 = $fixtureHash }) + $artifactRefs
    $timeline = @($timelineList | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_.current_version_id) } | Select-Object -Last 1)
    if ($timeline.Count -eq 0) { throw 'No current Timeline was returned through Product API.' }
    $timelineId = [string]$timeline[0].id
    $timelineVersionId = [string]$timeline[0].current_version_id
    $timelineVersion = Invoke-Api 'GET' ('/projects/' + $projectId + '/timelines/' + $timelineId + '/versions/' + $timelineVersionId) $token
    $timelineValidation = Invoke-Api 'POST' ('/projects/' + $projectId + '/timelines/' + $timelineId + '/versions/' + $timelineVersionId + '/validate') $token @{}
    $render = @($renderList | Where-Object { [string]$_.timeline_version_id -eq $timelineVersionId } | Select-Object -Last 1)
    $renderValue = if ($render.Count -gt 0) { $render[0] } else { $null }
    $downloadEvidence = @()
    foreach ($artifactRef in $artifactRefs) {
        $downloadPath = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-gate-h-download-' + [guid]::NewGuid().ToString() + '.bin')
        $downloadArtifactId = Convert-ContractArtifactIdToUuid ([string]$artifactRef.artifact_id)
        try {
            $download = Invoke-Api 'GET' ('/projects/' + $projectId + '/artifacts/' + $downloadArtifactId + '/download') $token
            $url = [string]$download.url
            if ([string]::IsNullOrWhiteSpace($url)) { throw 'Artifact download URL was empty.' }
            Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $downloadPath -TimeoutSec 60 | Out-Null
            $downloadHash = (Get-FileHash -LiteralPath $downloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
            $downloadSize = (Get-Item -LiteralPath $downloadPath).Length
            $downloadEvidence += [ordered]@{ artifact_id = $artifactRef.artifact_id; api_artifact_id = $downloadArtifactId; role = $artifactRef.role; download_url_issued = $true; downloaded = $true; downloaded_size_bytes = [int64]$downloadSize; downloaded_sha256 = $downloadHash; sha256_match = ($downloadHash -eq [string]$artifactRef.sha256) }
        } catch {
            $downloadEvidence += [ordered]@{ artifact_id = $artifactRef.artifact_id; api_artifact_id = $downloadArtifactId; role = $artifactRef.role; download_url_issued = $false; downloaded = $false; sha256_match = $false }
        } finally {
            if (Test-Path -LiteralPath $downloadPath) { Remove-Item -LiteralPath $downloadPath -Force }
        }
    }
    $report.run_count = $runs.Count
    $report.step_count = $steps.Count
    $report.completed_step_count = @($steps | Where-Object { [string]$_.status -eq 'completed' }).Count
    $report.job_pipeline_run_id = [string]$terminal.pipeline_run_id
    $report.timeline = [ordered]@{ id = $timelineId; version_id = $timelineVersionId; status = [string]$timelineVersion.status; content_hash = [string]$timelineVersion.content_hash; validation_valid = [bool]$timelineValidation.valid }
    $renderValue = Invoke-Api 'GET' ('/projects/' + $projectId + '/renders/' + $renderId) $token
    $report.render = [ordered]@{ id = $renderId; timeline_version_id = [string]$renderValue.render.timeline_version_id; status = [string]$renderValue.render.status; job_status = [string]$renderValue.job.status }
    $report.artifact_refs = $artifactRefs
    $report.downloads = $downloadEvidence
    $report.trace_complete = ($report.movie_job_status -eq 'completed' -and $report.render_job_status -eq 'completed' -and $report.render.status -eq 'completed' -and $report.completed_step_count -eq $report.step_count -and $report.timeline.validation_valid -and $artifactRefs.Count -gt 0 -and @($downloadEvidence | Where-Object { -not $_.sha256_match }).Count -eq 0)
    $report.finished_at = (Get-Date).ToUniversalTime().ToString('o')
    $report | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath $EvidencePath -Encoding UTF8
    Get-Content -Raw -LiteralPath $EvidencePath
}
catch {
    $report.status = 'FAIL'
    $report.error = $_.Exception.Message
    $report.failed_at = (Get-Date).ToUniversalTime().ToString('o')
    $report | ConvertTo-Json -Depth 30 | Set-Content -LiteralPath $EvidencePath -Encoding UTF8
    Get-Content -Raw -LiteralPath $EvidencePath
    exit 1
}
finally {
    if ($removeFixture -and (Test-Path -LiteralPath $fixture)) { Remove-Item -LiteralPath $fixture -Force }
}
