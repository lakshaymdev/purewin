package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultMaxLogSize is the maximum log size before rotation (10 MB).
	DefaultMaxLogSize = 10 * 1024 * 1024

	// logTimeFormat is the timestamp format used in log entries.
	logTimeFormat = "2006-01-02 15:04:05"

	// envNoOpLog is the environment variable to disable operation logging.
	envNoOpLog = "WM_NO_OPLOG"
)

// Logger writes structured operation logs to a file.
type Logger struct {
	file    *os.File
	path    string
	mu      sync.Mutex
	enabled bool
}

// NewLogger creates a new Logger that writes to the given path.
// If the WM_NO_OPLOG=1 environment variable is set, logging is disabled
// and all operations become no-ops.
func NewLogger(logPath string) (*Logger, error) {
	l := &Logger{
		path:    logPath,
		enabled: os.Getenv(envNoOpLog) != "1",
	}

	if !l.enabled {
		return l, nil
	}

	// Ensure the directory exists.
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create log directory %s: %w", dir, err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open log file %s: %w", logPath, err)
	}
	l.file = file

	return l, nil
}

// Log writes a single operation entry to the log file.
func (l *Logger) Log(operation, path string, size int64, err error) {
	if !l.enabled || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	status := "OK"
	detail := ""
	if err != nil {
		status = "ERROR"
		detail = fmt.Sprintf(" error=%q", err.Error())
	}

	line := fmt.Sprintf("[%s] %s %s path=%q size=%s%s\n",
		time.Now().Format(logTimeFormat),
		status,
		operation,
		path,
		FormatSize(size),
		detail,
	)
	_, _ = l.file.WriteString(line)
}

// LogSession writes a session start marker to the log file.
func (l *Logger) LogSession(command string) {
	if !l.enabled || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	line := fmt.Sprintf("\n═══ [%s] SESSION START: pw %s ═══\n",
		time.Now().Format(logTimeFormat),
		command,
	)
	_, _ = l.file.WriteString(line)
}

// LogSummary writes a session end summary to the log file.
func (l *Logger) LogSummary(freed int64, files int, errCount int) {
	if !l.enabled || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	line := fmt.Sprintf("═══ [%s] SESSION END: freed=%s files=%d errors=%d ═══\n\n",
		time.Now().Format(logTimeFormat),
		FormatSize(freed),
		files,
		errCount,
	)
	_, _ = l.file.WriteString(line)
}

// Close flushes and closes the log file.
func (l *Logger) Close() {
	if l.file != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		_ = l.file.Sync()
		_ = l.file.Close()
		l.file = nil
	}
}

// RotateIfNeeded rotates the log file if it exceeds maxSize bytes.
// The current log is renamed to operations.log.1 and a new file is opened.
func (l *Logger) RotateIfNeeded(maxSize int64) {
	if !l.enabled || l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	info, err := l.file.Stat()
	if err != nil || info.Size() < maxSize {
		return
	}

	// Close current file.
	_ = l.file.Sync()
	_ = l.file.Close()

	// Rotate: remove old backup, rename current to .1
	backupPath := l.path + ".1"
	_ = os.Remove(backupPath)
	_ = os.Rename(l.path, backupPath)

	// Open a new log file.
	file, openErr := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		l.file = nil
		l.enabled = false
		return
	}
	l.file = file
}
