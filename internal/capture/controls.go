package capture

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DeviceControl v4l2 设备控制参数
type DeviceControl struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
	Step  int    `json:"step"`
}

// GetControls 获取设备支持的所有控制参数
func GetControls(devicePath string) ([]DeviceControl, error) {
	cmd := exec.Command("v4l2-ctl", "-d", devicePath, "-L")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("v4l2-ctl -L 失败: %w", err)
	}
	return parseControls(string(out)), nil
}

// SetControl 设置设备控制参数
func SetControl(devicePath, name string, value int) error {
	cmd := exec.Command("v4l2-ctl", "-d", devicePath, "-c", fmt.Sprintf("%s=%d", name, value))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 %s=%d 失败: %s", name, value, string(out))
	}
	return nil
}

// parseControls 解析 v4l2-ctl -L 输出
func parseControls(output string) []DeviceControl {
	var controls []DeviceControl
	re := regexp.MustCompile(`(\w+)\s+0x[0-9a-f]+\s+\((\w+)\)\s*:\s*min=(-?\d+)\s+max=(-?\d+)\s+step=(-?\d+)\s+default=(-?\d+)\s+value=(-?\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		min, _ := strconv.Atoi(m[3])
		max, _ := strconv.Atoi(m[4])
		step, _ := strconv.Atoi(m[5])
		val, _ := strconv.Atoi(m[7])
		controls = append(controls, DeviceControl{
			Name:  m[1],
			Value: val,
			Min:   min,
			Max:   max,
			Step:  step,
		})
	}
	// 如果上面正则没匹配到，尝试更简单的格式
	if len(controls) == 0 {
		controls = parseControlsSimple(output)
	}
	return controls
}

func parseControlsSimple(output string) []DeviceControl {
	var controls []DeviceControl
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") || strings.HasPrefix(line, "User") || strings.HasPrefix(line, "Camera") {
			continue
		}
		// 格式: name (type) : min=X max=Y step=Z default=W value=V
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(parts[0], "(", 2)[0])
		info := parts[1]
		dc := DeviceControl{Name: name}
		for _, kv := range strings.Split(info, " ") {
			kv = strings.TrimSpace(kv)
			if strings.HasPrefix(kv, "min=") {
				dc.Min, _ = strconv.Atoi(strings.TrimPrefix(kv, "min="))
			} else if strings.HasPrefix(kv, "max=") {
				dc.Max, _ = strconv.Atoi(strings.TrimPrefix(kv, "max="))
			} else if strings.HasPrefix(kv, "step=") {
				dc.Step, _ = strconv.Atoi(strings.TrimPrefix(kv, "step="))
			} else if strings.HasPrefix(kv, "value=") {
				dc.Value, _ = strconv.Atoi(strings.TrimPrefix(kv, "value="))
			}
		}
		if dc.Name != "" {
			controls = append(controls, dc)
		}
	}
	return controls
}
