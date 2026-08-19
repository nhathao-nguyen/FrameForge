[CmdletBinding()]
param(
    [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
    [string]$EvidenceInput = 'docs/evidence/real-local-gate-h-live-20260819-r17.json',
    [string]$EvidenceOutput = 'docs/evidence/real-local-artifact-validation-20260819-r17.json',
    [string]$EnvFile = '',
    [string]$FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH,
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH
)

$ErrorActionPreference = 'Stop'
$apiRoot = $ApiBaseUrl.TrimEnd('/') + '/api/v1'

function Import-EnvFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Environment file was not found: $Path" }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { throw "Invalid environment line in $Path" }
        $name = $Matches[1]
        $value = $Matches[2].Trim()
        if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        Set-Item -Path ("Env:" + $name) -Value $value
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

function Convert-ContractArtifactIdToUuid([string]$ArtifactId) {
    if ($ArtifactId -match '^artifact_([0-9a-fA-F_]{36})$') { return ($Matches[1] -replace '_', '-') }
    return $ArtifactId
}

function Get-ArrayValue([object]$Value) {
    if ($null -eq $Value) { return @() }
    if ($Value -is [System.Collections.IEnumerable] -and $Value -isnot [string]) {
        $items = [System.Collections.Generic.List[object]]::new()
        foreach ($item in $Value) {
            if ($item -is [System.Collections.IEnumerable] -and $item -isnot [string] -and $item -isnot [pscustomobject]) {
                foreach ($nested in $item) { $items.Add($nested) }
            } else {
                $items.Add($item)
            }
        }
        return @($items.ToArray())
    }
    foreach ($name in @('items', 'segments', 'cues', 'scenes', 'frames')) {
        $property = $Value.psobject.Properties[$name]
        if ($null -ne $property) { return @(Get-ArrayValue $property.Value) }
    }
    return @($Value)
}

function Collection-Items([object]$Value) {
    if ($null -ne $Value.items) { return @($Value.items) }
    return @($Value)
}

function Read-JsonBytes([byte[]]$Bytes, [string]$Role) {
    $text = [Text.Encoding]::UTF8.GetString($Bytes).TrimStart([char]0xFEFF)
    try { return ($text | ConvertFrom-Json) }
    catch { throw "Artifact role '$Role' is not valid UTF-8 JSON: $($_.Exception.Message) prefix=$($text.Substring(0, [Math]::Min(120, $text.Length)))" }
}

function Get-TextValues([object]$Value) {
    $values = [System.Collections.Generic.List[string]]::new()
    function Visit([object]$Node) {
        if ($null -eq $Node) { return }
        if ($Node -is [string]) { if ($Node.Trim() -ne '') { $values.Add($Node.Trim()) }; return }
        if ($Node -is [System.Collections.IEnumerable] -and $Node -isnot [string]) { foreach ($item in $Node) { Visit $item }; return }
        if ($Node -is [pscustomobject]) {
            foreach ($name in @('text', 'content', 'narration', 'summary', 'description', 'caption')) {
                $property = $Node.psobject.Properties[$name]
                if ($null -ne $property -and $property.Value -is [string] -and $property.Value.Trim() -ne '') { $values.Add($property.Value.Trim()) }
            }
            foreach ($property in $Node.psobject.Properties) {
                if ($property.Name -notin @('text', 'content', 'narration', 'summary', 'description', 'caption', 'metadata', 'provenance')) { Visit $property.Value }
            }
        }
    }
    Visit $Value
    return @($values | Select-Object -Unique)
}

function Get-Intervals([object]$Value) {
    $items = Get-ArrayValue $Value
    $intervals = [System.Collections.Generic.List[object]]::new()
    foreach ($item in $items) {
        if ($null -eq $item) { continue }
        $start = $null; $end = $null
        foreach ($name in @('start', 'start_sec', 'timeline_in_sec', 'in_sec', 'from')) {
            $property = $item.psobject.Properties[$name]
            if ($null -ne $property) { $start = [double]$property.Value; break }
        }
        foreach ($name in @('end', 'end_sec', 'timeline_out_sec', 'out_sec', 'to')) {
            $property = $item.psobject.Properties[$name]
            if ($null -ne $property) { $end = [double]$property.Value; break }
        }
        if ($null -ne $start -and $null -ne $end) { $intervals.Add([pscustomobject]@{ start = $start; end = $end }) }
    }
    return @($intervals)
}

function Get-JsonProperty([object]$Value, [string[]]$Names) {
    foreach ($name in $Names) {
        if ($null -ne $Value -and $null -ne $Value.psobject.Properties[$name]) { return $Value.psobject.Properties[$name].Value }
    }
    return $null
}

function Invoke-Ffprobe([string]$Path) {
    $json = & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $Path | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw "ffprobe failed for $Path" }
    return $json
}

function Invoke-MediaDetection([string]$Path) {
    $previousErrorAction = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $black = (& $FfmpegPath '-hide_banner' '-nostats' '-loglevel' 'info' '-i' $Path '-vf' 'blackdetect=d=0.25:pix_th=0.1' '-an' '-f' 'null' 'NUL' 2>&1 | Out-String)
    $silence = (& $FfmpegPath '-hide_banner' '-nostats' '-loglevel' 'info' '-i' $Path '-af' 'silencedetect=n=-35dB:d=0.25' '-vn' '-f' 'null' 'NUL' 2>&1 | Out-String)
    $ErrorActionPreference = $previousErrorAction
    return [ordered]@{
        blackdetect_events = @([regex]::Matches($black, 'black_start:[^\s]+') | ForEach-Object { $_.Value })
        silencedetect_events = @([regex]::Matches($silence, 'silence_(?:start|end):[^\s]+') | ForEach-Object { $_.Value })
        blackdetect_observed = ($black -match 'black_start:')
        silencedetect_observed = ($silence -match 'silence_start:')
    }
}

Import-EnvFile $EnvFile
if ([string]::IsNullOrWhiteSpace($FfmpegPath)) { $FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH }
if ([string]::IsNullOrWhiteSpace($FfprobePath)) { $FfprobePath = $env:NH_MEDIA_FFPROBE_PATH }
if ([string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_USERNAME) -or [string]::IsNullOrWhiteSpace($env:NH_API_ADMIN_PASSWORD)) { throw 'LocalAuth environment is required.' }
if (-not (Test-Path -LiteralPath $EvidenceInput -PathType Leaf)) { throw "Evidence input was not found: $EvidenceInput" }
if (-not (Test-Path -LiteralPath $FfmpegPath -PathType Leaf) -or -not (Test-Path -LiteralPath $FfprobePath -PathType Leaf)) { throw 'Reviewed FFmpeg and ffprobe paths are required.' }

$input = Get-Content -Raw -LiteralPath $EvidenceInput | ConvertFrom-Json
$projectId = [string]$input.project_id
$roleRefs = @{}
foreach ($ref in @($input.artifact_refs)) {
    $role = [string]$ref.role
    if (-not $roleRefs.ContainsKey($role)) { $roleRefs[$role] = $ref }
}
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('nh-media-artifact-validation-' + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null
$files = @{}
$downloadEvidence = [System.Collections.Generic.List[object]]::new()

try {
    $login = Invoke-Api 'POST' '/auth/local/login' '' @{ username = $env:NH_API_ADMIN_USERNAME; password = $env:NH_API_ADMIN_PASSWORD }
    $token = [string]$login.access_token
    if ($token.Length -lt 24) { throw 'LocalAuth token was not returned.' }
    $renderEnvelope = if ([string]$input.render_id -ne '') { Invoke-Api 'GET' ('/projects/' + $projectId + '/renders/' + [string]$input.render_id) $token } else { $null }
    $renderProfileSnapshot = if ($null -ne $renderEnvelope.render.profile_snapshot) { $renderEnvelope.render.profile_snapshot } else { $null }
    if ([string]$input.render_job_id -ne '') {
        $renderStepEnvelope = Invoke-Api 'GET' ('/projects/' + $projectId + '/jobs/' + [string]$input.render_job_id + '/steps') $token
        foreach ($step in @(Collection-Items $renderStepEnvelope)) {
            foreach ($ref in @($step.output_refs)) {
                if ($null -ne $ref -and -not [string]::IsNullOrWhiteSpace([string]$ref.role)) { $roleRefs[[string]$ref.role] = $ref }
            }
        }
    }
    foreach ($role in $roleRefs.Keys) {
        $ref = $roleRefs[$role]
        $artifactId = Convert-ContractArtifactIdToUuid ([string]$ref.artifact_id)
        $download = Invoke-Api 'GET' ('/projects/' + $projectId + '/artifacts/' + $artifactId + '/download') $token
        $path = Join-Path $tempRoot ($role + '.bin')
        Invoke-WebRequest -UseBasicParsing -Uri ([string]$download.url) -OutFile $path -TimeoutSec 120 | Out-Null
        $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
        $match = $hash -eq [string]$ref.sha256
        if (-not $match) { throw "Artifact checksum mismatch for role '$role'." }
        $files[$role] = $path
        $downloadEvidence.Add([ordered]@{ role = $role; artifact_id = [string]$ref.artifact_id; size_bytes = (Get-Item -LiteralPath $path).Length; sha256_match = $match })
    }

    $checks = [ordered]@{}
    $scriptDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['script_version'])) 'script_version'
    $scriptText = @(Get-TextValues $scriptDoc)
    $checks.script = [ordered]@{
        nonempty = ($scriptText.Count -gt 0)
        structured = ($null -ne $scriptDoc.psobject.Properties['segments'] -or $null -ne $scriptDoc.psobject.Properties['scenes'] -or $null -ne $scriptDoc.psobject.Properties['script'] -or $null -ne $scriptDoc.state.script.psobject.Properties['segments'])
        no_placeholder = (@($scriptText | Where-Object { $_ -match '(?i)placeholder|dummy|lorem ipsum|test text' }).Count -eq 0)
        text_count = $scriptText.Count
        provenance_present = ($null -ne $scriptDoc.psobject.Properties['provenance'] -or $null -ne $scriptDoc.psobject.Properties['metadata'] -or $null -ne $scriptDoc.state.script.psobject.Properties['provider_snapshot'] -or $null -ne $scriptDoc.state.script.psobject.Properties['input_refs'])
        language = [string](Get-JsonProperty $scriptDoc @('language', 'language_code', 'locale'))
    }
    if ([string]::IsNullOrWhiteSpace($checks.script.language)) { $checks.script.language = [string](Get-JsonProperty $scriptDoc.state.script @('language', 'language_code', 'locale')) }
    if (-not $checks.script.nonempty -or -not $checks.script.structured -or -not $checks.script.no_placeholder) { throw ('Script artifact content validation failed: ' + ($checks.script | ConvertTo-Json -Compress) + ' root_keys=' + (($scriptDoc.psobject.Properties.Name) -join ',') + ' nested_script_keys=' + ((@($scriptDoc.state.script.psobject.Properties.Name)) -join ',')) }

    $narrationProbe = Invoke-Ffprobe $files['narration_audio']
    $narrationDuration = [double]$narrationProbe.format.duration
    $checks.narration = [ordered]@{ nonzero_bytes = ((Get-Item -LiteralPath $files['narration_audio']).Length -gt 0); duration_sec = $narrationDuration; has_audio_stream = (@($narrationProbe.streams | Where-Object codec_type -eq 'audio').Count -gt 0) }
    if (-not $checks.narration.nonzero_bytes -or $narrationDuration -le 0 -or -not $checks.narration.has_audio_stream) { throw 'Narration audio validation failed.' }

    $transcriptDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['transcript'])) 'transcript'
    $transcriptBody = if ($null -ne $transcriptDoc.state.source_transcript) { $transcriptDoc.state.source_transcript } else { $transcriptDoc }
    $transcriptIntervals = @(Get-Intervals (Get-JsonProperty $transcriptBody @('segments', 'items', 'words')))
    $transcriptOrdered = $true
    for ($index = 1; $index -lt $transcriptIntervals.Count; $index++) { if ($transcriptIntervals[$index].start -lt $transcriptIntervals[$index - 1].start) { $transcriptOrdered = $false } }
    $checks.transcript = [ordered]@{ segment_count = $transcriptIntervals.Count; nonempty = ($transcriptIntervals.Count -gt 0); monotonic = $transcriptOrdered; within_narration = (@($transcriptIntervals | Where-Object { $_.start -lt 0 -or $_.end -le $_.start -or $_.end -gt ($narrationDuration + 0.5) }).Count -eq 0) }
    if (-not $checks.transcript.nonempty -or -not $checks.transcript.monotonic -or -not $checks.transcript.within_narration) { throw ('Transcript interval validation failed: ' + ($checks.transcript | ConvertTo-Json -Compress) + ' root_keys=' + (($transcriptDoc.psobject.Properties.Name) -join ',') + ' state_keys=' + ((@($transcriptDoc.state.psobject.Properties.Name)) -join ',')) }

    $cueDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['subtitle_cues'])) 'subtitle_cues'
    $cueBody = if ($null -ne $cueDoc.state.subtitle_cues) { $cueDoc.state.subtitle_cues } else { $cueDoc }
    $cueValue = if ($cueBody -is [array]) { $cueBody } elseif ($null -ne (Get-JsonProperty $cueBody @('start', 'start_sec', 'end', 'end_sec'))) { @($cueBody) } else { Get-JsonProperty $cueBody @('cues', 'items', 'segments') }
    $cueIntervals = @(Get-Intervals $cueValue)
    $cueOrdered = $true
    for ($index = 1; $index -lt $cueIntervals.Count; $index++) { if ($cueIntervals[$index].start -lt $cueIntervals[$index - 1].start) { $cueOrdered = $false } }
    $srtText = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($files['subtitle_srt']))
    $checks.subtitles = [ordered]@{ cue_count = $cueIntervals.Count; srt_nonempty = ($srtText.Trim().Length -gt 0); ordered = $cueOrdered; within_narration = (@($cueIntervals | Where-Object { $_.start -lt 0 -or $_.end -le $_.start -or $_.end -gt ($narrationDuration + 0.5) }).Count -eq 0); formats = @('srt', 'vtt', 'ass') }
    if (-not $checks.subtitles.srt_nonempty -or $cueIntervals.Count -eq 0 -or -not $checks.subtitles.ordered -or -not $checks.subtitles.within_narration) { throw ('Subtitle cue validation failed: ' + ($checks.subtitles | ConvertTo-Json -Compress) + ' root_keys=' + (($cueDoc.psobject.Properties.Name) -join ',') + ' state_keys=' + ((@($cueDoc.state.psobject.Properties.Name)) -join ',') + ' cue_keys=' + ((@($cueDoc.state.subtitle_cues.psobject.Properties.Name)) -join ',')) }

    $sceneIndex = Read-JsonBytes ([IO.File]::ReadAllBytes($files['scene_index'])) 'scene_index'
    $sceneAnalysis = Read-JsonBytes ([IO.File]::ReadAllBytes($files['scene_analysis'])) 'scene_analysis'
    $sceneIndexBody = if ($null -ne $sceneIndex.state.scene_features) { $sceneIndex.state.scene_features } else { $sceneIndex }
    $sceneAnalysisBody = if ($null -ne $sceneAnalysis.state.scene_analyses) { $sceneAnalysis.state.scene_analyses } else { $sceneAnalysis }
    $sceneValue = if ($sceneIndexBody -is [array]) { $sceneIndexBody } else { Get-JsonProperty $sceneIndexBody @('scenes', 'items', 'segments') }
    $analysisValue = if ($sceneAnalysisBody -is [array]) { $sceneAnalysisBody } else { Get-JsonProperty $sceneAnalysisBody @('scenes', 'items', 'analyses', 'segments') }
    $scenes = @(Get-ArrayValue $sceneValue)
    $analysisItems = @(Get-ArrayValue $analysisValue)
    $sceneText = @(Get-TextValues $sceneAnalysis)
    $checks.scenes = [ordered]@{ index_count = $scenes.Count; analysis_count = $analysisItems.Count; nonempty = ($scenes.Count -gt 0 -and $analysisItems.Count -gt 0); no_placeholder = (@($sceneText | Where-Object { $_ -match '(?i)placeholder|dummy|lorem ipsum' }).Count -eq 0); provenance_present = ($null -ne $sceneAnalysis.psobject.Properties['provenance'] -or $null -ne $sceneAnalysis.psobject.Properties['metadata'] -or $null -ne $sceneAnalysis.state.psobject.Properties['provenance'] -or $null -ne $sceneAnalysis.state.psobject.Properties['metadata']) }
    if (-not $checks.scenes.nonempty -or -not $checks.scenes.no_placeholder) { throw ('Scene artifact content validation failed: ' + ($checks.scenes | ConvertTo-Json -Compress) + ' index_root=' + (($sceneIndex.psobject.Properties.Name) -join ',') + ' index_state=' + ((@($sceneIndex.state.psobject.Properties.Name)) -join ',') + ' index_body=' + ((@($sceneIndexBody.psobject.Properties.Name)) -join ',') + ' analysis_root=' + (($sceneAnalysis.psobject.Properties.Name) -join ',') + ' analysis_state=' + ((@($sceneAnalysis.state.psobject.Properties.Name)) -join ',') + ' analysis_body=' + ((@($sceneAnalysisBody.psobject.Properties.Name)) -join ',')) }

    $embeddingDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['media_embeddings'])) 'media_embeddings'
    $embeddingBody = if ($null -ne $embeddingDoc.state.embedding_index) { $embeddingDoc.state.embedding_index } else { $embeddingDoc }
    $embeddingValue = Get-JsonProperty $embeddingBody @('embeddings', 'items', 'vectors')
    $embeddingItems = if ($embeddingValue -is [array]) { @($embeddingValue) } elseif ($embeddingValue -is [pscustomobject]) { @($embeddingValue.psobject.Properties | ForEach-Object { ,$_.Value }) } else { @() }
    $vectorLengths = @($embeddingItems | ForEach-Object { $vector = if ($_ -is [array]) { $_ } else { Get-JsonProperty $_ @('vector', 'embedding', 'values') }; if ($null -ne $vector) { @($vector).Count } })
    $checks.embeddings = [ordered]@{ item_count = $embeddingItems.Count; vector_count = $vectorLengths.Count; dimensions = @($vectorLengths | Select-Object -Unique); metadata_present = ($null -ne $embeddingBody.psobject.Properties['metadata'] -or $null -ne $embeddingBody.psobject.Properties['model'] -or $null -ne $embeddingBody.psobject.Properties['provider_snapshot']) }
    if ($embeddingItems.Count -eq 0 -or $vectorLengths.Count -eq 0 -or @($vectorLengths | Where-Object { $_ -le 0 }).Count -gt 0) { throw ('Embedding artifact validation failed: ' + ($checks.embeddings | ConvertTo-Json -Compress) + ' root_keys=' + (($embeddingDoc.psobject.Properties.Name) -join ',') + ' state_keys=' + ((@($embeddingDoc.state.psobject.Properties.Name)) -join ',') + ' body_keys=' + ((@($embeddingDoc.state.embedding_index.psobject.Properties.Name)) -join ',')) }

    $matchDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['match_candidates'])) 'match_candidates'
    $selectedDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['selected_match_proposal'])) 'selected_match_proposal'
    $matchBody = if ($null -ne $matchDoc.state.candidates -and @($matchDoc.state.candidates).Count -gt 0) { $matchDoc.state.candidates } elseif ($null -ne $matchDoc.state.match_proposals) { $matchDoc.state.match_proposals } else { $matchDoc }
    $matchValue = if ($matchBody -is [array]) { $matchBody } elseif ($null -ne $matchBody.psobject.Properties['candidate_id']) { @($matchBody) } else { Get-JsonProperty $matchBody @('candidates', 'items', 'matches') }
    $matchItems = @(Get-ArrayValue $matchValue)
    $checks.matching = [ordered]@{ candidate_count = $matchItems.Count; selected_nonempty = (@(Get-TextValues $selectedDoc).Count -gt 0); coverage_present = (Test-Path -LiteralPath $files['coverage_report']) }
    if ($matchItems.Count -eq 0 -or -not $checks.matching.selected_nonempty) { throw ('Matching artifact validation failed: ' + ($checks.matching | ConvertTo-Json -Compress) + ' match_root=' + (($matchDoc.psobject.Properties.Name) -join ',') + ' match_state=' + ((@($matchDoc.state.psobject.Properties.Name)) -join ',') + ' match_body=' + ((@($matchBody.psobject.Properties.Name)) -join ',') + ' selected_root=' + (($selectedDoc.psobject.Properties.Name) -join ',') + ' selected_state=' + ((@($selectedDoc.state.psobject.Properties.Name)) -join ',')) }

    $timelineDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['timeline_version'])) 'timeline_version'
    $timelineValue = Get-JsonProperty $timelineDoc @('segments', 'clips', 'tracks')
    $timelineIntervals = [System.Collections.Generic.List[object]]::new()
    foreach ($track in @(Get-ArrayValue $timelineValue)) {
        $clipValue = Get-JsonProperty $track @('clips', 'segments', 'items')
        $clipItems = if ($null -ne $clipValue) { @(Get-ArrayValue $clipValue) } else { @($track) }
        foreach ($clip in $clipItems) { foreach ($interval in @(Get-Intervals $clip)) { $timelineIntervals.Add($interval) } }
    }
    $timelineJson = $timelineDoc | ConvertTo-Json -Compress -Depth 30
    $checks.timeline = [ordered]@{ content_hash_present = ($null -ne $timelineDoc.psobject.Properties['content_hash'] -or [string]$input.timeline.content_hash -ne ''); interval_count = $timelineIntervals.Count; valid_ranges = (@($timelineIntervals | Where-Object { $_.start -lt 0 -or $_.end -le $_.start }).Count -eq 0); artifact_refs_present = ($null -ne $timelineDoc.psobject.Properties['artifact_refs'] -or $timelineJson -match 'artifact_id') }
    if (-not $checks.timeline.content_hash_present -or -not $checks.timeline.valid_ranges -or -not $checks.timeline.artifact_refs_present) { throw ('Timeline artifact validation failed: ' + ($checks.timeline | ConvertTo-Json -Compress) + ' root_keys=' + (($timelineDoc.psobject.Properties.Name) -join ',') + ' state_keys=' + ((@($timelineDoc.state.psobject.Properties.Name)) -join ',')) }

    $mixedProbe = Invoke-Ffprobe $files['mixed_audio']
    $loudnessDoc = Read-JsonBytes ([IO.File]::ReadAllBytes($files['loudness_report'])) 'loudness_report'
    $checks.audio_mix = [ordered]@{ duration_sec = [double]$mixedProbe.format.duration; has_audio_stream = (@($mixedProbe.streams | Where-Object codec_type -eq 'audio').Count -gt 0); loudness_report_nonempty = ($null -ne $loudnessDoc) }
    if ($checks.audio_mix.duration_sec -le 0 -or -not $checks.audio_mix.has_audio_stream) { throw 'Mixed audio validation failed.' }

    $renderProbe = Invoke-Ffprobe $files['render_video']
    $renderStream = @($renderProbe.streams | Where-Object codec_type -eq 'video' | Select-Object -First 1)
    $renderAudio = @($renderProbe.streams | Where-Object codec_type -eq 'audio')
    $renderMetadata = Read-JsonBytes ([IO.File]::ReadAllBytes($files['render_metadata'])) 'render_metadata'
    $deliverableQa = Read-JsonBytes ([IO.File]::ReadAllBytes($files['deliverable_qa'])) 'deliverable_qa'
    $expectedProfile = [string]$input.render_profile_key
    $expectedWidth = if ($null -ne $renderProfileSnapshot.width) { [int]$renderProfileSnapshot.width } else { @{ youtube_16_9 = 640; shorts_9_16 = 360; square_1_1 = 480 }[$expectedProfile] }
    $expectedHeight = if ($null -ne $renderProfileSnapshot.height) { [int]$renderProfileSnapshot.height } else { @{ youtube_16_9 = 360; shorts_9_16 = 640; square_1_1 = 480 }[$expectedProfile] }
    $expectedSize = $expectedWidth.ToString() + 'x' + $expectedHeight.ToString()
    $actualSize = ([int]$renderStream.width).ToString() + 'x' + ([int]$renderStream.height).ToString()
    $checks.render = [ordered]@{ profile = $expectedProfile; expected_size = $expectedSize; actual_size = $actualSize; duration_sec = [double]$renderProbe.format.duration; has_video_stream = ($renderStream.Count -gt 0); has_audio_stream = ($renderAudio.Count -gt 0); metadata_nonempty = ($null -ne $renderMetadata); deliverable_qa_nonempty = ($null -ne $deliverableQa); detection = (Invoke-MediaDetection $files['render_video']) }
    if ($renderStream.Count -eq 0 -or $renderAudio.Count -eq 0 -or $checks.render.duration_sec -le 0 -or $actualSize -ne $expectedSize) { throw ('Render media validation failed: ' + ($checks.render | ConvertTo-Json -Compress) + ' metadata=' + ($renderMetadata | ConvertTo-Json -Compress)) }

    $allPassed = $true
    foreach ($check in $checks.GetEnumerator()) {
        foreach ($property in $check.Value.GetEnumerator()) {
            if ($property.Key -eq 'detection') { continue }
            if ($property.Value -is [bool] -and -not $property.Value) { $allPassed = $false }
        }
    }
    $report = [ordered]@{
        schema_version = 'nh-media/real-local-artifact-validation/v1'
        evidence_input = [IO.Path]::GetFullPath($EvidenceInput)
        project_id = $projectId
        render_profile_key = $expectedProfile
        artifact_downloads = @($downloadEvidence)
        render_profile_snapshot = $renderProfileSnapshot
        checks = $checks
        all_checks_passed = $allPassed
        status = if ($allPassed) { 'PASS' } else { 'NOT_PASS' }
        finished_at = (Get-Date).ToUniversalTime().ToString('o')
    }
    $parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
    $report | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
    Get-Content -Raw -LiteralPath $EvidenceOutput
}
finally {
    if (Test-Path -LiteralPath $tempRoot) { Remove-Item -LiteralPath $tempRoot -Recurse -Force }
}
