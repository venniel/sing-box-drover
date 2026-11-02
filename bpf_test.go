package main

import (
	"path/filepath"
	"testing"
)

func TestLooksLikeBpfProfileData(t *testing.T) {
	if !looksLikeBpfProfileData([]byte{bpfMessageTypeProfileContent, bpfVersion1, 0x1F, 0x8B}) {
		t.Error("valid BPF header was not recognized")
	}
	if looksLikeBpfProfileData([]byte(`{"inbounds":[]}`)) {
		t.Error("JSON text was recognized as BPF")
	}
	if looksLikeBpfProfileData([]byte{bpfMessageTypeProfileContent}) {
		t.Error("short data was recognized as BPF")
	}
}

func TestBpfProfileLocalRoundTrip(t *testing.T) {
	profile := bpfProfile{
		Name:        "本地配置",
		ProfileType: bpfProfileTypeLocal,
		ConfigJSON:  `{"inbounds":[],"outbounds":[]}`,
	}

	data, err := encodeBpfProfile(profile)
	if err != nil {
		t.Fatalf("encodeBpfProfile: %v", err)
	}
	if !looksLikeBpfProfileData(data) {
		t.Fatal("encoded profile does not look like BPF")
	}

	decoded, err := decodeBpfProfile(data)
	if err != nil {
		t.Fatalf("decodeBpfProfile: %v", err)
	}
	if decoded.Version != bpfCurrentVersion {
		t.Errorf("Version = %d, want %d", decoded.Version, bpfCurrentVersion)
	}
	if decoded.Name != profile.Name || decoded.ProfileType != profile.ProfileType || decoded.ConfigJSON != profile.ConfigJSON {
		t.Errorf("decoded profile mismatch: %+v", decoded)
	}
	if decoded.RemotePath != "" || decoded.AutoUpdate {
		t.Errorf("local profile should not have remote fields: %+v", decoded)
	}
	if !decoded.IsLocal() || decoded.IsRemote() {
		t.Error("IsLocal/IsRemote mismatch")
	}
}

func TestBpfProfileRemoteRoundTrip(t *testing.T) {
	profile := bpfProfile{
		Version:            bpfCurrentVersion,
		Name:               "remote",
		ProfileType:        bpfProfileTypeRemote,
		ConfigJSON:         `{"inbounds":[]}`,
		RemotePath:         "https://example.com/profile",
		AutoUpdate:         true,
		AutoUpdateInterval: 3600,
		LastUpdated:        1234567890,
	}

	data, err := encodeBpfProfile(profile)
	if err != nil {
		t.Fatalf("encodeBpfProfile: %v", err)
	}
	decoded, err := decodeBpfProfile(data)
	if err != nil {
		t.Fatalf("decodeBpfProfile: %v", err)
	}
	if decoded != profile {
		t.Errorf("decoded = %+v, want %+v", decoded, profile)
	}
	if !decoded.IsRemote() || decoded.IsLocal() {
		t.Error("IsLocal/IsRemote mismatch")
	}
}

func TestBpfProfileICloudRoundTrip(t *testing.T) {
	profile := bpfProfile{
		Name:        "icloud",
		ProfileType: bpfProfileTypeICloud,
		ConfigJSON:  `{"inbounds":[]}`,
		RemotePath:  "iCloud/profile",
	}

	data, err := encodeBpfProfile(profile)
	if err != nil {
		t.Fatalf("encodeBpfProfile: %v", err)
	}
	decoded, err := decodeBpfProfile(data)
	if err != nil {
		t.Fatalf("decodeBpfProfile: %v", err)
	}
	if decoded.Name != profile.Name || decoded.ProfileType != profile.ProfileType || decoded.RemotePath != profile.RemotePath {
		t.Errorf("decoded = %+v, want %+v", decoded, profile)
	}
}

func TestDecodeBpfProfileErrors(t *testing.T) {
	if _, err := decodeBpfProfile([]byte(`{"inbounds":[]}`)); err == nil {
		t.Error("expected an error for non-BPF data")
	}
	if _, err := decodeBpfProfile(nil); err == nil {
		t.Error("expected an error for empty data")
	}

	data, err := encodeBpfProfile(bpfProfile{Name: "x", ProfileType: bpfProfileTypeLocal, ConfigJSON: "{}"})
	if err != nil {
		t.Fatalf("encodeBpfProfile: %v", err)
	}

	tooNew := append([]byte(nil), data...)
	tooNew[1] = bpfCurrentVersion + 1
	if _, err := decodeBpfProfile(tooNew); err == nil {
		t.Error("expected an error for a newer BPF version")
	}

	wrongType := append([]byte(nil), data...)
	wrongType[0] = 99
	if _, err := decodeBpfProfile(wrongType); err == nil {
		t.Error("expected an error for an unsupported message type")
	}
}

func TestBpfProfileFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.bpf")
	profile := bpfProfile{
		Name:        "exported",
		ProfileType: bpfProfileTypeRemote,
		ConfigJSON:  `{"inbounds":[]}`,
		RemotePath:  "https://example.com/sub",
		AutoUpdate:  true,
	}

	if err := writeBpfProfileToFile(path, profile); err != nil {
		t.Fatalf("writeBpfProfileToFile: %v", err)
	}

	src, err := readConfigSource(path)
	if err != nil {
		t.Fatalf("readConfigSource: %v", err)
	}
	if !src.IsBpf() {
		t.Error("BPF file was not detected as BPF")
	}
	if src.JSONText != profile.ConfigJSON {
		t.Errorf("JSONText = %q, want %q", src.JSONText, profile.ConfigJSON)
	}
	if src.BpfProfile.RemotePath != profile.RemotePath {
		t.Errorf("RemotePath = %q", src.BpfProfile.RemotePath)
	}
}
