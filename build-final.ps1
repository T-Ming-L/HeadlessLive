# 文字源修复：构建 + 提交 + 清理
Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"

# 1. 构建 Linux 二进制
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
& "C:\Go\bin\go.exe" build -ldflags="-s -w" -o "$env:USERPROFILE\Desktop\HeadlessLive-linux-amd64" . *> buildlog.txt
$bcode = $LASTEXITCODE

# 2. 提交
& $git add -A 2>&1 | Out-Null
& $git commit -m "fix: 文字源改用 fontfile 显式指定中文字体（font=Sans 只匹配英文 DejaVu 导致中文方块）" *> cmsg.txt
$ccode = $LASTEXITCODE

# 3. 清理临时文件（排除本脚本）
& $git clean -fd -e build-final.ps1 *> cl.txt
& $git status --short *> st.txt

Write-Output "build=$bcode commit=$ccode"
Get-Content cmsg.txt -Encoding Unicode | Select-Object -First 3
Write-Output "==== status ===="
Get-Content st.txt -Encoding Unicode
