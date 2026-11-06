package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	updaterInitialDelay    = 60 * time.Second
	updaterMinInterval     = 15 * time.Minute
	updaterConnTimeout     = 15 * time.Second
	updaterResponseTimeout = 30 * time.Second
)

type configUpdater struct {
	source    configSource
	cli       *singBoxCLI
	logger    *logger
	transport *http.Transport
	client    *http.Client

	stopCh   chan struct{}
	stopOnce sync.Once
	done     chan struct{}

	mu     sync.Mutex
	cancel context.CancelFunc
}

func newConfigUpdater(source configSource, cli *singBoxCLI, l *logger) *configUpdater {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: updaterConnTimeout}).DialContext,
		ResponseHeaderTimeout: updaterResponseTimeout,
	}
	u := &configUpdater{
		source:    source,
		cli:       cli,
		logger:    l,
		transport: transport,
		client:    &http.Client{Transport: transport},
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
	go u.run()
	return u
}

func (u *configUpdater) stop() {
	u.stopOnce.Do(func() {
		close(u.stopCh)
		u.mu.Lock()
		if u.cancel != nil {
			u.cancel()
		}
		u.mu.Unlock()
	})
	<-u.done
	u.transport.CloseIdleConnections()
}

func (u *configUpdater) run() {
	defer close(u.done)

	interval := clampUpdateInterval(u.source.BpfProfile.AutoUpdateInterval)

	select {
	case <-time.After(updaterInitialDelay):
	case <-u.stopCh:
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		u.doUpdate()
		select {
		case <-ticker.C:
		case <-u.stopCh:
			return
		}
	}
}

func clampUpdateInterval(minutes int32) time.Duration {
	if minutes < 15 {
		minutes = 15
	}
	return time.Duration(minutes) * time.Minute
}

func (u *configUpdater) doUpdate() {
	select {
	case <-u.stopCh:
		return
	default:
	}

	u.log("Config update started.")

	ctx, cancel := context.WithTimeout(context.Background(), updaterResponseTimeout)
	u.mu.Lock()
	u.cancel = cancel
	u.mu.Unlock()
	defer func() {
		cancel()
		u.mu.Lock()
		u.cancel = nil
		u.mu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.source.BpfProfile.RemotePath, nil)
	if err != nil {
		u.log("Config update failed. " + err.Error())
		return
	}
	req.Header.Set("User-Agent", u.buildUserAgent())

	resp, err := u.client.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			u.log("Config update failed. " + err.Error())
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		u.log(fmt.Sprintf("Config update failed. HTTP %d.", resp.StatusCode))
		return
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		u.log("Config update failed. " + err.Error())
		return
	}

	select {
	case <-u.stopCh:
		return
	default:
	}

	jsonText := string(payload)
	if _, err := readSingBoxConfig(jsonText); err != nil {
		u.log("Config update failed. " + err.Error())
		return
	}

	updated := u.source.BpfProfile
	updated.ConfigJSON = jsonText
	updated.LastUpdated = time.Now().UnixMilli()

	if err := writeBpfProfileToFile(u.source.FilePath, updated); err != nil {
		u.log("Config update failed. " + err.Error())
		return
	}

	u.source.BpfProfile = updated
	u.source.JSONText = jsonText
	u.log("Config updated successfully.")
}

func (u *configUpdater) buildUserAgent() string {
	result := "sing-box-go"
	if version := u.cli.getVersion(); version != "" {
		result += " (sing-box " + version + ")"
	}
	return result
}

func (u *configUpdater) log(message string) {
	u.logger.log("ConfigUpdater", message)
}
