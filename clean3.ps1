Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
Remove-Item a.txt, b.txt, c.txt, ab.ps1, clean2.ps1, rbuild-check.txt, rclean.txt, rlog2.txt -ErrorAction SilentlyContinue
& $git clean -fd -e clean3.ps1 *> d.txt
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 清理临时文件" *> e.txt
& $git status --short *> f.txt
Write-Output "==== commit ===="
Get-Content e.txt -Encoding utf8 | Select-Object -First 2
Write-Output "==== status ===="
Get-Content f.txt -Encoding utf8
Write-Output "==== END ===="
