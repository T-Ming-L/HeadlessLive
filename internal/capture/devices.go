package capture

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Device 系统音视频设备
type Device struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	USB    bool   `json:"usb,omitempty"` // 是否为 USB 设备（即插即用）
}

// ListVideoDevices 扫描 /dev/video* 设备，带名称（v4l2-ctl --info 解析）
// Name 格式为 "路径 (驱动名)"，方便用户确认具体设备节点。
func ListVideoDevices() []Device {
	var devices []Device
	for i := 0; i < 16; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		dev := Device{Path: path, Exists: true, Name: path}
		if name := deviceName(path); name != "" {
			dev.Name = fmt.Sprintf("%s (%s)", path, name)
		}
		devices = append(devices, dev)
	}
	return devices
}

// deviceName 通过 v4l2-ctl --info 获取设备驱动/卡名（优先 Card type 的友好名）
func deviceName(path string) string {
	cmd := exec.Command("v4l2-ctl", "-d", path, "--info")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var card, driver string
	// 形如 "Driver name   : uvcvideo" / "Card type     : USB Camera"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Card type") {
			card = fieldValue(line)
		} else if strings.HasPrefix(line, "Driver name") {
			driver = fieldValue(line)
		}
	}
	if card != "" {
		return card
	}
	return driver
}

// fieldValue 提取 "Key : value" 中的值
func fieldValue(line string) string {
	if idx := strings.Index(line, ":"); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return ""
}

// ListAudioDevices 列出 ALSA 音频输入设备。
// 只保留真正用于采集的项（hw:/plughw:/pulse），过滤掉 sysdefault/front/dsnoop 等
// 重复别名，避免同一声卡出现一堆相同项。Path 天然带前缀（如 plughw:CARD=S100,DEV=0）
// 方便分辨，USB 设备优先排列。
func ListAudioDevices() []Device {
	all := parseALSAOutput(runArecordL())
	devices := make([]Device, 0, len(all))
	for _, d := range all {
		if keepCaptureDevice(d.Path) {
			devices = append(devices, d)
		}
	}
	if len(devices) == 0 {
		return []Device{{Path: "hw:0", Name: "默认 (hw:0)", Exists: true}}
	}

	// 排序：USB 设备在前，便于选择
	usbFirst := make([]Device, 0, len(devices))
	rest := make([]Device, 0, len(devices))
	for _, d := range devices {
		if d.USB {
			usbFirst = append(usbFirst, d)
		} else {
			rest = append(rest, d)
		}
	}
	return append(usbFirst, rest...)
}

// keepCaptureDevice 判断设备名是否为可采集的输入项。
// 保留 plughw:/hw:（CARD= 命名或 hw:N），以及 pulse；过滤 sysdefault/front/dsnoop/null/default 别名。
func keepCaptureDevice(path string) bool {
	if strings.HasPrefix(path, "plughw:") || strings.HasPrefix(path, "hw:") {
		return true
	}
	if strings.HasPrefix(path, "pulse") || strings.Contains(path, "@") {
		return true
	}
	return false
}

// FindUSBAudioDevice 返回第一个 USB 声卡的可采集设备名（如 "plughw:CARD=S100,DEV=0"）。
// 找不到时返回空字符串。USB 声卡即插即用，运行时探测保证插拔后仍可用。
func FindUSBAudioDevice() string {
	return findUSBDevice(parseALSAOutput(runArecordL()))
}

// findUSBDevice 从解析结果中挑选 USB 声卡设备
func findUSBDevice(devices []Device) string {
	// 优先 plughw（带软件转换，采样率/格式不匹配也能采），其次 hw / default
	for _, d := range devices {
		if d.USB && strings.HasPrefix(d.Path, "plughw:CARD=") {
			return d.Path
		}
	}
	for _, d := range devices {
		if d.USB && (strings.HasPrefix(d.Path, "hw:CARD=") || strings.HasPrefix(d.Path, "default:CARD=")) {
			return d.Path
		}
	}
	return ""
}

// runArecordL 执行 arecord -L，失败返回空串
func runArecordL() string {
	cmd := exec.Command("arecord", "-L")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// parseALSAOutput 解析 arecord -L 输出。
// 格式：顶层设备名行（无缩进），后续缩进行为设备描述。
// 例：
//
//	plughw:CARD=S100,DEV=0
//	    Synido Voice 100, USB Audio
//	    Hardware device with all software conversions
func parseALSAOutput(output string) []Device {
	var devices []Device
	idx := -1
	for _, line := range strings.Split(output, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// 描述行
			if idx >= 0 {
				if devices[idx].Name == devices[idx].Path {
					devices[idx].Name = t
				}
				if strings.Contains(strings.ToUpper(t), "USB") {
					devices[idx].USB = true
				}
			}
			continue
		}
		// 设备名行
		if strings.ContainsAny(t, " \t") {
			continue // 过滤多余内容
		}
		switch t {
		case "null", "":
			continue
		}
		devices = append(devices, Device{Path: t, Name: t, Exists: true})
		idx = len(devices) - 1
	}
	return devices
}
