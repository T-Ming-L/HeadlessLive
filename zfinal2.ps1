Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
# 恢复 uploads 目录
if (-not (Test-Path uploads)) { New-Item -ItemType Directory -Path uploads | Out-Null }
if (-not (Test-Path uploads\.gitkeep)) { New-Item -ItemType File -Path uploads\.gitkeep | Out-Null }
# 清理临时文件
Remove-Item c.txt, c2.txt, c3.txt, zzzz-final.ps1 -ErrorAction SilentlyContinue
# 提交
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 恢复 uploads/.gitkeep；清理临时脚本" *> final.txt
& $git status --short *> final-s.txt
Get-Content final.txt -Encoding Unicode
Write-Output "==== status ===="
Get-Content final-s.txt -Encoding Unicode
Write-Output "==== END ===="
