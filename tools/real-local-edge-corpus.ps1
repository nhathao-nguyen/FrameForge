[CmdletBinding()]
param(
    [string]$OutputDirectory = (Join-Path $env:LOCALAPPDATA 'NH-Media\evidence\real-local-corpus-20260819-acceptance'),
    [string]$FfmpegPath = $env:NH_MEDIA_FFMPEG_PATH,
    [string]$FfprobePath = $env:NH_MEDIA_FFPROBE_PATH
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($FfmpegPath) -or -not (Test-Path -LiteralPath $FfmpegPath -PathType Leaf)) { throw 'Reviewed FFmpeg path is required.' }
if ([string]::IsNullOrWhiteSpace($FfprobePath) -or -not (Test-Path -LiteralPath $FfprobePath -PathType Leaf)) { throw 'Reviewed ffprobe path is required.' }
New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$edge = Join-Path $OutputDirectory 'corpus-edge-fastcuts-static-silence.mp4'

$videoInputs = @(
    'color=c=darkred:s=640x360:r=24:d=0.5',
    'color=c=steelblue:s=640x360:r=24:d=0.5',
    'color=c=darkgreen:s=640x360:r=24:d=0.5',
    'color=c=gold:s=640x360:r=24:d=0.5',
    'color=c=steelblue:s=640x360:r=24:d=4',
    'color=c=darkred:s=640x360:r=24:d=6'
)
$args = @('-hide_banner', '-loglevel', 'error')
foreach ($input in $videoInputs) { $args += @('-f', 'lavfi', '-i', $input) }
$args += @(
    '-f', 'lavfi', '-i', 'sine=frequency=220:sample_rate=48000:duration=2',
    '-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo:d=2',
    '-f', 'lavfi', '-i', 'sine=frequency=330:sample_rate=48000:duration=2',
    '-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo:d=6',
    '-filter_complex', '[0:v][1:v][2:v][3:v][4:v][5:v]concat=n=6:v=1:a=0,format=yuv420p[v];[6:a]volume=0.08[a0];[8:a]volume=0.03[a2];[a0][7:a][a2][9:a]concat=n=4:v=0:a=1[a]',
    '-map', '[v]', '-map', '[a]', '-t', '12', '-c:v', 'libx264', '-preset', 'veryfast', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-b:a', '128k', '-metadata', 'comment=NH-Media synthetic edge corpus: fast cuts, static, quiet and silence', '-y', $edge
)
& $FfmpegPath @args
if ($LASTEXITCODE -ne 0) { throw 'Edge corpus FFmpeg build failed.' }

$probe = & $FfprobePath '-v' 'error' '-of' 'json' '-show_format' '-show_streams' $edge | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Edge corpus ffprobe validation failed.' }
$blackLog = (& $FfmpegPath '-hide_banner' '-nostats' '-loglevel' 'info' '-i' $edge '-vf' 'blackdetect=d=0.25:pix_th=0.1' '-an' '-f' 'null' 'NUL' 2>&1 | Out-String)
$silenceLog = (& $FfmpegPath '-hide_banner' '-nostats' '-loglevel' 'info' '-i' $edge '-af' 'silencedetect=n=-35dB:d=0.25' '-vn' '-f' 'null' 'NUL' 2>&1 | Out-String)
$manifest = [ordered]@{
    schema_version = 'nh-media/real-local-edge-corpus/v1'
    rights_status = 'owned-generated-test-media'
    path = $edge
    sha256 = (Get-FileHash -LiteralPath $edge -Algorithm SHA256).Hash.ToLowerInvariant()
    bytes = (Get-Item -LiteralPath $edge).Length
    streams = $probe.streams
    designed_segments = @(
        [ordered]@{ start_sec = 0; end_sec = 2; kind = 'fast_cuts'; detail = 'four 0.5-second color changes' },
        [ordered]@{ start_sec = 2; end_sec = 6; kind = 'static'; detail = 'four-second steelblue card' },
        [ordered]@{ start_sec = 6; end_sec = 12; kind = 'static'; detail = 'six-second darkred card' },
        [ordered]@{ start_sec = 0; end_sec = 2; kind = 'quiet_audio'; detail = 'low-level sine bed' },
        [ordered]@{ start_sec = 2; end_sec = 4; kind = 'silence'; detail = 'anullsrc gap' },
        [ordered]@{ start_sec = 4; end_sec = 6; kind = 'quiet_audio'; detail = 'lower-level sine bed' },
        [ordered]@{ start_sec = 6; end_sec = 12; kind = 'silence'; detail = 'trailing anullsrc gap' }
    )
    detection = [ordered]@{
        blackdetect_observed = ($blackLog -match 'black_start:')
        silencedetect_observed = ($silenceLog -match 'silence_start:')
    }
    generated_at = (Get-Date).ToUniversalTime().ToString('o')
}
$manifestPath = Join-Path $OutputDirectory 'edge-manifest.json'
$manifest | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath $manifestPath -Encoding UTF8
Get-Content -Raw -LiteralPath $manifestPath
