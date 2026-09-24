package agymeter

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCAAndBundle(t *testing.T) {
	home := t.TempDir()
	m, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if !strings.HasPrefix(m.listener.Addr().String(), "127.0.0.1:") {
		t.Fatalf("listener bound to %s", m.listener.Addr())
	}
	key := filepath.Join(home, ".harnez", "agymeter", "ca.key")
	info, err := os.Stat(key)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("key mode %o", info.Mode().Perm())
	}
	bundle, err := m.Bundle()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(bundle)
	ca, _ := os.ReadFile(filepath.Join(filepath.Dir(bundle), "ca.crt"))
	if !bytes.Contains(b, ca) {
		t.Fatal("bundle does not include meter CA")
	}
}

func TestObserveSSERecordsOnlyUsage(t *testing.T) {
	home := t.TempDir()
	m, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	input := []byte("data: {\"usageMetadata\":{\"promptTokenCount\":99,\"candidatesTokenCount\":3,\"thoughtsTokenCount\":4,\"cachedContentTokenCount\":5,\"totalTokenCount\":111}}\n\ndata: [DONE]\n")
	var delivered bytes.Buffer
	observer := &observed{ReadCloser: io.NopCloser(bytes.NewReader(input)), fn: func(b []byte) { m.observe(nil, b, "gemini-test", "session-x") }}
	if _, err = io.Copy(&delivered, observer); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, delivered.Bytes()) {
		t.Fatal("response bytes changed")
	}
	raw, err := os.ReadFile(m.logPath)
	if err != nil {
		t.Fatal(err)
	}
	var rec Record
	if err = json.Unmarshal(bytes.TrimSpace(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Prompt != 99 || rec.Total != 111 || rec.Model != "gemini-test" || rec.Session != "session-x" {
		t.Fatalf("record: %+v", rec)
	}
	if bytes.Contains(raw, []byte("Authorization")) || bytes.Contains(raw, []byte("usageMetadata")) {
		t.Fatalf("stored sensitive response content: %s", raw)
	}
}

func TestObserveQuotaBuckets(t *testing.T) {
	home := t.TempDir()
	m, err := New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.observe(nil, []byte("data: {\"groups\":[{\"buckets\":[{\"bucketId\":\"gemini-5h\",\"remainingFraction\":0.7592765,\"resetTime\":\"soon\"}]}]}\n"), "", "")
	raw, err := os.ReadFile(m.logPath)
	if err != nil {
		t.Fatal(err)
	}
	var rec Record
	if err = json.Unmarshal(bytes.TrimSpace(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Kind != "quota" || rec.Bucket != "gemini-5h" || rec.Remaining != 0.7592765 {
		t.Fatalf("record: %+v", rec)
	}
}
