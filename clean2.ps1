Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
# 确认 Linux 二进制
Get-Item "$env:USERPROFILE\Desktop\HeadlessLive-linux-amd64" | Select-Object Name,Length,LastWriteTime | Out-File -Encoding utf8 rbuild-check.txt
# 清理临时文件
Remove-Item rtest.txt, rbuild.txt, rcommit.txt, rootfix.ps1, viewfix.ps1 -ErrorAction SilentlyContinue
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 清理临时脚本" *> rclean.txt
& $git log --oneline -2 *> rlog2.txt
Get-Content rbuild-check.txt -Encoding utf8
Write-Output "==== log ===="
Get-Content rlog2.txt -Encoding utf8
Write-Output "==== clean ===="
Get-Content rclean.txt -Encoding utf8 | Select-Object -First 2
