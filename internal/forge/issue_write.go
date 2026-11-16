package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Writer sends authenticated Forgejo issue writes.
type Writer struct {
	Client *http.Client
	Token  string
}

// EditFields specifies the issue fields to update.
type EditFields struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
	State *string `json:"state,omitempty"`
}

func issueWriteBase(fg Forge, owner, repo string) string {
	return strings.TrimSuffix(fg.APIBase, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

func (w Writer) request(fg Forge, method, endpoint string, payload, result any) error {
	if fg.Kind != "forgejo" {
		return fmt.Errorf("forge writes support Forgejo only; GitHub is ticket 1209")
	}
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("forge write: invalid request")
	}
	req.Header.Set("Authorization", "token "+w.Token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("forge write request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		detail := ""
		if resp.StatusCode == http.StatusPreconditionFailed {
			var msg struct {
				Message string `json:"message"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&msg) == nil && safeWriteDetail(msg.Message, w.Token, data) {
				detail = msg.Message
			}
		}
		return &APIError{Status: resp.StatusCode, Transient: resp.StatusCode >= 500, Detail: detail}
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("forge write: invalid JSON response")
		}
	}
	return nil
}

func safeWriteDetail(detail, token string, data []byte) bool {
	if token != "" && strings.Contains(detail, token) {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) == nil {
		for _, name := range []string{"body", "title"} {
			var value string
			if json.Unmarshal(fields[name], &value) == nil && value != "" && strings.Contains(detail, value) {
				return false
			}
		}
	}
	return true
}

func (w Writer) CreateIssue(fg Forge, owner, repo, title, body string, labelIDs []int64) (RemoteIssue, error) {
	var result RemoteIssue
	payload := struct {
		Title  string  `json:"title"`
		Body   string  `json:"body"`
		Labels []int64 `json:"labels,omitempty"`
	}{title, body, labelIDs}
	err := w.request(fg, http.MethodPost, issueWriteBase(fg, owner, repo)+"/issues", payload, &result)
	return result, err
}

func (w Writer) EditIssue(fg Forge, owner, repo string, n int, fields EditFields) (RemoteIssue, error) {
	var result RemoteIssue
	err := w.request(fg, http.MethodPatch, issueWriteBase(fg, owner, repo)+"/issues/"+strconv.Itoa(n), fields, &result)
	return result, err
}

func (w Writer) Comment(fg Forge, owner, repo string, n int, body string) error {
	var result json.RawMessage
	return w.request(fg, http.MethodPost, issueWriteBase(fg, owner, repo)+"/issues/"+strconv.Itoa(n)+"/comments", struct {
		Body string `json:"body"`
	}{body}, &result)
}

func (w Writer) LabelIDs(fg Forge, owner, repo string, names []string) ([]int64, error) {
	var labels []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := w.request(fg, http.MethodGet, issueWriteBase(fg, owner, repo)+"/labels", nil, &labels); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(names))
	existing := make([]string, 0, len(labels))
	for _, label := range labels {
		existing = append(existing, label.Name)
	}
	for _, name := range names {
		found := false
		for _, label := range labels {
			if label.Name == name {
				ids = append(ids, label.ID)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown label %q; existing labels: %s", name, strings.Join(existing, ", "))
		}
	}
	return ids, nil
}
