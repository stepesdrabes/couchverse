# Build a standalone Couchpush.exe with PyInstaller.
# Prereqs (one time):  .\.venv\Scripts\Activate.ps1 ; pip install -r requirements.txt pyinstaller
# Output: dist\Couchpush.exe  (single file, no Python needed to run)
param([string]$OutputDirectory = 'dist')
$ErrorActionPreference = "Stop"
Set-Location -LiteralPath $PSScriptRoot

$pushOutputPath = if ([IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory
} else {
    Join-Path $PSScriptRoot $OutputDirectory
}
& .\.venv\Scripts\python.exe -m PyInstaller --noconfirm --distpath $pushOutputPath Couchpush.spec

if ($LASTEXITCODE -ne 0) {
    throw "CouchPush build failed (exit code $LASTEXITCODE)"
}

$pushSmoke = Start-Process -FilePath (Join-Path $pushOutputPath 'Couchpush.exe') `
    -ArgumentList '--smoke-test' -WindowStyle Hidden -Wait -PassThru
if ($pushSmoke.ExitCode -ne 0) {
    throw "CouchPush packaged startup check failed (exit code $($pushSmoke.ExitCode))"
}

Write-Host "Built: $(Join-Path $pushOutputPath 'Couchpush.exe')"
