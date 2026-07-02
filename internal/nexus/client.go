// Package nexus implements a thin REST client for Nexus Repository 3.76,
// scoped to the endpoints nexus-cli needs: repositories, security privileges,
// and security roles (PRD section 20).
//
// The admin password is never logged. HTTP errors are decoded into typed
// APIError values so callers can branch on status (PRD section 18).
package nexus

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Nexus REST API.
type Client struct {
	baseURL string // e.g. http://nexus.example.com  (no trailing slash)
	v1      string // baseURL + "/service/rest/v1"
	username string
	password string
	http    *http.Client
}

// New constructs a Client. timeoutSeconds <=0 falls back to 30s.
func New(baseURL, username, password string, timeoutSeconds int, insecureSkipTLS bool) *Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipTLS}, //nolint:gosec // configurable per PRD 9.1
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		v1:       strings.TrimRight(baseURL, "/") + "/service/rest/v1",
		username: username,
		password: password,
		http: &http.Client{
			Timeout:   time.Duration(timeoutSeconds) * time.Second,
			Transport: tr,
		},
	}
}

// BaseURL returns the configured Nexus base URL (without trailing slash).
func (c *Client) BaseURL() string { return c.baseURL }

// APIError carries the HTTP status and a best-effort body excerpt from Nexus.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("nexus api error: status=%d body=%s", e.Status, e.Body)
}

// IsNotFound reports whether the error is a 404.
func IsNotFound(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.Status == http.StatusNotFound
}

// do performs an authenticated request. The body argument may be nil.
// It returns the raw response body and a non-nil error on non-2xx responses.
// Passwords and Authorization headers are never included in errors or logs.
func (c *Client) do(method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, c.v1+path, reader)
	if err != nil {
		return nil, fmt.Errorf("build request %s %s: %w", method, path, err)
	}
	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MiB cap
	if readErr != nil {
		return nil, fmt.Errorf("read response %s %s: %w", method, path, readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, &APIError{Status: resp.StatusCode, Body: truncate(string(data), 500)}
	}
	return data, nil
}

func (c *Client) get(path string, out any) error {
	data, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response %s: %w (body=%s)", path, err, truncate(string(data), 200))
	}
	return nil
}

func (c *Client) post(path string, body, out any) error {
	data, err := c.do(http.MethodPost, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func (c *Client) put(path string, body any) error {
	_, err := c.do(http.MethodPut, path, body)
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
