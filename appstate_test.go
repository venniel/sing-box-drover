package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := newAppState(path)

	if _, ok := s.getBool("tun"); ok {
		t.Error("missing state should have no boolean values")
	}
	if _, ok := s.getSelector("sel"); ok {
		t.Error("missing state should have no selectors")
	}
}

func TestAppStateCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newAppState(path)
	if _, ok := s.getBool("tun"); ok {
		t.Error("corrupt state should be ignored")
	}
}

func TestAppStateBoolPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := newAppState(path)

	if err := s.setBool("tun", true); err != nil {
		t.Fatal(err)
	}
	if err := s.setBool("systemProxy", false); err != nil {
		t.Fatal(err)
	}

	if v, ok := s.getBool("tun"); !ok || !v {
		t.Errorf("getBool(tun) = %v, %v", v, ok)
	}

	reloaded := newAppState(path)
	if v, ok := reloaded.getBool("tun"); !ok || !v {
		t.Errorf("reloaded getBool(tun) = %v, %v", v, ok)
	}
	if v, ok := reloaded.getBool("systemProxy"); !ok || v {
		t.Errorf("reloaded getBool(systemProxy) = %v, %v", v, ok)
	}
}

func TestAppStateSelectorsAndSync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	initial := `{"selectors":{"proxy-select":"Proxy1","old-select":"A"}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newAppState(path)
	if v, ok := s.getSelector("proxy-select"); !ok || v != "Proxy1" {
		t.Errorf("getSelector(proxy-select) = %q, %v", v, ok)
	}

	if err := s.syncSelectors(map[string]string{"proxy-select": "Proxy2", "new-select": "B"}, []string{"proxy-select", "old-select"}); err != nil {
		t.Fatal(err)
	}

	if v, ok := s.getSelector("proxy-select"); !ok || v != "Proxy2" {
		t.Errorf("updated selector = %q, %v", v, ok)
	}
	if v, ok := s.getSelector("new-select"); !ok || v != "B" {
		t.Errorf("new selector = %q, %v", v, ok)
	}
	if _, ok := s.getSelector("old-select"); ok {
		t.Error("selector in scope but missing from values should be removed")
	}

	reloaded := newAppState(path)
	if v, ok := reloaded.getSelector("proxy-select"); !ok || v != "Proxy2" {
		t.Errorf("reloaded selector = %q, %v", v, ok)
	}
	if _, ok := reloaded.getSelector("old-select"); ok {
		t.Error("removed selector came back after reload")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("state file is not valid JSON: %v", err)
	}
	if _, ok := root[stateSelectorsKey].(map[string]any); !ok {
		t.Error("state file is missing the selectors object")
	}
}

func TestAppStatePreservesUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	initial := `{"custom":"value","selectors":{}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newAppState(path)
	if err := s.setBool("tun", true); err != nil {
		t.Fatal(err)
	}

	reloaded := newAppState(path)
	if v, ok := reloaded.root["custom"]; !ok || v != "value" {
		t.Errorf("unknown key lost: %v, %v", v, ok)
	}
}
