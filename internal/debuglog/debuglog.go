package debuglog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	logFile *os.File
)

func logPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "debug.log"
	}
	return filepath.Join(filepath.Dir(exe), "debug.log")
}

func init() {
	path := logPath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logFile = f
	Log("=== SCTroll gestartet ===")
}

func Log(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	if logFile == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logFile, "[%s] %s\n", ts, msg)
	logFile.Sync()
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
	}
}

func GetLogPath() string {
	return logPath()
}
