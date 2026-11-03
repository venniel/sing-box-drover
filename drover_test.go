package main

import "testing"

func TestApplySelectorNow(t *testing.T) {
	selectors := []configSelector{
		{Name: "first", Outbounds: []string{"A1", "A2"}, DefaultIndex: 0},
		{Name: "second", Outbounds: []string{"B1", "B2"}, DefaultIndex: -1},
	}

	if applySelectorNow(selectors, "first", "A1") {
		t.Error("same index should not report a change")
	}
	if !applySelectorNow(selectors, "first", "A2") {
		t.Error("expected a change for a different outbound")
	}
	if selectors[0].DefaultIndex != 1 {
		t.Errorf("DefaultIndex = %d, want 1", selectors[0].DefaultIndex)
	}

	if !applySelectorNow(selectors, "second", "B1") {
		t.Error("expected a change when there was no selection")
	}
	if selectors[1].DefaultIndex != 0 {
		t.Errorf("DefaultIndex = %d, want 0", selectors[1].DefaultIndex)
	}

	if applySelectorNow(selectors, "missing", "A1") {
		t.Error("unknown selector should not report a change")
	}
	if applySelectorNow(selectors, "first", "unknown") {
		t.Error("unknown outbound should not report a change")
	}
	if selectors[0].DefaultIndex != 1 {
		t.Error("failed lookups must not modify the selection")
	}
}
