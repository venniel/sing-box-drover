package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	maxCaptureBytes       = 8 * 1024
	readerDrainTimeout    = 5 * time.Second
	postKillWait          = time.Second
	gracefulStopTimeout   = 10 * time.Second
	apiCheckDelay         = 500 * time.Millisecond
	apiCheckInterval      = 500 * time.Millisecond
	apiCheckTimeout       = 60 * time.Second
	connectionsFlushLimit = 2 * time.Second
)

type coreState int32

const (
	csStopped coreState = iota
	csStarting
	csRunning
	csStopping
	csFailed
)

var coreStateNames = [...]string{"Stopped", "Starting", "Running", "Stopping", "Failed"}

type coreEventKind int

const (
	cekState coreEventKind = iota
	cekError
	cekAPIReady
	cekConfigModes
	cekSelectorDone
	cekModeDone
)

type coreEvent struct {
	Kind      coreEventKind
	State     coreState
	Msg       string
	RequestID int
	Mode      string
	Modes     []string
}

type selectorTask struct {
	Name  string
	Value string
}

type coreCommandKind int

const (
	cmdNone coreCommandKind = iota
	cmdStart
	cmdStop
	cmdSetSelectors
	cmdSetMode
)

var coreCommandNames = [...]string{"None", "Start", "Stop", "SetSelectors", "SetMode"}

type coreCommand struct {
	Kind          coreCommandKind
	ConfigJSON    string
	SelectorTasks []selectorTask
	RequestID     int
	Mode          string
}

type coreSupervisor struct {
	exePath string
	logger  *logger
	api     *coreAPIClient
	onEvent func(coreEvent)

	commands chan coreCommand
	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	state int32

	job       windows.Handle
	cmd       *exec.Cmd
	processID uint32
	stdinW    *os.File
	reader    *outputReader
	waitCh    chan struct{}
	lastOut   string

	apiCheckActive bool
	apiNextCheck   time.Time
	apiDeadline    time.Time
}

func newCoreSupervisor(exePath string, l *logger, cfg clashAPIConfig, onEvent func(coreEvent)) *coreSupervisor {
	s := &coreSupervisor{
		exePath:  exePath,
		logger:   l,
		api:      newCoreAPIClient(cfg, l),
		onEvent:  onEvent,
		commands: make(chan coreCommand, 100),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	atomic.StoreInt32(&s.state, int32(csStopped))
	go s.run()
	return s
}

func (s *coreSupervisor) currentState() coreState {
	return coreState(atomic.LoadInt32(&s.state))
}

func (s *coreSupervisor) run() {
	defer close(s.done)
	s.log("Supervisor started.")

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		s.checkProcessStatus()
		s.checkAPIStatus()

		select {
		case <-s.stopCh:
			s.log("Supervisor stopping...")
			s.stopGraceful()
			s.log("Supervisor stopped.")
			return
		case cmd := <-s.commands:
			s.handleCommand(cmd)
		case <-ticker.C:
		}
	}
}

func (s *coreSupervisor) requestStart(configJSON string) {
	s.post(coreCommand{Kind: cmdStart, ConfigJSON: configJSON})
}

func (s *coreSupervisor) requestStop() {
	s.post(coreCommand{Kind: cmdStop})
}

func (s *coreSupervisor) requestSetSelectors(tasks []selectorTask, requestID int) bool {
	return s.post(coreCommand{Kind: cmdSetSelectors, SelectorTasks: tasks, RequestID: requestID})
}

func (s *coreSupervisor) requestSetMode(mode string) bool {
	return s.post(coreCommand{Kind: cmdSetMode, Mode: mode})
}

func (s *coreSupervisor) post(cmd coreCommand) bool {
	select {
	case s.commands <- cmd:
		return true
	case <-s.stopCh:
		return false
	}
}

func (s *coreSupervisor) stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	<-s.done
}

func (s *coreSupervisor) handleCommand(cmd coreCommand) {
	s.log(fmt.Sprintf("Command: %s.", coreCommandNames[cmd.Kind]))

	switch cmd.Kind {
	case cmdStart:
		s.doStart(cmd.ConfigJSON)
	case cmdStop:
		s.stopGraceful()
	case cmdSetSelectors:
		s.doSetSelectors(cmd.SelectorTasks, cmd.RequestID)
	case cmdSetMode:
		s.doSetMode(cmd.Mode)
	}
}

func (s *coreSupervisor) doStart(configJSON string) {
	s.stopGraceful()
	s.lastOut = ""
	s.setStateAndNotify(csStarting, "")

	if err := s.startProcess(configJSON); err != nil {
		s.cleanupProcess()
		s.setStateAndNotify(csFailed, s.buildFailureMessage(err.Error(), s.lastOut))
		return
	}

	s.log(fmt.Sprintf("Process created, PID: %d.", s.processID))
	s.setStateAndNotify(csRunning, "")
	s.initAPICheck()
}

func (s *coreSupervisor) startProcess(configJSON string) error {
	if !fileExists(s.exePath) {
		return errors.New("sing-box executable not found.")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("CreateJobObject failed: %v", err)
	}
	closeJob := true
	defer func() {
		if closeJob {
			_ = windows.CloseHandle(job)
		}
	}()

	var jobInfo windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	jobInfo.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&jobInfo)), uint32(unsafe.Sizeof(jobInfo))); err != nil {
		return fmt.Errorf("SetInformationJobObject failed: %v", err)
	}

	inR, inW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("CreatePipe (stdin) failed: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return fmt.Errorf("CreatePipe (output) failed: %v", err)
	}

	cmd := exec.Command(s.exePath, "--disable-color", "run", "-c", "stdin")
	cmd.Dir = filepath.Dir(s.exePath)
	cmd.Stdin = inR
	cmd.Stdout = outW
	cmd.Stderr = outW
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_CONSOLE | windows.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		_ = inR.Close()
		_ = inW.Close()
		_ = outR.Close()
		_ = outW.Close()
		return fmt.Errorf("CreateProcess failed: %v", err)
	}

	_ = inR.Close()
	_ = outW.Close()

	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = inW.Close()
		_ = cmd.Process.Kill()
		return fmt.Errorf("OpenProcess failed: %v", err)
	}
	assignErr := windows.AssignProcessToJobObject(job, processHandle)
	_ = windows.CloseHandle(processHandle)
	if assignErr != nil {
		_ = inW.Close()
		_ = cmd.Process.Kill()
		return fmt.Errorf("AssignProcessToJobObject failed: %v", assignErr)
	}

	closeJob = false
	s.job = job
	s.cmd = cmd
	s.processID = uint32(cmd.Process.Pid)
	s.stdinW = inW
	s.waitCh = make(chan struct{}, 1)
	go func(c *exec.Cmd, ch chan struct{}) {
		_ = c.Wait()
		ch <- struct{}{}
	}(cmd, s.waitCh)
	s.reader = startOutputReader(outR)

	if _, err := inW.Write([]byte(configJSON)); err != nil {
		return fmt.Errorf("WriteFile (stdin) failed: %v", err)
	}
	_ = inW.Close()
	s.stdinW = nil

	return nil
}

func (s *coreSupervisor) sendGracefulSignal(processID uint32) bool {
	pFreeConsole.Call()
	if r, _, _ := pAttachConsole.Call(uintptr(processID)); r == 0 {
		return false
	}
	defer pFreeConsole.Call()

	pSetConsoleCtrlHandler.Call(0, 1)
	sent := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, processID) == nil
	time.Sleep(10 * time.Millisecond)
	pSetConsoleCtrlHandler.Call(0, 0)
	return sent
}

func (s *coreSupervisor) stopGraceful() {
	if s.cmd == nil {
		return
	}
	s.setStateAndNotify(csStopping, "")

	kill := true
	if s.sendGracefulSignal(s.processID) {
		s.log("Graceful shutdown signal sent.")
		select {
		case <-s.waitCh:
			kill = false
			s.log("Graceful shutdown complete.")
		case <-time.After(gracefulStopTimeout):
		}
	}

	if kill {
		s.log("Graceful shutdown timeout, forcing termination.")
		_ = s.cmd.Process.Kill()
		select {
		case <-s.waitCh:
		case <-time.After(postKillWait):
		}
	}

	s.cleanupProcess()
	s.setStateAndNotify(csStopped, "")
}

func (s *coreSupervisor) checkProcessStatus() {
	if s.cmd == nil {
		return
	}

	select {
	case <-s.waitCh:
		s.cleanupProcess()
		s.setStateAndNotify(csFailed, s.buildFailureMessage("sing-box process exited unexpectedly.", s.lastOut))
	default:
	}
}

func (s *coreSupervisor) cleanupProcess() {
	if s.job != 0 {
		_ = windows.CloseHandle(s.job)
		s.job = 0
	}

	s.processID = 0
	s.cmd = nil

	if s.stdinW != nil {
		_ = s.stdinW.Close()
		s.stdinW = nil
	}

	if s.reader != nil {
		output, drained := s.reader.drain(readerDrainTimeout)
		s.lastOut = output
		if !drained {
			s.log("Output reader still blocked; leaking it to keep supervisor responsive.")
		}
		s.reader = nil
	}
}

func (s *coreSupervisor) initAPICheck() {
	if !s.api.isConfigured() {
		s.apiCheckActive = false
		return
	}

	now := time.Now()
	s.apiCheckActive = true
	s.apiNextCheck = now.Add(apiCheckDelay)
	s.apiDeadline = now.Add(apiCheckTimeout)
}

func (s *coreSupervisor) checkAPIStatus() {
	if !s.apiCheckActive {
		return
	}
	if s.cmd == nil {
		s.apiCheckActive = false
		return
	}

	now := time.Now()
	if now.Before(s.apiNextCheck) {
		return
	}
	if !now.Before(s.apiDeadline) {
		s.log("API check timeout, stopping checks.")
		s.apiCheckActive = false
		return
	}
	s.apiNextCheck = now.Add(apiCheckInterval)

	if s.api.checkReady() {
		s.log("API ready.")
		s.apiCheckActive = false
		s.notifyEvent(coreEvent{Kind: cekAPIReady})
		s.doQueryConfig()
	}
}

func (s *coreSupervisor) doQueryConfig() {
	text, err := s.api.request("GET", "/configs", "", time.Second)
	if err != nil {
		s.log(strings.TrimSpace("Config query failed. " + err.Error()))
		return
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err != nil || obj == nil {
		return
	}

	event := coreEvent{Kind: cekConfigModes}
	if mode, ok := obj["mode"].(string); ok {
		event.Mode = mode
	}
	if list, ok := obj["mode-list"].([]any); ok {
		modes := make([]string, 0, len(list))
		for _, item := range list {
			if mode, ok := item.(string); ok {
				modes = append(modes, mode)
			}
		}
		event.Modes = modes
	}

	s.log(strings.TrimSpace(fmt.Sprintf("Clash mode: %s, available modes: %d.", event.Mode, len(event.Modes))))
	s.notifyEvent(event)
}

func (s *coreSupervisor) doSetMode(mode string) {
	body, _ := json.Marshal(map[string]string{"mode": mode})
	if _, err := s.api.request("PATCH", "/configs", string(body), time.Second); err != nil {
		s.notifyEvent(coreEvent{Kind: cekError, Msg: "Failed to switch mode. " + err.Error()})
		return
	}
	s.notifyEvent(coreEvent{Kind: cekModeDone, Mode: mode})
}

func (s *coreSupervisor) doSetSelectors(tasks []selectorTask, requestID int) {
	hasError := false
	for _, task := range tasks {
		body, _ := json.Marshal(map[string]string{"name": task.Value})
		if _, err := s.api.request("PUT", "/proxies/"+task.Name, string(body), time.Second); err != nil {
			hasError = true
			s.log(strings.TrimSpace(fmt.Sprintf("Selector update failed for %q. %s", task.Name, err.Error())))
		}
	}

	if hasError {
		s.notifyEvent(coreEvent{Kind: cekError, Msg: "Selector update failed."})
	}

	s.notifyEvent(coreEvent{Kind: cekSelectorDone, RequestID: requestID})

	if _, err := s.api.request("DELETE", "/connections", "", connectionsFlushLimit); err != nil {
		s.log(strings.TrimSpace("Connections flush failed. " + err.Error()))
	}
}

func (s *coreSupervisor) setStateAndNotify(state coreState, msg string) {
	s.log(strings.TrimSpace(fmt.Sprintf("State: %s. %s", coreStateNames[state], msg)))
	atomic.StoreInt32(&s.state, int32(state))
	s.notifyEvent(coreEvent{Kind: cekState, State: state, Msg: msg})
}

func (s *coreSupervisor) notifyEvent(event coreEvent) {
	if s.onEvent != nil {
		s.onEvent(event)
	}
}

func (s *coreSupervisor) buildFailureMessage(base, captured string) string {
	if captured == "" {
		return base
	}
	if critical := extractCriticalError(captured); critical != "" {
		return base + "\r\n" + critical
	}
	return base
}

func (s *coreSupervisor) log(message string) {
	s.logger.log("Core", message)
}

// --- Output capture ------------------------------------------------------

type captureBuffer struct {
	mu      sync.Mutex
	data    []byte
	wrapped bool
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(p) >= maxCaptureBytes {
		b.data = append(b.data[:0], p[len(p)-maxCaptureBytes:]...)
		b.wrapped = true
		return len(p), nil
	}

	if len(b.data)+len(p) <= maxCaptureBytes {
		b.data = append(b.data, p...)
	} else {
		keep := maxCaptureBytes - len(p)
		b.data = append(b.data[:0], b.data[len(b.data)-keep:]...)
		b.data = append(b.data, p...)
		b.wrapped = true
	}
	return len(p), nil
}

func (b *captureBuffer) text() string {
	b.mu.Lock()
	snapshot := append([]byte(nil), b.data...)
	wrapped := b.wrapped
	b.mu.Unlock()

	if len(snapshot) == 0 {
		return ""
	}

	endIdx := bytes.LastIndexByte(snapshot, '\n')
	if endIdx < 0 {
		return ""
	}

	startIdx := 0
	if wrapped {
		first := bytes.IndexByte(snapshot[:endIdx+1], '\n')
		if first < 0 {
			return ""
		}
		startIdx = first + 1
		if startIdx > endIdx {
			return ""
		}
	}

	return strings.TrimSpace(string(snapshot[startIdx : endIdx+1]))
}

type outputReader struct {
	file *os.File
	cap  *captureBuffer
	done chan struct{}
}

func startOutputReader(file *os.File) *outputReader {
	r := &outputReader{file: file, cap: &captureBuffer{}, done: make(chan struct{})}
	go func() {
		_, _ = io.Copy(r.cap, file)
		_ = file.Close()
		close(r.done)
	}()
	return r
}

func (r *outputReader) drain(timeout time.Duration) (string, bool) {
	select {
	case <-r.done:
		return r.cap.text(), true
	case <-time.After(timeout):
	}

	_ = windows.CancelIoEx(windows.Handle(r.file.Fd()), nil)
	select {
	case <-r.done:
		return r.cap.text(), true
	case <-time.After(postKillWait):
		return r.cap.text(), false
	}
}

// --- Critical error extraction ------------------------------------------

func stripSingBoxTimestamp(line string) string {
	if len(line) >= 27 && (line[0] == '+' || line[0] == '-') && line[5] == ' ' && line[10] == '-' &&
		line[13] == '-' && line[16] == ' ' && line[19] == ':' && line[22] == ':' && line[25] == ' ' {
		return line[26:]
	}
	return line
}

type logEntryKind int

const (
	lekNone logEntryKind = iota
	lekTrace
	lekDebug
	lekInfo
	lekWarn
	lekError
	lekFatal
	lekPanic
)

var logEntryPrefixes = []struct {
	prefix string
	kind   logEntryKind
}{
	{"TRACE", lekTrace},
	{"DEBUG", lekDebug},
	{"INFO", lekInfo},
	{"WARN", lekWarn},
	{"ERROR", lekError},
	{"FATAL", lekFatal},
	{"PANIC", lekPanic},
}

func classifyLogEntry(line string) logEntryKind {
	if strings.HasPrefix(line, "panic:") {
		return lekPanic
	}
	if strings.HasPrefix(line, "fatal error:") {
		return lekFatal
	}

	rest := stripSingBoxTimestamp(line)
	for _, entry := range logEntryPrefixes {
		if strings.HasPrefix(rest, entry.prefix+"[") || strings.HasPrefix(rest, entry.prefix+" ") {
			return entry.kind
		}
	}
	return lekNone
}

func extractCriticalError(captured string) string {
	const maxResultChars = 2000

	if captured == "" {
		return ""
	}

	lines := strings.Split(captured, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}

	lastIdx := -1
	var lastKind logEntryKind
	for i := len(lines) - 1; i >= 0; i-- {
		lastKind = classifyLogEntry(lines[i])
		if lastKind != lekNone {
			lastIdx = i
			break
		}
	}

	if lastKind != lekFatal && lastKind != lekPanic {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(stripSingBoxTimestamp(lines[lastIdx]))
	for i := lastIdx + 1; i < len(lines); i++ {
		builder.WriteString("\r\n")
		builder.WriteString(lines[i])
		if builder.Len() > maxResultChars {
			break
		}
	}
	return builder.String()
}
