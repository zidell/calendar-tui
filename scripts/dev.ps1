# Windows 개발 실행: 빌드해서 ~/.local/bin/calendar-tui.exe에 설치하고 Windows Terminal 새 창으로 띄운다.
# 윈도우는 실행 중인 exe를 덮어쓸 수 없고 자동 재시작도 없어(reload_windows.go) 떠 있던 calendar-tui를 끝내고 새로 띄운다.
# ~/.calendar-tui-google-client.json(Cloud 콘솔에서 받은 데스크톱 앱 클라이언트 JSON, 커밋 금지)이 있으면
# 릴리스처럼 구글 OAuth 기본 클라이언트를 넣어 빌드한다. 맥은 dev.sh.
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot)
$ldf = ''
$cj = Join-Path $HOME '.calendar-tui-google-client.json'
if (Test-Path $cj) {
    $c = (Get-Content $cj -Raw | ConvertFrom-Json).installed
    $ldf = "-X main.googleClientID=$($c.client_id) -X main.googleClientSecret=$($c.client_secret)"
}
go build -ldflags $ldf -o calendar.exe .
if ($LASTEXITCODE) { exit $LASTEXITCODE }
$bin = Join-Path $HOME '.local\bin'
New-Item -ItemType Directory -Force $bin | Out-Null
Get-Process calendar-tui -ErrorAction SilentlyContinue | Stop-Process
Start-Sleep -Milliseconds 300
Copy-Item calendar.exe (Join-Path $bin 'calendar-tui.exe') -Force
Start-Process wt.exe -ArgumentList '-w', 'new', (Join-Path $bin 'calendar-tui.exe')
