# 查看修复结果
Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
Get-Content rtest.txt -Encoding Unicode | Select-Object -Last 5
Write-Output "==== build ===="
Get-Content rbuild.txt -Encoding Unicode | Select-Object -Last 3
Write-Output "==== commit ===="
Get-Content rcommit.txt -Encoding Unicode | Select-Object -First 3
Write-Output "==== log ===="
& $git log --oneline -1 *> rlog.txt
Get-Content rlog.txt -Encoding Unicode
