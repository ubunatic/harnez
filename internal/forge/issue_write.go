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
	// Source names the environment variable the token came from; empty for the
	// stored write token. Only the name is ever reported, never the value.
	Source string
}

// scopeHint adds the missing-scope hint to a 401 or 403 from an environment token.
func (w Writer) scopeHint(status int, err error) error {
	if w.Source == "" || (status != http.StatusUnauthorized && status != http.StatusForbidden) {
		return err
	}
	return fmt.Errorf("%w; the %s token was rejected: give it the issue write scope (write:issue) or run harnez forge token create --write", err, w.Source)
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
		return fmt.Errorf("forge write: no HTTP client")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("forge write request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		detail := ""
		{
			var msg struct {
				Message string `json:"message"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&msg) == nil && safeWriteDetail(msg.Message, w.Token, data) {
				detail = strings.Join(strings.Fields(msg.Message), " ")
				if runes := []rune(detail); len(runes) > 200 {
					detail = string(runes[:200])
				}
			}
		}
		if detail == "" {
			return w.scopeHint(resp.StatusCode, fmt.Errorf("forge API request failed: HTTP %d", resp.StatusCode))
		}
		return w.scopeHint(resp.StatusCode, &APIError{Status: resp.StatusCode, Transient: resp.StatusCode >= 500, Detail: detail})
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

func (w Writer) Comment(fg Forge, owner, repo string, n int, body string) (string, error) {
	var result struct {
		HTMLURL string `json:"html_url"`
	}
	err := w.request(fg, http.MethodPost, issueWriteBase(fg, owner, repo)+"/issues/"+strconv.Itoa(n)+"/comments", struct {
		Body string `json:"body"`
	}{body}, &result)
	return result.HTMLURL, err
}

func (w Writer) LabelIDs(fg Forge, owner, repo string, names []string) ([]int64, error) {
	var labels []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	seen := make(map[int64]bool)
	for page := 1; ; page++ {
		var batch []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		endpoint := issueWriteBase(fg, owner, repo) + "/labels?limit=50&page=" + strconv.Itoa(page)
		if err := w.request(fg, http.MethodGet, endpoint, nil, &batch); err != nil {
			return nil, err
		}
		added := 0
		for _, label := range batch {
			if !seen[label.ID] {
				seen[label.ID] = true
				labels = append(labels, label)
				added++
			}
		}
		if added == 0 {
			break
		}
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
