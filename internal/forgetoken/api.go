package forgetoken

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Login is the basic-auth identity for the token endpoints. Forgejo's /users/{user}/tokens
// accepts only a password, never a token; OTP goes in X-Forgejo-OTP when 2FA is on.
// Password is never logged or put into an error.
type Login struct {
	User     string
	Password string
	OTP      string
}

// Created is the token Forgejo returns once on creation; Secret is its sha1 field.
type Created struct {
	ID     int64
	Name   string
	Secret string
}

// Client talks to one Forgejo API base, e.g. https://codeberg.org/api/v1.
type Client struct {
	HTTP    *http.Client
	APIBase string
}

// NewHTTPClient returns a client that never follows redirects, so the basic-auth header cannot
// travel to another URL.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func (c Client) do(method, path string, l Login, body any) ([]byte, int, error) {
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.APIBase, "/")+path, rd)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(l.User, l.Password)
	if l.OTP != "" {
		req.Header.Set("X-Forgejo-OTP", l.OTP)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// url.Error carries the URL, which holds no credentials (basic auth is a header).
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return data, resp.StatusCode, err
}

func apiError(op string, status int, body []byte) error {
	var msg struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &msg)
	detail := strings.TrimSpace(msg.Message)
	hint := ""
	switch status {
	case http.StatusUnauthorized:
		hint = " (wrong user or password, or a missing/expired 2FA code)"
	case http.StatusForbidden:
		hint = " (forbidden; a token cannot create tokens, only the password can)"
	}
	if detail == "" {
		return fmt.Errorf("%s: HTTP %d%s", op, status, hint)
	}
	return fmt.Errorf("%s: HTTP %d: %s%s", op, status, detail, hint)
}

// Create makes a new access token named name with scopes.
func (c Client) Create(l Login, name string, scopes []string) (Created, error) {
	body := map[string]any{"name": name, "scopes": scopes}
	data, status, err := c.do(http.MethodPost, "/users/"+url.PathEscape(l.User)+"/tokens", l, body)
	if err != nil {
		return Created{}, fmt.Errorf("create token: %w", err)
	}
	if status != http.StatusCreated {
		return Created{}, apiError("create token", status, data)
	}
	var out struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		SHA1 string `json:"sha1"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return Created{}, fmt.Errorf("create token: decode response: %w", err)
	}
	if out.SHA1 == "" {
		return Created{}, fmt.Errorf("create token: response holds no token")
	}
	return Created{ID: out.ID, Name: out.Name, Secret: out.SHA1}, nil
}

// CheckRead probes a read with the token itself: GET /repos/{owner}/{repo}/issues?limit=1.
// Forgejo lists a token's scopes only with the password, so this is the check a token allows.
func (c Client) CheckRead(token, ownerRepo string) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(c.APIBase, "/")+"/repos/"+ownerRepo+"/issues?limit=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("check read: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return apiError("check read of "+ownerRepo, resp.StatusCode, data)
	}
	return nil
}

// Delete revokes the token with id on the forge; 404 counts as already gone.
func (c Client) Delete(l Login, id int64) error {
	return c.DeleteRef(l, strconv.FormatInt(id, 10))
}

// DeleteRef revokes a token by id or, when Forgejo finds no such id, by name.
func (c Client) DeleteRef(l Login, ref string) error {
	path := "/users/" + url.PathEscape(l.User) + "/tokens/" + url.PathEscape(ref)
	data, status, err := c.do(http.MethodDelete, path, l, nil)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	if status == http.StatusNoContent || status == http.StatusNotFound {
		return nil
	}
	return apiError("revoke token", status, data)
}
