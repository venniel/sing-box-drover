package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	bpfMessageTypeProfileContent = 3

	bpfProfileTypeLocal  = 0
	bpfProfileTypeICloud = 1
	bpfProfileTypeRemote = 2

	bpfVersion0       = 0
	bpfVersion1       = 1
	bpfCurrentVersion = bpfVersion1
)

type bpfProfile struct {
	Version            byte
	Name               string
	ProfileType        int32
	ConfigJSON         string
	RemotePath         string
	AutoUpdate         bool
	AutoUpdateInterval int32
	LastUpdated        int64
}

func (p bpfProfile) IsLocal() bool  { return p.ProfileType == bpfProfileTypeLocal }
func (p bpfProfile) IsRemote() bool { return p.ProfileType == bpfProfileTypeRemote }

func looksLikeBpfProfileData(data []byte) bool {
	return len(data) >= 4 && data[0] == bpfMessageTypeProfileContent && data[2] == 0x1F && data[3] == 0x8B
}

type bpfReader struct {
	data []byte
	pos  int
}

func (r *bpfReader) require(count int) error {
	if r.pos < 0 || count < 0 || r.pos > len(r.data)-count {
		return fmt.Errorf("Unexpected end of BPF data.")
	}
	return nil
}

func (r *bpfReader) readByte() (byte, error) {
	if err := r.require(1); err != nil {
		return 0, err
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

func (r *bpfReader) readBool() (bool, error) {
	v, err := r.readByte()
	if err != nil {
		return false, err
	}
	switch v {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("Invalid BPF boolean value.")
	}
}

func (r *bpfReader) readUVarInt() (uint64, error) {
	var result uint64
	shift := 0
	for shift <= 63 {
		v, err := r.readByte()
		if err != nil {
			return 0, err
		}
		if shift == 63 && v > 1 {
			return 0, fmt.Errorf("Invalid BPF varint.")
		}
		result |= uint64(v&0x7F) << uint(shift)
		if v&0x80 == 0 {
			return result, nil
		}
		shift += 7
	}
	return 0, fmt.Errorf("Invalid BPF varint.")
}

func (r *bpfReader) readInt32BE() (int32, error) {
	if err := r.require(4); err != nil {
		return 0, err
	}
	v := int32(binary.BigEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return v, nil
}

func (r *bpfReader) readInt64BE() (int64, error) {
	if err := r.require(8); err != nil {
		return 0, err
	}
	v := int64(binary.BigEndian.Uint64(r.data[r.pos:]))
	r.pos += 8
	return v, nil
}

func (r *bpfReader) readString() (string, error) {
	n, err := r.readUVarInt()
	if err != nil {
		return "", err
	}
	maxInt := int(^uint(0) >> 1)
	if n > uint64(maxInt) {
		return "", fmt.Errorf("BPF string is too large.")
	}
	count := int(n)
	if err := r.require(count); err != nil {
		return "", err
	}
	s := string(r.data[r.pos : r.pos+count])
	r.pos += count
	return s, nil
}

func decompressBPF(data []byte) ([]byte, error) {
	if gr, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
		defer gr.Close()
		if out, err := io.ReadAll(gr); err == nil {
			return out, nil
		}
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func decodeBpfProfile(data []byte) (bpfProfile, error) {
	var profile bpfProfile
	if !looksLikeBpfProfileData(data) {
		return profile, fmt.Errorf("Data is not a BPF profile.")
	}

	r := &bpfReader{data: data}
	msgType, err := r.readByte()
	if err != nil {
		return profile, err
	}
	if msgType != bpfMessageTypeProfileContent {
		return profile, fmt.Errorf("Unsupported BPF message type.")
	}

	version, err := r.readByte()
	if err != nil {
		return profile, err
	}
	if version > bpfCurrentVersion {
		return profile, fmt.Errorf("Unsupported BPF version: %d.", version)
	}
	profile.Version = version

	payload, err := decompressBPF(data[2:])
	if err != nil {
		return profile, fmt.Errorf("Failed to decode BPF profile.")
	}
	pr := &bpfReader{data: payload}

	if profile.Name, err = pr.readString(); err != nil {
		return profile, err
	}
	if profile.ProfileType, err = pr.readInt32BE(); err != nil {
		return profile, err
	}
	if profile.ConfigJSON, err = pr.readString(); err != nil {
		return profile, err
	}
	if profile.ProfileType != bpfProfileTypeLocal {
		if profile.RemotePath, err = pr.readString(); err != nil {
			return profile, err
		}
	}

	if profile.ProfileType == bpfProfileTypeRemote ||
		(profile.Version == bpfVersion0 && profile.ProfileType != bpfProfileTypeLocal) {
		if profile.AutoUpdate, err = pr.readBool(); err != nil {
			return profile, err
		}
		if profile.Version >= bpfVersion1 {
			if profile.AutoUpdateInterval, err = pr.readInt32BE(); err != nil {
				return profile, err
			}
		}
		if profile.LastUpdated, err = pr.readInt64BE(); err != nil {
			return profile, err
		}
	}

	return profile, nil
}

type bpfWriter struct {
	buf bytes.Buffer
}

func (w *bpfWriter) writeByte(v byte) { w.buf.WriteByte(v) }

func (w *bpfWriter) writeBool(v bool) {
	if v {
		w.writeByte(1)
	} else {
		w.writeByte(0)
	}
}

func (w *bpfWriter) writeUVarInt(v uint64) {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		w.writeByte(b)
		if v == 0 {
			return
		}
	}
}

func (w *bpfWriter) writeInt32BE(v int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	w.buf.Write(b[:])
}

func (w *bpfWriter) writeInt64BE(v int64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	w.buf.Write(b[:])
}

func (w *bpfWriter) writeString(s string) {
	w.writeUVarInt(uint64(len(s)))
	w.buf.WriteString(s)
}

func encodeBpfProfile(profile bpfProfile) ([]byte, error) {
	if profile.Version == bpfVersion0 {
		profile.Version = bpfCurrentVersion
	}

	var payload bpfWriter
	payload.writeString(profile.Name)
	payload.writeInt32BE(profile.ProfileType)
	payload.writeString(profile.ConfigJSON)
	if profile.ProfileType != bpfProfileTypeLocal {
		payload.writeString(profile.RemotePath)
	}
	if profile.ProfileType == bpfProfileTypeRemote {
		payload.writeBool(profile.AutoUpdate)
		payload.writeInt32BE(profile.AutoUpdateInterval)
		payload.writeInt64BE(profile.LastUpdated)
	}

	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write(payload.buf.Bytes()); err != nil {
		_ = gw.Close()
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	out := make([]byte, 0, 2+compressed.Len())
	out = append(out, bpfMessageTypeProfileContent, profile.Version)
	out = append(out, compressed.Bytes()...)
	return out, nil
}

func writeBpfProfileToFile(fileName string, profile bpfProfile) error {
	data, err := encodeBpfProfile(profile)
	if err != nil {
		return fmt.Errorf("Failed to write BPF profile file.")
	}
	if err := os.WriteFile(fileName, data, 0o644); err != nil {
		return fmt.Errorf("Failed to write BPF profile file.")
	}
	return nil
}
