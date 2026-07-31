// Package logging 提供文件日志输出（logs/ 目录）。
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileLog 带时间戳的文件日志写入器
type FileLog struct {
	mu   sync.Mutex
	file *os.File
	dir  string
}

// Open 在 dir 目录下创建新的日志文件（文件名带时间戳）
func Open(dir string) (*FileLog, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}
	name := fmt.Sprintf("headlesslive-%s.log", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}
	fmt.Printf("[logging] 日志文件: %s\n", path)
	return &FileLog{file: f, dir: dir}, nil
}

// WriteLine 写一行日志（带时间戳）
func (l *FileLog) WriteLine(line string) {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.file, "[%s] %s\n", time.Now().Format("15:04:05"), line)
}

// Close 关闭日志文件
func (l *FileLog) Close() {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.file.Close()
	l.file = nil
}
