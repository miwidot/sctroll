# SCTroll signed build script
# Requires Certum SimplySign Desktop running and logged in (mobile app auth)

$ErrorActionPreference = "Stop"

$exePath = "build\bin\SCTroll.exe"
$signtool = "C:\Program Files (x86)\Windows Kits\10\bin\10.0.28000.0\x64\signtool.exe"
$timestamp = "http://time.certum.pl"
$subject = "Open Source Developer Martin Wilke"

Write-Host "[1/3] Building SCTroll..." -ForegroundColor Cyan
wails build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $exePath)) {
    Write-Host "Exe not found at $exePath" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[2/3] Signing..." -ForegroundColor Cyan
Write-Host "Make sure SimplySign Desktop is running and logged in via mobile app!" -ForegroundColor Yellow
Write-Host ""

& $signtool sign /n $subject /fd SHA256 /td SHA256 /tr $timestamp /v $exePath
if ($LASTEXITCODE -ne 0) {
    Write-Host "Signing failed!" -ForegroundColor Red
    Write-Host "Check: 1) SimplySign Desktop running? 2) Mobile app logged in?" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "[3/3] Verifying signature..." -ForegroundColor Cyan
& $signtool verify /pa /v $exePath

Write-Host ""
Write-Host "Done! Signed exe: $exePath" -ForegroundColor Green
