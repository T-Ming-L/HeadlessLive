Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
& "C:\Go\bin\go.exe" test ./internal/scene/ ./internal/model/ *> x1.txt
$t = $LASTEXITCODE
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"
& "C:\Go\bin\go.exe" build -ldflags="-s -w" -o "$env:USERPROFILE\Desktop\HeadlessLive-linux-amd64" . *> x2.txt
$b = $LASTEXITCODE
& $git add -A 2>&1 | Out-Null
& $git commit -m "fix: format=rgba 应为独立滤镜（color 源无 format 参数），放 drawtext 前" *> x3.txt
$c = $LASTEXITCODE
Write-Output "test=$t build=$b commit=$c"
