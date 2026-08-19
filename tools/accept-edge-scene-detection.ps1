[CmdletBinding()]
param(
    [string]$MediaPath = 'C:\Users\PC\AppData\Local\NH-Media\evidence\real-local-corpus-20260819-acceptance\corpus-edge-fastcuts-static-silence.mp4',
    [string]$EvidenceOutput = 'docs/evidence/real-local-edge-scene-detection-20260819.json',
    [string]$FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH,
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH
)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath $MediaPath -PathType Leaf)) { throw 'Edge corpus media was not found.' }
if ([string]::IsNullOrWhiteSpace($FfmpegPath) -or [string]::IsNullOrWhiteSpace($FfprobePath)) { throw 'Reviewed FFmpeg paths are required.' }
$env:NH_MEDIA_EDGE_MEDIA = [IO.Path]::GetFullPath($MediaPath)
$env:NH_MEDIA_FFMPEG_PATH = $FfmpegPath
$env:NH_MEDIA_FFPROBE_PATH = $FfprobePath
$code = "import json, os; from nh_media.gate_g import detect_scenes_from_media; scenes, features = detect_scenes_from_media(os.environ['NH_MEDIA_EDGE_MEDIA'], 'edge-source', os.environ['NH_MEDIA_FFMPEG_PATH'], os.environ['NH_MEDIA_FFPROBE_PATH'], 0.2); print(json.dumps({'scene_count': len(scenes), 'scene_durations': [round(x.end_sec-x.start_sec,3) for x in scenes], 'feature_count': len(features), 'keyframe_count': sum(len(x.keyframe_refs) for x in features)}))"
$raw = & uv run --project (Join-Path $PSScriptRoot '..\services\ml-worker') python -c $code
if ($LASTEXITCODE -ne 0) { throw 'Real scene detector failed on edge corpus.' }
$result = ($raw | Select-Object -Last 1) | ConvertFrom-Json
$report = [ordered]@{
    schema_version = 'nh-media/real-local-edge-scene-detection/v1'
    media_path = [IO.Path]::GetFullPath($MediaPath)
    scene_count = [int]$result.scene_count
    scene_durations = @($result.scene_durations)
    feature_count = [int]$result.feature_count
    keyframe_count = [int]$result.keyframe_count
    status = if ($result.scene_count -gt 0 -and $result.feature_count -eq $result.scene_count -and $result.keyframe_count -ge $result.scene_count) { 'PASS' } else { 'NOT_PASS' }
    finished_at = (Get-Date).ToUniversalTime().ToString('o')
}
$parent = Split-Path -Parent ([IO.Path]::GetFullPath($EvidenceOutput))
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$report | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $EvidenceOutput -Encoding UTF8
Get-Content -Raw -LiteralPath $EvidenceOutput
