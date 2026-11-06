package main

import (
	"os"
	"strings"
	"sync"
	"time"
)

// The log is truncated once it passes this size; the app runs for weeks in
// the tray and nobody prunes the file manually.
const logMaxBytes = 1 << 20

type logger struct {
	mu   sync.Mutex
	file *os.File
	size int64
}

func newLogger(filePath string) *logger {
	l := &logger{}
	if filePath == "" {
		return l
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		if info, statErr := f.Stat(); statErr == nil && info.Size() >= logMaxBytes {
			f.Close()
			f, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		} else if statErr == nil {
			l.size = info.Size()
		}
	}
	if err == nil {
		l.file = f
	}
	return l
}

func (l *logger) log(section, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}

	prefix := ""
	if section != "" {
		prefix = "[" + section + "] "
	}
	line := strings.TrimSpace(time.Now().Format("2006-01-02 15:04:05.000")+" "+prefix+message) + "\r\n"
	if _, err := l.file.WriteString(line); err != nil {
		l.closeLocked()
		return
	}

	l.size += int64(len(line))
	if l.size >= logMaxBytes {
		l.rotateLocked()
	}
}

// rotateLocked truncates the file in place rather than moving it aside: the
// file lives next to the core where a second copy would only confuse users.
func (l *logger) rotateLocked() {
	path := l.file.Name()
	l.closeLocked()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	l.file = f
	l.size = 0
}

func (l *logger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closeLocked()
}

func (l *logger) closeLocked() {
	if l.file == nil {
		return
	}
	_ = l.file.Close()
	l.file = nil
}
