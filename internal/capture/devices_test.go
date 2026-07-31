package capture

import (
	"strings"
	"testing"
)

// arecord -L 实际输出样例：Synido Voice 100（USB 声卡）+ 主板声卡
const synidoOutput = `null
    Discard all samples (playback) or generate zero samples (capture)
default
    Default Audio Device
default:CARD=PCH
    HDA Intel PCH, ALC897 Analog
    Default Audio Device
plughw:CARD=PCH,DEV=0
    HDA Intel PCH, ALC897 Analog
    Hardware device with all software conversions
hw:CARD=S100,DEV=0
    Synido Voice 100, USB Audio
    Direct hardware device without any conversions
plughw:CARD=S100,DEV=0
    Synido Voice 100, USB Audio
    Hardware device with all software conversions
default:CARD=S100
    Synido Voice 100, USB Audio
    Default Audio Device
sysdefault:CARD=S100
    Synido Voice 100, USB Audio
    Default Audio Device
front:CARD=S100,DEV=0
    Synido Voice 100, USB Audio
    Front output / input
dsnoop:CARD=S100,DEV=0
    Synido Voice 100, USB Audio
    Direct sample snooping device
`

func TestParseALSAOutput(t *testing.T) {
	devices := parseALSAOutput(synidoOutput)
	if len(devices) == 0 {
		t.Fatal("未解析出任何设备")
	}

	// 校验 USB 声卡设备被标记
	foundUSB := 0
	for _, d := range devices {
		if d.Path == "plughw:CARD=S100,DEV=0" {
			if !d.USB {
				t.Error("Synido Voice 100 应被标记为 USB 设备")
			}
			if d.Name != "Synido Voice 100, USB Audio" {
				t.Errorf("设备名解析错误: %q", d.Name)
			}
			foundUSB++
		}
	}
	if foundUSB == 0 {
		t.Error("未找到 plughw:CARD=S100 设备")
	}

	// 主板声卡不应标记 USB
	for _, d := range devices {
		if d.Path == "plughw:CARD=PCH,DEV=0" && d.USB {
			t.Error("主板声卡不应标记为 USB")
		}
	}
}

func TestFindUSBDevice(t *testing.T) {
	devices := parseALSAOutput(synidoOutput)

	got := findUSBDevice(devices)
	if got != "plughw:CARD=S100,DEV=0" {
		t.Errorf("期望 plughw:CARD=S100,DEV=0，实际 %q", got)
	}
}

func TestFindUSBDeviceNone(t *testing.T) {
	// 只有主板声卡，无 USB 设备
	output := `default:CARD=PCH
    HDA Intel PCH, ALC897 Analog
    Default Audio Device
`
	devices := parseALSAOutput(output)
	if got := findUSBDevice(devices); got != "" {
		t.Errorf("无 USB 声卡时应返回空串，实际 %q", got)
	}
}

func TestKeepCaptureDevice(t *testing.T) {
	// 保留：plughw/hw/pulse 输入项
	for _, keep := range []string{
		"plughw:CARD=S100,DEV=0",
		"hw:CARD=S100,DEV=0",
		"hw:1",
		"pulse",
	} {
		if !keepCaptureDevice(keep) {
			t.Errorf("应保留 %q", keep)
		}
	}
	// 过滤：重复别名
	for _, drop := range []string{
		"default",
		"default:CARD=S100",
		"sysdefault:CARD=S100",
		"front:CARD=S100,DEV=0",
		"dsnoop:CARD=S100,DEV=0",
		"null",
	} {
		if keepCaptureDevice(drop) {
			t.Errorf("应过滤 %q", drop)
		}
	}
}

func TestListAudioDevicesDedup(t *testing.T) {
	devices := parseALSAOutput(synidoOutput)
	var kept []string
	for _, d := range devices {
		if keepCaptureDevice(d.Path) {
			kept = append(kept, d.Path)
		}
	}
	// USB 声卡只保留 plughw + hw，无 default/sysdefault/front/dsnoop
	for _, want := range []string{"plughw:CARD=S100,DEV=0", "hw:CARD=S100,DEV=0"} {
		found := false
		for _, p := range kept {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("缺少 %q，实际 %v", want, kept)
		}
	}
	for _, drop := range []string{"sysdefault", "front:", "dsnoop:"} {
		for _, p := range kept {
			if strings.Contains(p, drop) {
				t.Errorf("不应包含 %q（实际 %v）", drop, kept)
			}
		}
	}
}
