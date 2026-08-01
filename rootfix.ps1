# 文字源根因修复：测试 + 构建 + 提交
Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"

# 1. 测试
$env:GOOS = "windows"; $env:GOARCH = "amd64"
& "C:\Go\bin\go.exe" test ./... *> rtest.txt
$tcode = $LASTEXITCODE

# 2. 构建 Linux 二进制
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"
& "C:\Go\bin\go.exe" build -ldflags="-s -w" -o "$env:USERPROFILE\Desktop\HeadlessLive-linux-amd64" . *> rbuild.txt
$bcode = $LASTEXITCODE

# 3. 提交
& $git add -A 2>&1 | Out-Null
& $git commit -m "fix: 文字源从未进入渲染——IsVideoSource 缺少 SourceText，补上" *> rcommit.txt
$ccode = $LASTEXITCODE

Write-Output "test=$tcode build=$bcode commit=$ccode"
