// Package store 负责配置（场景/源/输出）的 YAML 持久化与 CRUD。
package store

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/T-Ming-L/HeadlessLive/internal/model"
	"gopkg.in/yaml.v3"
)

// Store 配置存储，带读写锁
type Store struct {
	mu   sync.RWMutex
	path string
	data *model.Store
}

// New 创建内存中的空存储（未绑定文件）
func New() *Store {
	return &Store{
		data: &model.Store{
			Sources:   []*model.Source{},
			Scenes:    []*model.Scene{},
			Outputs:   []*model.Output{},
			UpdatedAt: time.Now(),
		},
	}
}

// Load 从 YAML 文件加载配置；文件不存在则生成默认示例
func Load(path string) (*Store, error) {
	s := New()
	s.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[store] %s 不存在，生成默认示例配置\n", path)
			s.data = DefaultData()
			if err := s.Save(); err != nil {
				return nil, fmt.Errorf("写入默认配置失败: %w", err)
			}
			return s, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	if err := yaml.Unmarshal(data, s.data); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	// 保证切片非 nil，方便 JSON 序列化
	if s.data.Sources == nil {
		s.data.Sources = []*model.Source{}
	}
	if s.data.Scenes == nil {
		s.data.Scenes = []*model.Scene{}
	}
	if s.data.Outputs == nil {
		s.data.Outputs = []*model.Output{}
	}
	fmt.Printf("[store] 已加载 %d 源 / %d 场景 / %d 输出\n",
		len(s.data.Sources), len(s.data.Scenes), len(s.data.Outputs))
	return s, nil
}

// Save 保存到磁盘
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	s.data.UpdatedAt = time.Now()
	data, err := yaml.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

// Data 返回数据快照（浅拷贝，调用方不应修改内部切片元素）
func (s *Store) Data() *model.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Path 返回配置文件路径
func (s *Store) Path() string {
	return s.path
}

// --- 源 CRUD ---

func (s *Store) AddSource(src *model.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.FindSource(src.ID) != nil {
		return fmt.Errorf("源 %s 已存在", src.ID)
	}
	s.data.Sources = append(s.data.Sources, src)
	return s.saveLocked()
}

func (s *Store) UpdateSource(src *model.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Sources {
		if existing.ID == src.ID {
			s.data.Sources[i] = src
			return s.saveLocked()
		}
	}
	return fmt.Errorf("源 %s 不存在", src.ID)
}

func (s *Store) DeleteSource(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, src := range s.data.Sources {
		if src.ID == id {
			s.data.Sources = append(s.data.Sources[:i], s.data.Sources[i+1:]...)
			// 同时从所有场景中移除引用
			for _, sc := range s.data.Scenes {
				items := sc.Items[:0]
				for _, it := range sc.Items {
					if it.SourceID != id {
						items = append(items, it)
					}
				}
				sc.Items = items
			}
			return s.saveLocked()
		}
	}
	return fmt.Errorf("源 %s 不存在", id)
}

// --- 场景 CRUD ---

func (s *Store) AddScene(sc *model.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.FindScene(sc.ID) != nil {
		return fmt.Errorf("场景 %s 已存在", sc.ID)
	}
	s.data.Scenes = append(s.data.Scenes, sc)
	if s.data.CurrentScene == "" {
		s.data.CurrentScene = sc.ID
	}
	return s.saveLocked()
}

func (s *Store) UpdateScene(sc *model.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Scenes {
		if existing.ID == sc.ID {
			s.data.Scenes[i] = sc
			return s.saveLocked()
		}
	}
	return fmt.Errorf("场景 %s 不存在", sc.ID)
}

func (s *Store) DeleteScene(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sc := range s.data.Scenes {
		if sc.ID == id {
			s.data.Scenes = append(s.data.Scenes[:i], s.data.Scenes[i+1:]...)
			if s.data.CurrentScene == id {
				s.data.CurrentScene = ""
				if len(s.data.Scenes) > 0 {
					s.data.CurrentScene = s.data.Scenes[0].ID
				}
			}
			return s.saveLocked()
		}
	}
	return fmt.Errorf("场景 %s 不存在", id)
}

// SetCurrentScene 设置当前活动场景
func (s *Store) SetCurrentScene(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.FindScene(id) == nil {
		return fmt.Errorf("场景 %s 不存在", id)
	}
	s.data.CurrentScene = id
	return s.saveLocked()
}

// --- 输出 CRUD ---

func (s *Store) AddOutput(o *model.Output) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.FindOutput(o.ID) != nil {
		return fmt.Errorf("输出 %s 已存在", o.ID)
	}
	s.data.Outputs = append(s.data.Outputs, o)
	return s.saveLocked()
}

func (s *Store) UpdateOutput(o *model.Output) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Outputs {
		if existing.ID == o.ID {
			s.data.Outputs[i] = o
			return s.saveLocked()
		}
	}
	return fmt.Errorf("输出 %s 不存在", o.ID)
}

func (s *Store) DeleteOutput(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, o := range s.data.Outputs {
		if o.ID == id {
			s.data.Outputs = append(s.data.Outputs[:i], s.data.Outputs[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("输出 %s 不存在", id)
}

// DefaultData 生成示例配置：一个场景、采集卡 + 图片 + 浏览器三个源、一个 RTMP 输出
func DefaultData() *model.Store {
	now := time.Now()
	return &model.Store{
		Sources: []*model.Source{
			{
				ID: "src-camera", Name: "采集卡", Type: model.SourceVideoDevice,
				Enabled: true, DevicePath: "/dev/video0",
				PixelFormat: "yuyv422", Resolution: "1920x1080", FPS: 30, ColorSpace: "bt709",
			},
			{
				ID: "src-logo", Name: "Logo", Type: model.SourceImage,
				Enabled: true, FilePath: "uploads/logo.png", Loop: true,
			},
			{
				ID: "src-browser", Name: "浏览器面板", Type: model.SourceBrowser,
				Enabled: false, URL: "https://example.com",
				BrowserW: 1280, BrowserH: 720, BrowserFPS: 30,
			},
			{
				ID: "src-mic", Name: "USB 声卡", Type: model.SourceAudioDevice,
				Enabled: true,
				// "usb" = 自动探测 USB 声卡（即插即用，如 Synido Voice 100），
				// 也可手动指定设备名，如 "plughw:CARD=S100,DEV=0"
				AudioDevice: "usb", SampleRate: 48000, Channels: 2, Volume: 1.0,
			},
		},
		Scenes: []*model.Scene{
			{
				ID: "scene-main", Name: "主场景",
				CanvasW: 1920, CanvasH: 1080, FPS: 60,
				Items: []model.SceneItem{
					{SourceID: "src-camera", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1.0, ZIndex: 0, Visible: true},
					{SourceID: "src-logo", X: 20, Y: 20, Width: 240, Height: 135, Opacity: 1.0, ZIndex: 1, Visible: true},
					{SourceID: "src-browser", X: 0, Y: 0, Width: 1920, Height: 1080, Opacity: 1.0, ZIndex: 2, Visible: false},
				},
			},
		},
		Outputs: []*model.Output{
			{
				ID: "out-rtmp", Name: "哔哩哔哩", Type: model.OutputRTMP,
				SceneID: "scene-main", Enabled: false,
				// B 站推流地址；密钥（?streamname=...）在网页端填入
				URL: "rtmp://live-push.bilivideo.com/live-bvc/", StreamKey: "",
				Encoder: "h264_vaapi", Bitrate: 6000, KeyFrame: 120,
			},
		},
		CurrentScene: "scene-main",
		UpdatedAt:    now,
	}
}
