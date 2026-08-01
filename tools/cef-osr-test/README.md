# CEF OSR 验证程序（cef-osr-test）

验证 `energye/cef` 的 **OSR 离屏渲染** 在 N100 上的三项关键能力：

1. **真透明** —— `OnPaint` 原始 buffer 的 alpha 通道是否保留
2. **帧率** —— `SetWindowlessFrameRate` 下实际回调帧率
3. **CPU** —— 本进程 + chromium 子进程的总占用

## 推荐流程：WSL2 隔离构建（服务器零编译环境）

编译环境全在 Windows 的 WSL2 里，N100 服务器只解压运行，不装任何编译工具，清理 = `wsl --unregister Ubuntu`。

```powershell
# 1. Windows 侧装 WSL2（一次）
wsl --install -d Ubuntu

# 2. 把本目录拷进 WSL（建议放家目录，避免 /mnt 性能问题）
wsl
cp -r /mnt/e/WORK/Web-RTMP/tools/cef-osr-test ~/cef-osr-test
cd ~/cef-osr-test

# 3. 一键构建（自动装 Go/GTK/CEF 二进制 + 编译 + 打包，全程国内镜像）
bash build-linux-cef.sh

# 4. 产物在 dist/cef-osr-test-linux.tar.gz，拷到 N100
exit
scp e:\WORK\Web-RTMP\tools\cef-osr-test\dist\cef-osr-test-linux.tar.gz root@N100:~/
```

N100 上：

```bash
tar xzf cef-osr-test-linux.tar.gz && cd cef-osr-test-linux
pkill Xvfb; Xvfb :99 -screen 0 1920x1080x24 -ac &
sudo apt install -y libgtk-3-0   # 若提示缺 GTK 运行时（几 MB）
DISPLAY=:99 ./run.sh -fps 30 -duration 20 -log test.log
```

`run.sh` 会自动用包内的 CEF 运行库（`LD_LIBRARY_PATH` 指向 `runtime/`），**不污染服务器系统**。

---

## 备用：直接在 N100 上搭建（不推荐，会留编译残留）

```bash
# 1. 装 GTK3（energye/cef Linux 依赖）
sudo apt install -y libgtk-3-dev

# 2. 下载 liblcl + CEF 二进制（Linux64），解压到 /opt/cef
cd /tmp
wget https://sourceforge.net/projects/liblcl/files/v3.0.0/lcl_cef_binary_linux64.zip/download -O lcl_cef.zip
unzip lcl_cef.zip -d /opt/cef
export ENERGY_HOME=/opt/cef   # 建议写进 ~/.bashrc

# 3. 确认 libenergy 库存在（147 版）
ls -la $ENERGY_HOME/libenergy-gtk3-147.so

# 4. 编译
cd ~/cef-osr-test
go mod tidy
go build -o cef-osr-test .
```

## 运行

```bash
# 先起 Xvfb（CEF 在 Linux 上即使 OSR 也要 X 环境）
pkill Xvfb; Xvfb :99 -screen 0 1920x1080x24 -ac &

# 跑 20 秒验证（30fps, 1280x720）
DISPLAY=:99 ENERGY_HOME=/opt/cef ./cef-osr-test -fps 30 -duration 20 -log test.log
```

## 怎么读结果

日志（stdout + `test.log`）每秒一行：

```
fps=29.8  cpu=45.2%  帧=596  透明%=95.10 半透明%=0.05 不透明%=4.85
```

结束时有汇总：

```
总帧数: 596  平均 fps: 29.8（目标 30）
平均 CPU（含 chromium 子进程）: 45.2%
alpha 统计: 透明 95.10% / 半透明 0.05% / 不透明 4.85%
结论: 透明像素占比 > 50% → OSR 真透明验证通过 ✅
```

| 指标 | 判定标准 |
|---|---|
| 平均 fps | ≥ 目标 × 0.9（如 30fps 目标要 ≥ 27） |
| 透明 % | 测试页背景是透明的，应 > 50%（红块+文字只占小部分） |
| CPU | N100 上 1280x720 渲染，合理范围 30~80%；远高于此则考虑降分辨率/帧率 |

## 参数

```
-url <URL>        加载指定网页（默认生成本地透明测试页）
-width 1280       渲染宽
-height 720       渲染高
-fps 30           目标帧率（1-60）
-duration 15      测试秒数
-log test.log     日志文件
-verbose          DEBUG 级别日志
-lib <path>       libenergy 路径（默认 $ENERGY_HOME/libenergy-gtk3-147.so）
```

## 常见问题

- **找不到 libenergy** → 检查 `ENERGY_HOME` 和文件是否在 `/opt/cef/libenergy-gtk3-147.so`
- **启动即崩 / 打不开 DISPLAY** → 确认 Xvfb 在跑、`DISPLAY=:99` 已设
- **root 下报沙箱错** → 程序已内置 `SetNoSandbox(true)`
- **编译报某个 API 不存在** → 把报错发我，energye/cef 有多版本目录（109/127/147），可能要用对应版本
