package forge

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failTransport struct{ err error }

func (f failTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func TestWriterErrorDetails(t *testing.T) {
	message := "token does not have at least one of required scope(s): [write:issue]"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"message":%q}`, message)
	}))
	defer srv.Close()
	fg := Forge{Kind: "forgejo", APIBase: srv.URL}
	wr := Writer{Client: srv.Client(), Token: "super-secret"}
	_, err := wr.CreateIssue(fg, "o", "r", "Title", "Body", nil)
	if err == nil || err.Error() != "forge API request failed: HTTP 403 "+message {
		t.Fatalf("got %v", err)
	}
	message = "exposed super-secret"
	_, err = wr.CreateIssue(fg, "o", "r", "Title", "Body", nil)
	if err == nil || err.Error() != "forge API request failed: HTTP 403 " {
		t.Fatalf("got %v", err)
	}
	cause := errors.New("transport failed")
	wr.Client = &http.Client{Transport: failTransport{cause}}
	_, err = wr.CreateIssue(fg, "o", "r", "Title", "Body", nil)
	if !errors.Is(err, cause) {
		t.Fatalf("cause lost: %v", err)
	}
}

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
	if _, err := wr.Comment(fg, "o", "r", 7, "c"); err != nil {
		t.Fatal(err)
	}
}

func TestWriterNilClient(t *testing.T) {
	_, err := (Writer{Token: "tok"}).CreateIssue(Forge{Kind: "forgejo", APIBase: "https://example.invalid"}, "o", "r", "T", "B", nil)
	if err == nil || err.Error() != "forge write: no HTTP client" {
		t.Fatalf("got %v", err)
	}
}

func TestWriterRefinements(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/labels":
			page++
			if r.Method != "GET" || r.URL.RawQuery != fmt.Sprintf("limit=50&page=%d", page) {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			}
			if page == 1 {
				fmt.Fprint(w, "[")
				for i := 1; i <= 50; i++ {
					if i > 1 {
						fmt.Fprint(w, ",")
					}
					fmt.Fprintf(w, `{"id":%d,"name":"n%d"}`, i, i)
				}
				fmt.Fprint(w, "]")
			} else {
				fmt.Fprint(w, `[{"id":51,"name":"bug"}]`)
			}
		case "/repos/o/r/issues":
			fmt.Fprint(w, `{"number":7,"html_url":"https://example/7","updated_at":"2026-01-02T03:04:05Z"}`)
		case "/repos/o/r/issues/7/comments":
			fmt.Fprint(w, `{"html_url":"https://example/comment"}`)
		}
	}))
	defer srv.Close()
	fg := Forge{Kind: "forgejo", APIBase: srv.URL}
	wr := Writer{Client: srv.Client(), Token: "tok"}
	ids, err := wr.LabelIDs(fg, "o", "r", []string{"n1", "bug"})
	if err != nil || fmt.Sprint(ids) != "[1 51]" || page != 2 {
		t.Fatalf("ids=%v page=%d err=%v", ids, page, err)
	}
	is, err := wr.CreateIssue(fg, "o", "r", "T", "B", nil)
	if err != nil || is.Number != 7 || is.HTMLURL != "https://example/7" || is.UpdatedAt.Format("2006-01-02T15:04:05Z") != "2026-01-02T03:04:05Z" {
		t.Fatalf("issue=%+v err=%v", is, err)
	}
	u, err := wr.Comment(fg, "o", "r", 7, "c")
	if err != nil || u != "https://example/comment" {
		t.Fatalf("url=%q err=%v", u, err)
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
