package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// systemOne speaks the TypeSafe System One protocol: POST a Request as JSON,
// get a Response back.
type systemOne struct {
	url    string
	model  string
	key    string
	client *http.Client
}

func newSystemOne(b BackendSpec, getenv func(string) string) (Backend, error) {
	base := b.BaseURL
	if b.BaseURLEnv != "" {
		if v := getenv(b.BaseURLEnv); v != "" {
			base = v
		}
	}
	var key string
	if b.APIKeyEnv != "" {
		key = getenv(b.APIKeyEnv)
		if key == "" {
			return nil, fmt.Errorf("backend %q needs an API key in $%s", b.Name, b.APIKeyEnv)
		}
	}
	timeout, _ := time.ParseDuration(b.Timeout) // checked by parseSpec
	return &systemOne{
		url:    strings.TrimRight(base, "/") + b.Path,
		model:  b.Model,
		key:    key,
		client: &http.Client{Timeout: timeout},
	}, nil
}

func (s *systemOne) Decide(ctx context.Context, req *Request) (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	body := *req
	if body.Model == "" {
		body.Model = s.model
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if s.key != "" {
		hreq.Header.Set("Authorization", "Bearer "+s.key)
	}
	resp, err := s.client.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d: %s", s.url, resp.StatusCode, errorMessage(raw))
	}
	var out Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: bad response: %w", s.url, err)
	}
	for _, id := range req.QuestionIDs() {
		a, ok := out.Answers[id]
		if !ok {
			return nil, fmt.Errorf("%s: no answer for question %q", s.url, id)
		}
		if want := req.Questions[id].Type; a.Type != want {
			return nil, fmt.Errorf("%s: question %q: got a %q answer, want %q", s.url, id, a.Type, want)
		}
	}
	return &out, nil
}

// errorMessage extracts detail.message from an error body, else returns
// the body trimmed.
func errorMessage(raw []byte) string {
	var e struct {
		Detail struct {
			Message string `json:"message"`
		} `json:"detail"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Detail.Message != "" {
		return e.Detail.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
