package forge

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriterRequests(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method || r.URL.Path != path || r.Header.Get("Authorization") != "token tok" {
			t.Errorf("request %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		data, _ := io.ReadAll(r.Body)
		if string(data) != body {
			t.Errorf("body %q want %q", data, body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":7}`))
	}))
	defer srv.Close()
	fg := Forge{Kind: "forgejo", APIBase: srv.URL + "/api/v1"}
	wr := Writer{Client: srv.Client(), Token: "tok"}
	method, path, body = "POST", "/api/v1/repos/o/r/issues", `{"title":"T","body":"B\n\nx"}`
	if _, err := wr.CreateIssue(fg, "o", "r", "T", "B\n\nx", nil); err != nil {
		t.Fatal(err)
	}
	body = `{"title":"T","body":"B\n\nx","labels":[1,2]}`
	if _, err := wr.CreateIssue(fg, "o", "r", "T", "B\n\nx", []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	method, path, body = "PATCH", "/api/v1/repos/o/r/issues/7", `{"body":"B"}`
	b := "B"
	if _, err := wr.EditIssue(fg, "o", "r", 7, EditFields{Body: &b}); err != nil {
		t.Fatal(err)
	}
	body = `{"state":"closed"}`
	state := "closed"
	if _, err := wr.EditIssue(fg, "o", "r", 7, EditFields{State: &state}); err != nil {
		t.Fatal(err)
	}
	method, path, body = "POST", "/api/v1/repos/o/r/issues/7/comments", `{"body":"c"}`
	if err := wr.Comment(fg, "o", "r", 7, "c"); err != nil {
		t.Fatal(err)
	}
}

func TestWriterErrorsAndLabels(t *testing.T) {
	status, calls := 200, 0
	response := `[{"id":1,"name":"bug"},{"id":2,"name":"P2"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()
	fg := Forge{Kind: "forgejo", APIBase: srv.URL}
	wr := Writer{Client: srv.Client(), Token: "tok"}
	if _, err := wr.LabelIDs(fg, "o", "r", []string{"bug", "nope"}); err == nil || !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "bug") || !strings.Contains(err.Error(), "P2") {
		t.Fatal(err)
	}
	for _, status = range []int{403, 404, 412, 500} {
		calls = 0
		response = `{"message":"safe detail"}`
		_, err := wr.CreateIssue(fg, "o", "r", "T", "private body", nil)
		if err == nil || !strings.Contains(err.Error(), "HTTP ") || strings.Contains(err.Error(), "tok") || strings.Contains(err.Error(), "private body") || calls != 1 {
			t.Fatalf("status %d: %v calls=%d", status, err, calls)
		}
		if status == 412 && !strings.Contains(err.Error(), "safe detail") {
			t.Fatal(err)
		}
	}
	status = 200
	response = "not json"
	if _, err := wr.CreateIssue(fg, "o", "r", "T", "B", nil); err == nil {
		t.Fatal("invalid json accepted")
	}
	calls = 0
	fg.Kind = "github"
	if _, err := wr.CreateIssue(fg, "o", "r", "T", "B", nil); err == nil || !strings.Contains(err.Error(), "1209") || calls != 0 {
		t.Fatalf("github: %v calls=%d", err, calls)
	}
}
