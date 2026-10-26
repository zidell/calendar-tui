# Windows 개발 실행: 빌드해서 ~/.local/bin/calendar-tui.exe로 바꾸고, 떠 있으면 그 창에서 다시 시작하게 한다.
# 실행 중인 exe는 덮어쓸 수 없지만 이름은 바꿀 수 있어, 옛 파일을 *.old로 옮기고 새 빌드를 같은 자리에 둔다.
# 실행 중인 calendar-tui가 바뀐 파일을 감지해 같은 창에서 새 빌드로 다시 시작한다(reload_windows.go).
# 떠 있지 않으면: 이 exe를 가리키는 tuidock 앱이 있으면 그 앱으로(시작 메뉴 바로가기), 없으면 Windows Terminal 새 창으로 띄운다.
# ~/.calendar-tui-google-client.json(Cloud 콘솔에서 받은 데스크톱 앱 클라이언트 JSON, 커밋 금지)이 있으면
# 릴리스처럼 구글 OAuth 기본 클라이언트를 넣어 빌드한다. 맥은 dev.sh.
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot)
# 에이전트 셸(Claude Code PowerShell 도구)이 넣은 NO_COLOR가 띄운 앱까지 물려가 색이 꺼지는 것을 막는다
Remove-Item env:NO_COLOR -ErrorAction SilentlyContinue

$ldf = '-X main.version=dev'
$cj = Join-Path $HOME '.calendar-tui-google-client.json'
if (Test-Path $cj) {
    $c = (Get-Content $cj -Raw | ConvertFrom-Json).installed
    $ldf += " -X main.googleClientID=$($c.client_id) -X main.googleClientSecret=$($c.client_secret)"
}
go build -ldflags $ldf -o calendar.exe .
if ($LASTEXITCODE) { exit $LASTEXITCODE }

$bin = Join-Path $HOME '.local\bin'
$exe = Join-Path $bin 'calendar-tui.exe'
New-Item -ItemType Directory -Force $bin | Out-Null
# 지난번에 옮긴 옛 파일 중 이제 안 쓰는 것을 지운다(아직 실행 중인 것은 지워지지 않아 남는다)
Get-ChildItem $bin -Filter 'calendar-tui.exe.*.old' | Remove-Item -Force -ErrorAction SilentlyContinue
$running = @(Get-Process calendar-tui -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe }).Count -gt 0
if (Test-Path $exe) { Move-Item $exe "$exe.$(Get-Date -Format yyyyMMddHHmmss).old" }
Copy-Item calendar.exe $exe
if ($running) { Write-Host '실행 중인 창에서 다시 시작합니다'; exit 0 }

$app = Get-ChildItem (Join-Path $env:APPDATA 'tuidock') -Filter app.toml -Recurse -ErrorAction SilentlyContinue |
    Where-Object { (Get-Content $_.FullName -Raw) -match ('command = "' + [regex]::Escape(($exe -replace '\\', '/')) + '"') } |
    Select-Object -First 1
if ($app) {
    $name = ((Get-Content $app.FullName -Raw) | Select-String 'name = "([^"]+)"').Matches[0].Groups[1].Value
    $lnk = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\TUIDock\$name.lnk"
    if (Test-Path $lnk) { Start-Process explorer.exe -ArgumentList "`"$lnk`""; exit 0 }
}
Start-Process wt.exe -ArgumentList '-w', 'new', $exe
