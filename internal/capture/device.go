package capture

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DeviceInfo 采集卡信息
type DeviceInfo struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	FPS       int    `json:"fps"`
	Formats   []VideoFormat `json:"formats,omitempty"`
}

// VideoFormat 支持的分辨率/帧率
type VideoFormat struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

// Probe 探测采集卡信息
func Probe(devicePath string, defaultW, defaultH, defaultFPS int) *DeviceInfo {
	info := &DeviceInfo{
		Path:   devicePath,
		Exists: false,
		Width:  defaultW,
		Height: defaultH,
		FPS:    defaultFPS,
	}

	// 自动探测设备路径
	if devicePath == "" || devicePath == "/dev/video0" {
		if found := autoDetectDevice(); found != "" {
			devicePath = found
			info.Path = found
			fmt.Printf("[capture] 自动探测到采集卡: %s\n", found)
		}
	}

	// 检查设备文件是否存在
	if _, err := os.Stat(devicePath); os.IsNotExist(err) {
		fmt.Printf("[capture] 设备 %s 不存在，使用默认参数\n", devicePath)
		return info
	}
	info.Exists = true

	// 使用 v4l2-ctl 获取支持的格式
	formats := probeFormats(devicePath)
	if len(formats) > 0 {
		info.Formats = formats
		// 取最高分辨率
		best := formats[0]
		info.Width = best.Width
		info.Height = best.Height
		info.FPS = best.FPS
		fmt.Printf("[capture] 探测到 %d 种格式，最佳: %dx%d@%d\n", len(formats), best.Width, best.Height, best.FPS)
	} else {
		fmt.Printf("[capture] v4l2-ctl 探测失败，使用默认参数 %dx%d@%d\n", defaultW, defaultH, defaultFPS)
	}

	return info
}

// probeFormats 通过 v4l2-ctl 获取支持的视频格式
func probeFormats(devicePath string) []VideoFormat {
	cmd := exec.Command("v4l2-ctl", "--device", devicePath, "--list-formats-ext")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("[capture] v4l2-ctl 执行失败: %v\n", err)
		return nil
	}

	return parseV4L2Output(string(output))
}

// parseV4L2Output 解析 v4l2-ctl --list-formats-ext 输出
func parseV4L2Output(output string) []VideoFormat {
	var formats []VideoFormat

	// 正则匹配 Size: Discrete 1920x1080 和 Interval: Discrete 0.017s (60.000 fps)
	sizeRe := regexp.MustCompile(`Size:\s+Discrete\s+(\d+)x(\d+)`)
	fpsRe := regexp.MustCompile(`Interval:\s+Discrete\s+[\d.]+s\s+\((\d+\.?\d*)\s+fps\)`)

	lines := strings.Split(output, "\n")
	var currentW, currentH int

	for _, line := range lines {
		if match := sizeRe.FindStringSubmatch(line); match != nil {
			currentW, _ = strconv.Atoi(match[1])
			currentH, _ = strconv.Atoi(match[2])
		}
		if match := fpsRe.FindStringSubmatch(line); match != nil && currentW > 0 {
			fpsFloat, _ := strconv.ParseFloat(match[1], 64)
			fps := int(fpsFloat)
			formats = append(formats, VideoFormat{
				Width:  currentW,
				Height: currentH,
				FPS:    fps,
			})
			currentW, currentH = 0, 0
		}
	}

	return formats
}

// autoDetectDevice 扫描 /dev/video0~9，返回第一个存在的设备
func autoDetectDevice() string {
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		if _, err := os.Stat(path); err == nil {
			// 确认是视频采集设备（有 v4l2 能力）
			cmd := exec.Command("v4l2-ctl", "-d", path, "--info")
			out, err := cmd.Output()
			if err == nil && strings.Contains(string(out), "Video Capture") {
				return path
			}
			// 即使没有 v4l2-ctl，设备存在就返回
			return path
		}
	}
	return ""
}
