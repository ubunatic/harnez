package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/decide"
)

func TestDecideRequestQuick(t *testing.T) {
	o := &decideOpts{id: "q", noul: "Destructive?", options: []string{"true: deletes", "false: safe"}, state: `{"cmd":"rm -rf x"}`}
	req, err := decideRequest(o, nil, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	q := req.Questions["q"]
	if q.Type != decide.TypeNoul || q.Criteria.(map[string]string)["true"] != "deletes" {
		t.Fatalf("question %+v", q)
	}
	if req.State.(map[string]any)["cmd"] != "rm -rf x" {
		t.Fatalf("state %#v", req.State)
	}
}

func TestDecideRequestStateSources(t *testing.T) {
	base := func() *decideOpts {
		return &decideOpts{id: "q", score: "Severe?", levels: []string{"low", "high"}}
	}
	o := base()
	o.state = "plain text"
	if req, _ := decideRequest(o, nil, strings.NewReader("")); req.State != "plain text" {
		t.Fatalf("text state %#v", req.State)
	}
	if req, _ := decideRequest(base(), []string{"from", "args"}, strings.NewReader("")); req.State != "from args" {
		t.Fatalf("args state %#v", req.State)
	}
	if req, _ := decideRequest(base(), nil, strings.NewReader("piped\n")); req.State != "piped" {
		t.Fatalf("stdin state %#v", req.State)
	}
	if _, err := decideRequest(base(), nil, strings.NewReader("")); err == nil {
		t.Fatal("no state: want error")
	}
}

func TestDecideRequestFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gate.yaml")
	os.WriteFile(p, []byte("state: {branch: main}\nquestions:\n  risk:\n    type: choice\n    instructions: Classify\n    criteria: {safe: ok, danger: bad}\n"), 0o644)
	req, err := decideRequest(&decideOpts{file: p}, nil, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if req.Questions["risk"].Type != "choice" || req.State.(map[string]any)["branch"] != "main" {
		t.Fatalf("request %+v", req)
	}
}

// A file with a state must not read stdin (a never-closing pipe would hang);
// --state still replaces the file's state.
func TestDecideRequestFileStateSkipsStdin(t *testing.T) {
	p := filepath.Join(t.TempDir(), "gate.yaml")
	os.WriteFile(p, []byte("state: from file\nquestions: {q: {type: noul, instructions: x}}\n"), 0o644)
	req, err := decideRequest(&decideOpts{file: p}, nil, panicReader{})
	if err != nil || req.State != "from file" {
		t.Fatalf("state %#v %v", req, err)
	}
	req, _ = decideRequest(&decideOpts{file: p, state: "flag"}, nil, panicReader{})
	if req.State != "flag" {
		t.Fatalf("flag state %#v", req.State)
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("stdin read") }

func TestDecideRequestRejects(t *testing.T) {
	for name, o := range map[string]*decideOpts{
		"none":         {state: "x"},
		"two quick":    {noul: "a", choice: "b", state: "x"},
		"noul option":  {noul: "a", options: []string{"maybe: x"}, state: "x"},
		"one choice":   {choice: "a", options: []string{"a: x"}, state: "x"},
		"bad option":   {choice: "a", options: []string{"no colon", "b: y"}, state: "x"},
		"one level":    {score: "a", levels: []string{"x"}, state: "x"},
		"level+choice": {choice: "a", options: []string{"a: x", "b: y"}, levels: []string{"l"}, state: "x"},
		"option+score": {score: "a", levels: []string{"x", "y"}, options: []string{"a: x"}, state: "x"},
	} {
		if _, err := decideRequest(o, nil, strings.NewReader("")); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// End to end against a fake backend: --pick output and --threshold exit.
func TestDecideCmdThreshold(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") == "Bearer broken" {
			w.WriteHeader(500)
			return
		}
		w.Write([]byte(`{"answers":{"q":{"type":"noul","noul":0.3}}}`))
	}))
	defer srv.Close()
	t.Setenv("TYPESAFE_API_KEY", "k")
	t.Setenv("TYPESAFE_API_BASE", srv.URL)

	run := func(args ...string) (string, error) {
		cmd := newDecideCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetIn(strings.NewReader(""))
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}
	out, err := run("--noul", "Risky?", "--pick", "q", "ls")
	if err != nil || strings.TrimSpace(out) != "0.30" {
		t.Fatalf("pick: %q %v", out, err)
	}
	if _, err := run("--noul", "Risky?", "--threshold", "0.2", "--format", "quiet", "ls"); err != nil {
		t.Fatalf("threshold met: %v", err)
	}
	_, err = run("--noul", "Risky?", "--threshold", "0.5", "--format", "quiet", "ls")
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.Code != 1 {
		t.Fatalf("threshold missed: %v", err)
	}

	if out, err := run("--noul", "Risky?", "--pick", "q", "--format", "json", "ls"); err != nil || !strings.Contains(out, `"noul": 0.3`) {
		t.Fatalf("pick json: %q %v", out, err)
	}

	// Errors exit 2, never 1, so a gate can tell "no" from "broken".
	t.Setenv("TYPESAFE_API_KEY", "broken")
	_, err = run("--noul", "Risky?", "--threshold", "0.5", "--format", "quiet", "ls")
	if !errors.As(err, &ec) || ec.Code != 2 {
		t.Fatalf("http error: %v", err)
	}

	// A bad --format fails before the paid call.
	before := calls
	_, err = run("--noul", "Risky?", "--format", "yaml", "ls")
	if !errors.As(err, &ec) || ec.Code != 2 || calls != before {
		t.Fatalf("bad format: %v, calls %d->%d", err, before, calls)
	}
}
