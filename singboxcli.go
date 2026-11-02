package main

import (
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	cliVersionTimeout   = 5 * time.Second
	cliTerminateWait    = time.Second
	cliMaxOutputBytes   = 8192
	cliMaxVersionLength = 64
	cliVersionPrefix    = "sing-box version "
)

type singBoxCLI struct {
	corePath string
	logger   *logger

	mu      sync.Mutex
	version string
	checked bool
}

func newSingBoxCLI(corePath string, l *logger) *singBoxCLI {
	return &singBoxCLI{corePath: corePath, logger: l}
}

func (c *singBoxCLI) getVersion() string {
	justFetched := false

	c.mu.Lock()
	if !c.checked {
		c.version = c.fetchVersion()
		c.checked = true
		justFetched = true
	}
	version := c.version
	c.mu.Unlock()

	if justFetched {
		if version != "" {
			c.log("Detected version: " + version)
		} else {
			c.log("Failed to detect version.")
		}
	}
	return version
}

func (c *singBoxCLI) fetchVersion() string {
	output, ok := c.runCLI("version", cliVersionTimeout, cliMaxOutputBytes)
	if !ok {
		return ""
	}

	firstLine := output
	if pos := strings.IndexByte(firstLine, '\n'); pos >= 0 {
		firstLine = firstLine[:pos]
	}
	firstLine = strings.TrimSpace(firstLine)

	if !strings.HasPrefix(firstLine, cliVersionPrefix) {
		return ""
	}

	version := strings.TrimSpace(firstLine[len(cliVersionPrefix):])
	if len(version) >= cliMaxVersionLength {
		return ""
	}
	return version
}

func (c *singBoxCLI) runCLI(args string, timeout time.Duration, maxOutput int) (string, bool) {
	var argv []string
	if args != "" {
		argv = strings.Fields(args)
	}
	cmd := exec.Command(c.corePath, argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}

	out := &limitedBuffer{max: maxOutput}
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Start(); err != nil {
		return "", false
	}

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(cliTerminateWait):
		}
		return "", false
	}

	return out.text(), true
}

func (c *singBoxCLI) log(message string) {
	c.logger.log("SingBoxCli", message)
}

type limitedBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remain := b.max - len(b.buf)
	if remain > 0 {
		if len(p) < remain {
			remain = len(p)
		}
		b.buf = append(b.buf, p[:remain]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) text() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}
