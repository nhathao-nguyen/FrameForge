[CmdletBinding()]
param(
    [string]$OutputDirectory = (Join-Path $env:LOCALAPPDATA ('NH-Media\evidence\real-local-corpus-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))),
    [string]$FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH,
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($FfmpegPath) -or -not (Test-Path -LiteralPath $FfmpegPath -PathType Leaf)) { throw 'Reviewed FFmpeg path is required.' }
if ([string]::IsNullOrWhiteSpace($FfprobePath) -or -not (Test-Path -LiteralPath $FfprobePath -PathType Leaf)) { throw 'Reviewed ffprobe path is required.' }
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$narration = Join-Path $OutputDirectory 'narration.wav'
$srt = Join-Path $OutputDirectory 'corpus.srt'
$landscape = Join-Path $OutputDirectory 'corpus-landscape.mp4'
$landscapeWithSubtitles = Join-Path $OutputDirectory 'corpus-landscape-with-subtitles.mp4'
$portrait = Join-Path $OutputDirectory 'corpus-portrait.mp4'

uv run --project (Join-Path $PSScriptRoot '..\services\ml-worker') python (Join-Path $PSScriptRoot 'real-local-corpus.py') --output $narration
if ($LASTEXITCODE -ne 0) { throw 'Real local TTS corpus generation failed.' }

@'
1
00:00:00,000 --> 00:00:03,000
At dawn, Mira enters the quiet station.

2
00:00:03,000 --> 00:00:06,000
She finds a map beneath the old clock.

3
00:00:06,000 --> 00:00:09,000
The lights go dark beyond the bridge.

4
00:00:09,000 --> 00:00:12,000
Mira chooses the road home.
'@ | Set-Content -LiteralPath $srt -Encoding UTF8

$lavfi = @(
    'color=c=darkred:s=1280x720:r=24:d=3',
    'color=c=steelblue:s=1280x720:r=24:d=3',
    'color=c=black:s=1280x720:r=24:d=3',
    'color=c=darkgreen:s=1280x720:r=24:d=3'
)
$args = @('-hide_banner', '-loglevel', 'error')
foreach ($input in $lavfi) { $args += @('-f', 'lavfi', '-i', $input) }
$args += @('-i', $narration, '-f', 'lavfi', '-i', 'sine=frequency=220:sample_rate=48000:duration=12', '-i', $srt)
$args += @('-filter_complex', '[0:v][1:v][2:v][3:v]concat=n=4:v=1:a=0,format=yuv420p[v];[4:a]volume=1.0[voice];[5:a]volume=0.06[bgm];[voice][bgm]amix=inputs=2:duration=first:dropout_transition=0[a]', '-map', '[v]', '-map', '[a]', '-map', '6:0', '-t', '12', '-c:v', 'libx264', '-preset', 'veryfast', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-b:a', '128k', '-c:s', 'mov_text', '-metadata', 'comment=NH-Media synthetic rights-safe corpus', '-y', $landscapeWithSubtitles)
& $FfmpegPath @args
if ($LASTEXITCODE -ne 0) { throw 'Landscape corpus FFmpeg build failed.' }

# Asset validation intentionally rejects non-audio/video streams. Keep the
# subtitle-bearing source as corpus evidence and upload the validated media
# variant while retaining corpus.srt as the sidecar subtitle input.
& $FfmpegPath '-hide_banner' '-loglevel' 'error' '-i' $landscapeWithSubtitles '-map' '0:v:0' '-map' '0:a:0' '-c' 'copy' '-y' $landscape
if ($LASTEXITCODE -ne 0) { throw 'Validated landscape corpus variant build failed.' }

& $FfmpegPath '-hide_banner' '-loglevel' 'error' '-i' $landscape '-vf' 'scale=720:1280:force_original_aspect_ratio=decrease,pad=720:1280:(ow-iw)/2:(oh-ih)/2:color=black' '-c:v' 'libx264' '-pix_fmt' 'yuv420p' '-c:a' 'copy' '-c:s' 'copy' '-y' $portrait
if ($LASTEXITCODE -ne 0) { throw 'Portrait corpus FFmpeg build failed.' }

$landscapeProbe = & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $landscape | ConvertFrom-Json
$portraitProbe = & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $portrait | ConvertFrom-Json
$manifest = [ordered]@{
    schema_version = 'nh-media/real-local-corpus/v1'
    rights_status = 'owned-generated-test-media'
    source_text = 'Generated narration and color-card scenes; no third-party footage, image, voice or music.'
    landscape = [ordered]@{ path = $landscape; sha256 = (Get-FileHash -LiteralPath $landscape -Algorithm SHA256).Hash.ToLowerInvariant(); bytes = (Get-Item -LiteralPath $landscape).Length; streams = $landscapeProbe.streams }
    landscape_with_subtitles = [ordered]@{ path = $landscapeWithSubtitles; sha256 = (Get-FileHash -LiteralPath $landscapeWithSubtitles -Algorithm SHA256).Hash.ToLowerInvariant(); bytes = (Get-Item -LiteralPath $landscapeWithSubtitles).Length }
    portrait = [ordered]@{ path = $portrait; sha256 = (Get-FileHash -LiteralPath $portrait -Algorithm SHA256).Hash.ToLowerInvariant(); bytes = (Get-Item -LiteralPath $portrait).Length; streams = $portraitProbe.streams }
    narration = [ordered]@{ path = $narration; sha256 = (Get-FileHash -LiteralPath $narration -Algorithm SHA256).Hash.ToLowerInvariant(); bytes = (Get-Item -LiteralPath $narration).Length }
    subtitles = [ordered]@{ path = $srt; sha256 = (Get-FileHash -LiteralPath $srt -Algorithm SHA256).Hash.ToLowerInvariant() }
}
$manifestPath = Join-Path $OutputDirectory 'manifest.json'
$manifest | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $manifestPath -Encoding UTF8
Get-Content -Raw -LiteralPath $manifestPath
