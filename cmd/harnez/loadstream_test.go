package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

type cancelOnLineWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
	wrote  bool
}

func (w *cancelOnLineWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if !w.wrote && bytes.Contains(p, []byte("{")) {
		w.wrote = true
		w.cancel()
	}
	return n, err
}

func TestLoadStreamCommandEmitsTimestampedSample(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out cancelOnLineWriter
	out.cancel = cancel
	cmd := newLoadStreamCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	stdin, stdinWriter := io.Pipe()
	defer stdin.Close()
	defer stdinWriter.Close()
	cmd.SetIn(stdin)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var sample usage.LoadSnapshot
	if err := json.Unmarshal(bytes.SplitN(bytes.TrimSpace(out.Bytes()), []byte("\n"), 2)[0], &sample); err != nil {
		t.Fatalf("decode load-stream sample: %v", err)
	}
	if sample.ObservedAt.IsZero() || time.Since(sample.ObservedAt) > time.Minute {
		t.Fatalf("sample observation time = %s, want a current timestamp", sample.ObservedAt)
	}
	if sample.CPU.NumCPU == 0 || !sample.CPU.Memory.Ok || !sample.CPU.Memory.SwapKnown {
		t.Fatalf("load-stream sample lacks CPU/memory/swap readings: %+v", sample)
	}
}
