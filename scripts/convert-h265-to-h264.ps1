#requires -Version 5.1
<#
Convert HEVC (h265) .mkv files to H.264 .mp4, recursively, season folder by
season folder. Each output is verified (ffmpeg exit code + duration match)
before the original .mkv is permanently deleted.

Output is MP4 so it direct-plays on the Pi with zero transcoding. Video is
re-encoded to 8-bit h264 (browser-safe); already-h264 files are remuxed (no
quality loss). AAC/MP3 audio is copied; other audio is re-encoded to AAC.
Embedded text subtitles are saved as sidecar .srt before the original is removed.

WARNING: h264 is less efficient than h265, so the new files are usually LARGER
than the originals. You will not save disk space - you trade space for playback
that costs the Pi nothing.

Usage (from PowerShell):
  # dry run first - shows what it would do, changes nothing:
  powershell -ExecutionPolicy Bypass -File .\convert-h265-to-h264.ps1 -Root "D:\Media\The Simpsons" -DryRun

  # real run:
  powershell -ExecutionPolicy Bypass -File .\convert-h265-to-h264.ps1 -Root "D:\Media\The Simpsons"

  # keep originals (no delete), or use the GPU (much faster), or change quality:
  ... -KeepOriginals
  ... -Encoder nvenc        # nvenc (NVIDIA), qsv (Intel), amf (AMD)
  ... -Crf 20               # lower = better quality + bigger file (18 near-lossless, 23 smaller)

Needs ffmpeg + ffprobe on PATH. Get a "full" Windows build from
https://www.gyan.dev/ffmpeg/builds/ and add its bin folder to PATH.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string]$Root,
    [int]$Crf = 18,
    [string]$Preset = "medium",
    [ValidateSet("libx264", "nvenc", "qsv", "amf")] [string]$Encoder = "libx264",
    [switch]$KeepOriginals,
    [switch]$DryRun,
    [switch]$NoExtractSubs,
    [string]$FfmpegPath = "ffmpeg",
    [string]$FfprobePath = "ffprobe"
)

$ErrorActionPreference = "Stop"
# ffprobe emits UTF-8 JSON; decode native output as UTF-8 so accented names survive
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}

function Assert-Tool($name) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "$name not found on PATH. Install an ffmpeg full build and add its bin folder to PATH."
    }
}
Assert-Tool $FfmpegPath
Assert-Tool $FfprobePath
if (-not (Test-Path -LiteralPath $Root)) { throw "Root folder not found: $Root" }

$logPath = Join-Path $Root ("convert-log-{0}.txt" -f (Get-Date -Format "yyyyMMdd-HHmmss"))
function Log($msg) {
    $line = "{0}  {1}" -f (Get-Date -Format "HH:mm:ss"), $msg
    Write-Host $line
    Add-Content -LiteralPath $logPath -Value $line
}

function Probe-File($path) {
    $raw = & $FfprobePath -v error -show_format -show_streams -of json -i "$path" 2>$null
    if (-not $raw) { return $null }
    try { return ($raw | ConvertFrom-Json) } catch { return $null }
}
function To-Double($v) { $d = 0.0; [double]::TryParse("$v", [ref]$d) | Out-Null; return $d }
function Human($bytes) { "{0:N2} GB" -f ($bytes / 1GB) }

# video encoder args for a real h265 -> h264 re-encode
function Video-Args {
    switch ($Encoder) {
        "libx264" { @("-c:v", "libx264", "-preset", $Preset, "-crf", "$Crf", "-pix_fmt", "yuv420p") }
        "nvenc"   { @("-c:v", "h264_nvenc", "-preset", "p6", "-tune", "hq", "-rc", "vbr", "-cq", "$Crf", "-b:v", "0", "-pix_fmt", "yuv420p") }
        "qsv"     { @("-c:v", "h264_qsv", "-preset", "veryslow", "-global_quality", "$Crf", "-pix_fmt", "nv12") }
        "amf"     { @("-c:v", "h264_amf", "-rc", "cqp", "-qp_i", "$Crf", "-qp_p", "$Crf", "-quality", "quality", "-pix_fmt", "yuv420p") }
    }
}

$textSubCodecs  = @("subrip", "ass", "ssa", "mov_text", "webvtt", "srt")
$copyAudioCodecs = @("aac", "mp3")
$converted = 0; $skipped = 0; $failed = 0
[int64]$origTotal = 0; [int64]$newTotal = 0

Log "Root: $Root"
Log "Encoder=$Encoder  CRF=$Crf  Preset=$Preset  DryRun=$DryRun  KeepOriginals=$KeepOriginals"

# process each top-level folder (the 36 seasons) in order; if Root has no
# subfolders, process Root itself
$folders = @(Get-ChildItem -LiteralPath $Root -Directory | Sort-Object Name)
if ($folders.Count -eq 0) { $folders = @(Get-Item -LiteralPath $Root) }

foreach ($folder in $folders) {
    Log "==== Folder: $($folder.Name) ===="
    $files = @(Get-ChildItem -LiteralPath $folder.FullName -Filter *.mkv -File -Recurse | Sort-Object FullName)
    foreach ($file in $files) {
        $in   = $file.FullName
        $base = Join-Path $file.DirectoryName ([System.IO.Path]::GetFileNameWithoutExtension($in))
        $out  = "$base.mp4"
        $tmp  = "$base.converting.mp4"

        try {
            if (Test-Path -LiteralPath $out) { Log "SKIP (mp4 already exists): $($file.Name)"; $skipped++; continue }

            $probe = Probe-File $in
            if (-not $probe) { Log "SKIP (cannot probe): $($file.Name)"; $skipped++; continue }
            $v = $probe.streams | Where-Object { $_.codec_type -eq "video" } | Select-Object -First 1
            if (-not $v) { Log "SKIP (no video stream): $($file.Name)"; $skipped++; continue }

            $vcodec = "$($v.codec_name)".ToLower()
            $inDur  = To-Double $probe.format.duration
            $a      = $probe.streams | Where-Object { $_.codec_type -eq "audio" } | Select-Object -First 1
            $acodec = if ($a) { "$($a.codec_name)".ToLower() } else { "" }

            if ($vcodec -eq "h264") { $vargs = @("-c:v", "copy") } else { $vargs = Video-Args }
            $maps = @("-map", "0:v:0")
            $aargs = @()
            if ($a) {
                $maps += @("-map", "0:a?")
                if ($copyAudioCodecs -contains $acodec) { $aargs = @("-c:a", "copy") }
                else { $aargs = @("-c:a", "aac", "-b:a", "192k") }
            }

            $action = if ($vcodec -eq "h264") { "remux" } else { "$vcodec -> h264" }
            if ($DryRun) { Log "WOULD CONVERT ($action): $($file.Name)"; $skipped++; continue }

            Log "CONVERT ($action): $($file.Name)"
            $ff = @("-hide_banner", "-loglevel", "warning", "-stats", "-y", "-i", $in) + $maps + $vargs + $aargs +
                  @("-movflags", "+faststart", "-sn", "-dn", "-map_metadata", "0", "-map_chapters", "0", $tmp)
            & $FfmpegPath @ff
            $code = $LASTEXITCODE

            # verify before we trust it enough to delete the source
            $ok = ($code -eq 0) -and (Test-Path -LiteralPath $tmp)
            if ($ok) {
                $op = Probe-File $tmp
                $outDur = if ($op) { To-Double $op.format.duration } else { 0 }
                $size = (Get-Item -LiteralPath $tmp).Length
                $ok = ($outDur -gt 0) -and ($size -gt 100KB) -and ($inDur -le 0 -or $outDur -ge $inDur * 0.98)
            }
            if (-not $ok) {
                Log "FAILED (ffmpeg exit $code or verify failed): $($file.Name) - original kept"
                if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
                $failed++; continue
            }

            Move-Item -LiteralPath $tmp -Destination $out -Force

            # preserve embedded text subtitles as sidecar .srt (source still exists)
            if (-not $NoExtractSubs) {
                $subs = @($probe.streams | Where-Object { $_.codec_type -eq "subtitle" -and $textSubCodecs -contains "$($_.codec_name)".ToLower() })
                $idx = 0
                foreach ($s in $subs) {
                    $lang = if ($s.tags -and $s.tags.language) { "$($s.tags.language)" } else { "sub$idx" }
                    $srt = "$base.$lang.srt"
                    if (-not (Test-Path -LiteralPath $srt)) {
                        & $FfmpegPath -hide_banner -loglevel error -y -i $in -map "0:$($s.index)" -c:s srt "$srt" 2>$null
                    }
                    $idx++
                }
            }

            $origTotal += $file.Length
            $newTotal  += (Get-Item -LiteralPath $out).Length
            if (-not $KeepOriginals) {
                Remove-Item -LiteralPath $in -Force
                Log "DONE + deleted original: $($file.Name)"
            } else {
                Log "DONE (kept original): $($file.Name)"
            }
            $converted++
        }
        catch {
            Log "ERROR: $($file.Name) - $($_.Exception.Message)"
            if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
            $failed++
        }
    }
}

Log "==== Summary: converted=$converted  skipped=$skipped  failed=$failed ===="
if ($converted -gt 0 -and -not $DryRun) {
    Log ("Size of originals processed: {0}  ->  new files: {1}  (net {2})" -f (Human $origTotal), (Human $newTotal), (Human ($newTotal - $origTotal)))
}
Log "Log saved to: $logPath"
