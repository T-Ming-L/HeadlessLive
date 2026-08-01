package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelWarn:
		return "WARN "
	case LevelError:
		return "ERROR"
	}
	return "?????"
}

// Logger 线程安全的日志器：stdout + 可选文件，带时间戳和级别
type Logger struct {
	mu    sync.Mutex
	file  *os.File
	level LogLevel
}

var log = &Logger{level: LevelInfo}

// InitLogger 初始化日志器（写文件可选）
func InitLogger(path string, verbose bool) error {
	log.mu.Lock()
	defer log.mu.Unlock()
	if verbose {
		log.level = LevelDebug
	}
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("打开日志文件失败: %w", err)
		}
		log.file = f
	}
	return nil
}

func (l *Logger) logf(lv LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lv < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] [%s] %s\n", ts, lv.String(), fmt.Sprintf(format, args...))
	fmt.Print(line)
	if l.file != nil {
		l.file.WriteString(line)
	}
}

// Debugf 调试日志
func Debugf(format string, args ...interface{}) { log.logf(LevelDebug, format, args...) }

// Infof 信息日志
func Infof(format string, args ...interface{}) { log.logf(LevelInfo, format, args...) }

// Warnf 警告日志
func Warnf(format string, args ...interface{}) { log.logf(LevelWarn, format, args...) }

// Errorf 错误日志
func Errorf(format string, args ...interface{}) { log.logf(LevelError, format, args...) }

// Close 关闭日志文件
func Close() {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.file != nil {
		log.file.Close()
		log.file = nil
	}
}
