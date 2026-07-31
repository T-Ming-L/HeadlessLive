// Package config 服务器配置：端口 + 调试/日志开关。
// 场景/源/输出配置在 scenes.yaml，本文件只管理服务自身行为。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	Port  int  `yaml:"port"`  // HTTP 端口
	Debug bool `yaml:"debug"` // 调试模式：true 输出详细 FFmpeg 日志
	Log   bool `yaml:"log"`   // 将日志写入 logs/ 目录文件
}

// Config 顶层配置
type Config struct {
	Server ServerConfig `yaml:"server"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{Port: 8080, Debug: false, Log: true},
	}
}

// Load 加载配置；文件不存在则生成默认示例
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[config] %s 不存在，自动生成示例配置\n", path)
			gen := DefaultConfig()
			if err := generateSample(path, gen); err != nil {
				return nil, fmt.Errorf("写入默认配置失败: %w", err)
			}
			return gen, nil
		}
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	return cfg, nil
}

func generateSample(path string, cfg *Config) error {
	sample := `# HeadlessLive 服务器配置
# 场景/源/输出配置在 scenes.yaml，本文件只控制服务自身行为
server:
  port: 8080        # HTTP 端口
  debug: false      # 调试模式：true 输出详细 FFmpeg 日志
  log: true         # 将日志写入 logs/ 目录下的文件
`
	return os.WriteFile(path, []byte(sample), 0644)
}
