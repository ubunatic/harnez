package claude

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// CaptureTestStdout runs f while capturing stdout. The reader drains the pipe
// concurrently so writes larger than the pipe buffer cannot block f.
func CaptureTestStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	var buf bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = io.Copy(&buf, r)
	}()
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
		<-readDone
		_ = r.Close()
	}()

	callErr := f()
	_ = w.Close()
	<-readDone
	return buf.String(), callErr
}

func TestCaptureTestStdoutDrainsBeforeCallback(t *testing.T) {
	if os.Getenv("HARNEZ_STDOUT_CAPTURE_OLD_ORDER") == "1" {
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w
		_, _ = os.Stdout.Write(bytes.Repeat([]byte("x"), 256<<10))
		_ = w.Close()
		os.Stdout = oldStdout
		_ = r.Close()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCaptureTestStdoutDrainsBeforeCallback$")
	cmd.Env = append(os.Environ(), "HARNEZ_STDOUT_CAPTURE_OLD_ORDER=1")
	if err := cmd.Run(); ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("old capture ordering should block on 256 KiB, command err=%v, context err=%v", err, ctx.Err())
	}

	const size = 256 << 10
	out, err := CaptureTestStdout(func() error {
		_, err := os.Stdout.Write(bytes.Repeat([]byte("x"), size))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != size {
		t.Fatalf("captured %d bytes, want %d", len(out), size)
	}
}
