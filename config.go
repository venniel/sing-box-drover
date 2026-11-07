package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type configSelector struct {
	Name         string
	Outbounds    []string
	DefaultIndex int
	DefaultName  string
}

type clashAPIConfig struct {
	ExternalController string
	ExternalUI         string
	Secret             string
}

func (c clashAPIConfig) IsConfigured() bool {
	return c.ExternalController != ""
}

// onlineDashboardURL is opened when the config does not serve a dashboard
// of its own.
const onlineDashboardURL = "https://venniel.github.io/Yacd-meta"

// DashboardURL returns the address opened by the tray menu's dashboard
// item. A config with external_ui points at the copy the core serves; an
// absolute external_ui URL is used as-is. Without external_ui the hosted
// yacd panel is opened.
func (c clashAPIConfig) DashboardURL() string {
	ui := strings.TrimSpace(c.ExternalUI)
	if ui == "" {
		return onlineDashboardURL
	}
	if hasPrefixFold(ui, "http://") || hasPrefixFold(ui, "https://") {
		return ui
	}
	if c.ExternalController == "" {
		return onlineDashboardURL
	}

	host := c.ExternalController
	if strings.HasPrefix(host, "0.0.0.0:") {
		host = "127.0.0.1" + host[len("0.0.0.0"):]
	} else if host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	base := "http://" + host + "/ui/"
	if c.Secret != "" {
		base += "?secret=" + url.QueryEscape(c.Secret)
	}
	return base
}

type singBoxConfig struct {
	ClashAPI       clashAPIConfig
	Selectors      []configSelector
	ProxyHost      string
	ProxyPort      int
	HasTunInbound  bool
	JSONWithTun    string
	JSONWithoutTun string
}

func normalizeJSON(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	inString := false
	escape := false
	inSingle := false
	inMulti := false
	pendingComma := false

	for i := 0; i < len(src); i++ {
		ch := src[i]

		if inSingle {
			if ch == '\n' || ch == '\r' {
				inSingle = false
			}
			continue
		}
		if inMulti {
			if ch == '*' && i+1 < len(src) && src[i+1] == '/' {
				inMulti = false
				i++
			}
			continue
		}
		if inString {
			b.WriteByte(ch)
			if escape {
				escape = false
			} else if ch == '\\' {
				escape = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '/' && i+1 < len(src) {
			next := src[i+1]
			if next == '/' {
				inSingle = true
				i++
				continue
			}
			if next == '*' {
				inMulti = true
				i++
				continue
			}
		}

		if ch == ',' {
			pendingComma = true
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			continue
		}

		if pendingComma {
			if ch != '}' && ch != ']' {
				b.WriteByte(',')
			}
			pendingComma = false
		}

		if ch == '"' {
			inString = true
			escape = false
			b.WriteByte(ch)
			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}

func readConfigText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Configuration file not found.")
		}
		return "", fmt.Errorf("Failed to read configuration file.")
	}
	return utf8TextFromBytes(data), nil
}

func utf8TextFromBytes(data []byte) string {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return string(data[3:])
	}
	return string(data)
}

func readSingBoxConfig(jsonText string) (singBoxConfig, error) {
	var cfg singBoxConfig

	normalized := normalizeJSON(jsonText)
	dec := json.NewDecoder(strings.NewReader(normalized))
	dec.UseNumber()

	var rootValue any
	if err := dec.Decode(&rootValue); err != nil {
		return cfg, fmt.Errorf("Configuration file is corrupted or contains invalid JSON.")
	}
	root, ok := rootValue.(map[string]any)
	if !ok {
		return cfg, fmt.Errorf("Invalid JSON.")
	}

	if inbounds, ok := root["inbounds"].([]any); ok {
		for _, item := range inbounds {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			inboundType := getString(obj, "type")
			if strings.EqualFold(inboundType, "mixed") {
				cfg.ProxyHost = getString(obj, "listen")
				cfg.ProxyPort = parseIntSafe(getString(obj, "listen_port"))
			}
			if strings.EqualFold(inboundType, "tun") {
				cfg.HasTunInbound = true
			}
		}
	}

	if outbounds, ok := root["outbounds"].([]any); ok {
		for _, item := range outbounds {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if !strings.EqualFold(getString(obj, "type"), "selector") {
				continue
			}
			sel := configSelector{
				Name:         getString(obj, "tag"),
				DefaultName:  getString(obj, "default"),
				DefaultIndex: -1,
			}
			if list, ok := obj["outbounds"].([]any); ok {
				for i, raw := range list {
					name := stringValue(raw)
					sel.Outbounds = append(sel.Outbounds, name)
					if sel.DefaultName == name {
						sel.DefaultIndex = i
					}
				}
			}
			if len(sel.Outbounds) > 0 {
				cfg.Selectors = append(cfg.Selectors, sel)
			}
		}
	}

	cfg.ClashAPI = clashAPIConfig{}
	clashPath := lookupNested(root, "experimental", "clash_api")
	if clashObj, ok := clashPath.(map[string]any); ok {
		cfg.ClashAPI.ExternalController = getString(clashObj, "external_controller")
		cfg.ClashAPI.ExternalUI = getString(clashObj, "external_ui")
		cfg.ClashAPI.Secret = getString(clashObj, "secret")
	} else if len(cfg.Selectors) > 0 {
		createDefaultClashAPI(root, &cfg)
	}

	withTun, err := marshalJSONCompact(root)
	if err != nil {
		return cfg, fmt.Errorf("Failed to serialize configuration.")
	}
	cfg.JSONWithTun = withTun
	cfg.JSONWithoutTun = withTun

	if cfg.HasTunInbound {
		removeTunInbounds(root)
		withoutTun, err := marshalJSONCompact(root)
		if err != nil {
			return cfg, fmt.Errorf("Failed to serialize configuration.")
		}
		cfg.JSONWithoutTun = withoutTun
	}

	return cfg, nil
}

func checkSingBoxConfig(cfg singBoxConfig) error {
	if cfg.ProxyHost == "" || cfg.ProxyPort < 1 {
		return fmt.Errorf("No suitable mixed inbound found for the system proxy.")
	}
	return nil
}

func lookupNested(root map[string]any, path ...string) any {
	var current any = root
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = obj[key]
		if !ok {
			return nil
		}
	}
	return current
}

func getString(obj map[string]any, name string) string {
	v, ok := obj[name]
	if !ok {
		return ""
	}
	return stringValue(v)
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		return ""
	}
}

func parseIntSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func createDefaultClashAPI(root map[string]any, cfg *singBoxConfig) {
	expVal, hasExp := root["experimental"]
	var expObj map[string]any
	if hasExp {
		expObj, _ = expVal.(map[string]any)
		if expObj == nil {
			return
		}
	} else {
		expObj = map[string]any{}
		root["experimental"] = expObj
	}

	if _, exists := expObj["clash_api"]; exists {
		return
	}

	controller := "127.0.0.1:9090"
	secret := newGUIDString()

	expObj["clash_api"] = map[string]any{
		"external_controller": controller,
		"secret":              secret,
	}

	cfg.ClashAPI.ExternalController = controller
	cfg.ClashAPI.Secret = secret
}

func newGUIDString() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "{00000000-0000-0000-0000-000000000000}"
	}
	b[6] = (b[6] & 0x0F) | 0x40
	b[8] = (b[8] & 0x3F) | 0x80
	return fmt.Sprintf("{%08X-%04X-%04X-%04X-%012X}",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]))
}

func removeTunInbounds(root map[string]any) {
	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		return
	}
	kept := make([]any, 0, len(inbounds))
	for _, item := range inbounds {
		obj, ok := item.(map[string]any)
		if ok && strings.EqualFold(getString(obj, "type"), "tun") {
			continue
		}
		kept = append(kept, item)
	}
	root["inbounds"] = kept
}

func marshalJSONCompact(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
