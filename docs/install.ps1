# calendar-tui install / update (Windows)
#   irm https://zidell.github.io/calendar-tui/install.ps1 | iex
# Downloads the latest release (x64 or ARM64) to %LOCALAPPDATA%\Programs\calendar-tui\calendar-tui.exe
# and adds that folder to the user PATH. Run again to update (settings and sign-in are kept).
# Test overrides: BIN_DIR, ZIP_URL.
# Messages are ASCII only: Windows PowerShell 5.1 `irm` decodes a response without charset as Latin-1.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = 'zidell/calendar-tui'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$zipUrl = if ($env:ZIP_URL) { $env:ZIP_URL } else { "https://github.com/$repo/releases/latest/download/calendar-tui-windows-$arch.zip" }
$bin = if ($env:BIN_DIR) { $env:BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\calendar-tui' }

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("calendar-tui-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host 'Downloading...'
    Invoke-WebRequest -Uri $zipUrl -OutFile "$tmp\calendar-tui.zip" -UseBasicParsing
    Expand-Archive -Path "$tmp\calendar-tui.zip" -DestinationPath $tmp -Force
    if (-not (Test-Path "$tmp\calendar-tui.exe")) { throw 'calendar-tui.exe not found in the archive' }
    Unblock-File "$tmp\calendar-tui.exe"

    New-Item -ItemType Directory -Path $bin -Force | Out-Null
    $exe = Join-Path $bin 'calendar-tui.exe'
    # A running exe can't be overwritten but can be renamed; the old file is removed on the next install
    Remove-Item "$exe.old" -Force -ErrorAction SilentlyContinue
    if (Test-Path $exe) { Move-Item $exe "$exe.old" -Force }
    Copy-Item "$tmp\calendar-tui.exe" $exe
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

$ver = & $exe --version
Write-Host "Installed: $ver -> $exe"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $bin) {
    $sep = if ($userPath) { ';' } else { '' }
    [Environment]::SetEnvironmentVariable('Path', "$userPath$sep$bin", 'User')
    Write-Host "Added $bin to your PATH. Open a new terminal window to use it."
}

Write-Host @'

Run calendar-tui in a terminal (Windows Terminal recommended).
On first launch you sign in to Google in your browser. If you see
"Google hasn't verified this app", click Advanced -> Go to calendar-tui.
'@
