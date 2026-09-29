package decide

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if s.Backends[s.DefaultBackend] == nil {
		t.Fatalf("default backend %q missing", s.DefaultBackend)
	}
}

// Every protocol in the schema enum has a client, and vice versa.
func TestProtocolsMatchSchema(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/decide.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Backends struct {
				AdditionalProperties struct {
					Properties struct {
						Protocol struct {
							Enum []string `json:"enum"`
						} `json:"protocol"`
					} `json:"properties"`
				} `json:"additionalProperties"`
			} `json:"backends"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	enum := schema.Properties.Backends.AdditionalProperties.Properties.Protocol.Enum
	var have []string
	for p := range protocols {
		have = append(have, p)
	}
	slices.Sort(enum)
	slices.Sort(have)
	if !slices.Equal(enum, have) {
		t.Fatalf("schema protocols %v, code protocols %v", enum, have)
	}
}

func TestParseSpecRejects(t *testing.T) {
	for name, yml := range map[string]string{
		"unknown field":    "default_backend: a\nbackends:\n  a: {protocol: systemone, base_url: http://x, path: /p, timeout: 5s, bogus: 1}\n",
		"unknown protocol": "default_backend: a\nbackends:\n  a: {protocol: nope, base_url: http://x, path: /p, timeout: 5s}\n",
		"missing default":  "default_backend: b\nbackends:\n  a: {protocol: systemone, base_url: http://x, path: /p, timeout: 5s}\n",
		"no timeout":       "default_backend: a\nbackends:\n  a: {protocol: systemone, base_url: http://x, path: /p}\n",
		"bad timeout":      "default_backend: a\nbackends:\n  a: {protocol: systemone, base_url: http://x, path: /p, timeout: soon}\n",
	} {
		if _, err := parseSpec([]byte(yml)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func testSpec(url string) *Spec {
	s, err := parseSpec([]byte("default_backend: t\nbackends:\n  t:\n    protocol: systemone\n    base_url: " + url +
		"\n    path: /v1/systemone\n    timeout: 5s\n    model: jev-latest\n    api_key_env: TEST_KEY\n  local:\n    protocol: systemone\n    base_url: " + url + "\n    path: /v1/systemone\n    timeout: 5s\n"))
	if err != nil {
		panic(err)
	}
	return s
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

var noulReq = &Request{
	State: map[string]string{"command": "rm -rf /"},
	Questions: map[string]Question{
		"blocked": {Type: TypeNoul, Instructions: "Needs confirmation?"},
	},
}

func TestSystemOneRoundTrip(t *testing.T) {
	var got Request
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"model":"jev-1.13.0","answers":{"blocked":{"type":"noul","noul":0.93}},"usage":{"input_tokens":3,"output_tokens":1}}`))
	}))
	defer srv.Close()

	b, err := testSpec(srv.URL).Open("", env(map[string]string{"TEST_KEY": "k1"}))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := b.Decide(context.Background(), noulReq)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer k1" || got.Model != "jev-latest" {
		t.Fatalf("auth %q model %q", auth, got.Model)
	}
	a := resp.Answers["blocked"]
	if lv, ok := a.Level(); !ok || lv != 0.93 || a.Value() != "0.93" {
		t.Fatalf("answer %+v", a)
	}
}

// A backend without api_key_env (a local model) sends no auth header.
func TestLocalBackendNoKey(t *testing.T) {
	var auth = "unset"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"answers":{"blocked":{"type":"noul","noul":0.1}}}`))
	}))
	defer srv.Close()
	b, err := testSpec(srv.URL).Open("local", env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Decide(context.Background(), noulReq); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Fatalf("auth %q, want none", auth)
	}
}

func TestSystemOneErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer bad" {
			w.WriteHeader(401)
			w.Write([]byte(`{"detail":{"error_type":"authentication_error","message":"check your API key"}}`))
			return
		}
		if r.Header.Get("Authorization") == "Bearer wrongtype" {
			w.Write([]byte(`{"answers":{"blocked":{"type":"choice","choice":"x"}}}`))
			return
		}
		w.Write([]byte(`{"answers":{}}`))
	}))
	defer srv.Close()
	s := testSpec(srv.URL)

	if _, err := s.Open("", env(nil)); err == nil || !strings.Contains(err.Error(), "$TEST_KEY") {
		t.Fatalf("missing key: %v", err)
	}
	b, _ := s.Open("", env(map[string]string{"TEST_KEY": "bad"}))
	if _, err := b.Decide(context.Background(), noulReq); err == nil || !strings.Contains(err.Error(), "check your API key") {
		t.Fatalf("401: %v", err)
	}
	b, _ = s.Open("", env(map[string]string{"TEST_KEY": "ok"}))
	if _, err := b.Decide(context.Background(), noulReq); err == nil || !strings.Contains(err.Error(), "no answer") {
		t.Fatalf("missing answer: %v", err)
	}
	b, _ = s.Open("", env(map[string]string{"TEST_KEY": "wrongtype"}))
	if _, err := b.Decide(context.Background(), noulReq); err == nil || !strings.Contains(err.Error(), "want \"noul\"") {
		t.Fatalf("wrong answer type: %v", err)
	}
	if _, err := s.Open("nope", nil); err == nil {
		t.Fatal("unknown backend: want error")
	}
}

func TestValidate(t *testing.T) {
	bad := []*Request{
		{},
		{Questions: map[string]Question{"q": {Type: "maybe", Instructions: "x"}}},
		{Questions: map[string]Question{"q": {Type: TypeNoul}}},
	}
	for i, r := range bad {
		if r.Validate() == nil {
			t.Errorf("case %d: want error", i)
		}
	}
}

func TestAnswerLevel(t *testing.T) {
	two := 2.0
	c := Answer{Type: TypeChoice, Choice: "b", Probabilities: map[string]float64{"a": 0.1, "b": 0.9}}
	if lv, _ := c.Level(); lv != 0.9 || c.Value() != "b" {
		t.Fatalf("choice %v %s", lv, c.Value())
	}
	s := Answer{Type: TypeScore, Score: &two}
	if lv, _ := s.Level(); lv != 2 || s.Value() != "2" {
		t.Fatalf("score %v %s", lv, s.Value())
	}
}
