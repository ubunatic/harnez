package agymeter

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunChildProcessHelper(t *testing.T) {
	if os.Getenv("AGYMETER_RUN_CHILD_HELPER") != "1" {
		return
	}
	err := runChild(context.Background(), os.Getenv("AGYMETER_CHILD_COMMAND"), nil, os.Environ(), "", os.Stdin, os.Stdout, os.Stderr)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		os.Exit(125)
	}
	os.Exit(0)
}

func TestRunChildPassesStdinIgnoresInterruptAndReturnsExitCode(t *testing.T) {
	child := filepath.Join(t.TempDir(), "fake-agy")
	script := "#!/bin/sh\nsleep 0.2\nIFS= read -r line\nprintf '%s\\n' \"$line\"\nexit 7\n"
	if err := os.WriteFile(child, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunChildProcessHelper$")
	cmd.Env = append(os.Environ(), "AGYMETER_RUN_CHILD_HELPER=1", "AGYMETER_CHILD_COMMAND="+child)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(350 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := io.WriteString(stdin, "terminal input\n"); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("helper error = %v, want exit code 7", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "terminal input" {
		t.Fatalf("child stdout = %q, want stdin line", got)
	}
}

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

func TestResponseObserverPassesSSEThroughAndKeepsLastUsage(t *testing.T) {
	m, err := newMeter(t.TempDir(), "session-x", "prompt-x")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	input := []byte("data: {\"response\":{\"usageMetadata\":{\"promptTokenCount\":99,\"totalTokenCount\":102}}}\n\ndata: {\"response\":{\"usageMetadata\":{\"promptTokenCount\":11818,\"candidatesTokenCount\":1,\"thoughtsTokenCount\":22,\"totalTokenCount\":11841}}}\n\n")
	body := newResponseObserver(io.NopCloser(bytes.NewReader(input)), m, "/v1internal:streamGenerateContent", "text/event-stream", "", "gemini-test", "session-x", "provider-conversation")
	var forwarded bytes.Buffer
	if _, err = io.Copy(&forwarded, body); err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
	<-body.done
	if !bytes.Equal(input, forwarded.Bytes()) {
		t.Fatal("response bytes changed")
	}
	records := readRecords(t, m.logPath)
	if len(records) != 1 {
		t.Fatalf("got %d records, want one", len(records))
	}
	if records[0].Prompt != 11818 || records[0].Total != 11841 || records[0].Model != "gemini-test" || records[0].Session != "session-x" || records[0].PromptID != "prompt-x" || records[0].Conversation != "provider-conversation" {
		t.Fatalf("record: %+v", records[0])
	}
	raw, err := os.ReadFile(m.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("usageMetadata")) || bytes.Contains(raw, []byte("Authorization")) {
		t.Fatalf("stored response content: %s", raw)
	}
}

func TestResponseObserverDecodesGzipQuotaJSON(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	plain := []byte(`{"groups":[{"buckets":[{"bucketId":"gemini-5h","remainingFraction":0.7592765,"resetTime":"soon"},{"bucketId":"3p-weekly","remainingFraction":0.6,"resetTime":"later"}]}]}`)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write(plain)
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	in := compressed.Bytes()
	body := newResponseObserver(io.NopCloser(bytes.NewReader(in)), m, "/v1internal:retrieveUserQuotaSummary", "application/json", "gzip", "", "", "")
	var forwarded bytes.Buffer
	if _, err = io.Copy(&forwarded, body); err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
	<-body.done
	if !bytes.Equal(in, forwarded.Bytes()) {
		t.Fatal("compressed response bytes changed")
	}
	records := readRecords(t, m.logPath)
	if len(records) != 2 {
		t.Fatalf("got %d quota records, want 2", len(records))
	}
	for _, record := range records {
		if record.Bucket == "gemini-5h" && record.Remaining != nil && *record.Remaining == 0.7592765 {
			return
		}
	}
	t.Fatalf("gemini-5h bucket missing: %+v", records)
}

func TestZeroQuotaFractionIsRecorded(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	input := []byte(`{"groups":[{"buckets":[{"bucketId":"3p-weekly"}]}]}`)
	body := newResponseObserver(io.NopCloser(bytes.NewReader(input)), m, "/v1internal:retrieveUserQuotaSummary", "application/json", "", "", "", "")
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
	<-body.done
	records := readRecords(t, m.logPath)
	if len(records) != 1 || records[0].Remaining == nil || *records[0].Remaining != 0 {
		t.Fatalf("zero fraction missing: %+v", records)
	}
	raw, err := os.ReadFile(m.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"remainingFraction":0`)) {
		t.Fatalf("zero field omitted: %s", raw)
	}
}

func TestQuotaSnapshotsSkipUnchangedIncludingZero(t *testing.T) {
	m, err := newMeter(t.TempDir(), "session", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	zero := float64(0)
	changed := float64(0.25)
	row := Record{Kind: "quota", Bucket: "3p-weekly", Remaining: &zero, Reset: "reset-a"}
	m.record(row)
	m.record(row)
	row.Remaining = &changed
	m.record(row)
	records := readRecords(t, m.logPath)
	if len(records) != 2 || records[0].Remaining == nil || *records[0].Remaining != 0 || records[1].Remaining == nil || *records[1].Remaining != 0.25 {
		t.Fatalf("quota changes: %+v", records)
	}
}

func TestReadUsageRecordsFiltersSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	rows := []Record{{Kind: "usage", Session: "s1", Time: time.Unix(2, 0)}, {Kind: "usage", Session: "s2", Time: time.Unix(1, 0)}}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		b, _ := json.Marshal(r)
		_, _ = f.Write(append(b, '\n'))
	}
	_ = f.Close()
	got, err := ReadUsageRecords(path, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Session != "s1" {
		t.Fatalf("records: %+v", got)
	}
}

func TestSlowParserDoesNotBlockPassthroughOrClose(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	parserDone := make(chan struct{})
	body := newObservedBody(io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), 1<<20))), func(io.Reader) {
		close(started)
		<-release
		close(parserDone)
	}, nil)
	<-started
	var got bytes.Buffer
	if _, err := io.Copy(&got, body); err != nil {
		t.Fatal(err)
	}
	if got.Len() != 1<<20 {
		t.Fatalf("forwarded %d bytes", got.Len())
	}
	closed := make(chan struct{})
	go func() { _ = body.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("body Close waited for the side parser")
	}
	close(release)
	select {
	case <-parserDone:
	case <-time.After(2 * time.Second):
		t.Fatal("side parser did not exit")
	}
	<-body.done
}

func TestOtherHostIsBlindTunnel(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		c, e := target.Accept()
		if e == nil {
			defer c.Close()
			r := bufio.NewReader(c)
			b, _ := r.ReadString('\n')
			if b == "hello\n" {
				_, _ = io.WriteString(c, "world\n")
			}
		}
	}()
	proxy := httptest.NewServer(http.HandlerFunc(m.handle))
	defer proxy.Close()
	c, err := net.Dial("tcp", strings.TrimPrefix(proxy.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target.Addr(), target.Addr()); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(c)
	resp, err := http.ReadResponse(r, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status %d", resp.StatusCode)
	}
	_, _ = io.WriteString(c, "hello\n")
	got, err := r.ReadString('\n')
	if err != nil || got != "world\n" {
		t.Fatalf("tunnel payload %q, err %v", got, err)
	}
}

func TestStartFailureRunsChildPlainly(t *testing.T) {
	badHome := filepath.Join(t.TempDir(), "home")
	if err := os.WriteFile(badHome, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), badHome, "/bin/sh", []string{"-c", "printf plain-child"}, nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "plain-child" {
		t.Fatalf("child output %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "starting agy without metering") {
		t.Fatalf("missing fallback notice: %q", stderr.String())
	}
}

func readRecords(t *testing.T, path string) []Record {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var records []Record
	s := bufio.NewScanner(f)
	for s.Scan() {
		var r Record
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}
