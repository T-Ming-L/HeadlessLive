# 临时提交脚本
Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
Remove-Item test.ps1, test-out.txt, test-exit.txt, fe-build.ps1, fe-build.txt, build-bin.ps1, build-linux.txt, build-win.txt, build-exit.txt -ErrorAction SilentlyContinue
& $git restore static/.gitkeep 2>&1 | Out-Null
& $git add -A 2>&1 | Out-Null
& $git commit -m "feat: 移除浏览器源（Xvfb+Chromium+x11grab）——运行库/依赖成本过高不达完美标准；保留屏幕捕获源；删除 CEF OSR 验证工具" *> commit.txt
$code = $LASTEXITCODE
Set-Content -Path commit-exit.txt -Value $code
Write-Output "DONE code=$code"
