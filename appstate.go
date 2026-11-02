package main

import (
	"encoding/json"
	"os"
	"sync"
)

const stateSelectorsKey = "selectors"

type appState struct {
	mu        sync.Mutex
	filePath  string
	root      map[string]any
	selectors map[string]string
}

func newAppState(filePath string) *appState {
	s := &appState{
		filePath:  filePath,
		root:      map[string]any{},
		selectors: map[string]string{},
	}
	s.load()
	return s
}

func (s *appState) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.root = map[string]any{}
	s.selectors = map[string]string{}

	data, err := os.ReadFile(s.filePath)
	if err != nil || len(data) == 0 {
		return
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return
	}
	s.root = root

	values, ok := root[stateSelectorsKey].(map[string]any)
	if !ok {
		return
	}
	for name, raw := range values {
		if text, ok := raw.(string); ok {
			s.selectors[name] = text
		}
	}
}

func (s *appState) buildJSONText() []byte {
	root := make(map[string]any, len(s.root)+1)
	for k, v := range s.root {
		root[k] = v
	}
	selectors := make(map[string]any, len(s.selectors))
	for k, v := range s.selectors {
		selectors[k] = v
	}
	root[stateSelectorsKey] = selectors

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return []byte("{\"selectors\":{}}")
	}
	return data
}

func (s *appState) save() error {
	return os.WriteFile(s.filePath, s.buildJSONText(), 0o644)
}

func (s *appState) getSelector(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.selectors[name]
	return value, ok
}

func (s *appState) syncSelectors(values map[string]string, scope []string) error {
	if len(values) == 0 && len(scope) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range scope {
		if _, ok := values[name]; !ok {
			delete(s.selectors, name)
		}
	}
	for name, value := range values {
		s.selectors[name] = value
	}
	return s.save()
}

func (s *appState) getBool(name string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.root[name].(bool)
	return value, ok
}

func (s *appState) setBool(name string, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root[name] = value
	return s.save()
}
