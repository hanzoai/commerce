// Package kms provides a thin HTTP client for Hanzo KMS secret management.
package kms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StatusError is returned when KMS responds with a non-success HTTP status. It
// carries the status code so callers branch on it with errors.As instead of
// string-matching the message (e.g. a 404/400 = secret absent).
type StatusError struct {
	Op   string // operation, e.g. "get secret"
	Code int    // HTTP status code
	Body string // response body (may carry KMS error detail)
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("kms %s failed (status %d): %s", e.Op, e.Code, e.Body)
}

// Config holds KMS client configuration.
type Config struct {
	Enabled      bool
	URL          string
	ClientID     string
	ClientSecret string
	ProjectID    string
	// Org scopes every secret path and the JWT that reads it. Defaults to
	// "hanzo" when empty.
	Org         string
	Environment string
}

// Client is a thin HTTP client wrapping the KMS REST API.
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	projectID    string
	org          string
	environment  string

	accessToken string
	tokenExpiry time.Time
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewClient creates a new KMS client.
func NewClient(cfg *Config) *Client {
	org := cfg.Org
	if org == "" {
		org = "hanzo"
	}
	return &Client{
		baseURL:      strings.TrimRight(cfg.URL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		projectID:    cfg.ProjectID,
		org:          org,
		environment:  cfg.Environment,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// joinSecretRef renders a (path, name) pair as the {rest...} the server splits
// at its LAST slash. Each segment is escaped alone so the separators survive.
func joinSecretRef(secretPath, secretName string) string {
	segs := append(strings.Split(strings.Trim(secretPath, "/"), "/"), secretName)
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg == "" {
			continue
		}
		out = append(out, url.PathEscape(seg))
	}
	return strings.Join(out, "/")
}

// authResponse is the response from the KMS auth endpoint.
type authResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"` // seconds
}

// authenticate obtains or refreshes the access token via Universal Auth.
func (c *Client) authenticate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Skip if token is still valid (with 30s buffer)
	if c.accessToken != "" && time.Now().Add(30*time.Second).Before(c.tokenExpiry) {
		return nil
	}

	body, err := json.Marshal(map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.clientSecret,
	})
	if err != nil {
		return fmt.Errorf("kms auth encode error: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/kms/auth/login", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("kms auth request build error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kms auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return &StatusError{Op: "auth", Code: resp.StatusCode, Body: string(respBody)}
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("kms auth decode error: %w", err)
	}

	c.accessToken = authResp.AccessToken
	if authResp.ExpiresIn > 0 {
		c.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	} else {
		// Default to 5 minutes if not specified
		c.tokenExpiry = time.Now().Add(5 * time.Minute)
	}

	return nil
}

// secretResponse is the response from the KMS secrets endpoint.
type secretResponse struct {
	Secret struct {
		Value string `json:"value"`
	} `json:"secret"`
}

// GetSecretRaw fetches a secret from KMS by path and name.
func (c *Client) GetSecretRaw(secretPath, secretName string) (string, error) {
	if err := c.authenticate(); err != nil {
		return "", err
	}

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	// The server takes {rest...} and splits it at the LAST slash into
	// (path, name), so secretPath and secretName are joined here and each
	// segment is escaped on its own — escaping the joined string would encode
	// the separators and the server would read one long name.
	reqURL := fmt.Sprintf(
		"%s/v1/kms/orgs/%s/secrets/%s?env=%s",
		c.baseURL,
		url.PathEscape(c.org),
		joinSecretRef(secretPath, secretName),
		url.QueryEscape(c.environment),
	)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("kms request build error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("kms request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", &StatusError{Op: "get secret", Code: resp.StatusCode, Body: string(respBody)}
	}

	var secretResp secretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
		return "", fmt.Errorf("kms secret decode error: %w", err)
	}

	return secretResp.Secret.Value, nil
}

// SetSecret writes a secret to KMS at the given path.
func (c *Client) SetSecret(secretPath, secretName, secretValue string) error {
	if err := c.authenticate(); err != nil {
		return err
	}

	c.mu.RLock()
	token := c.accessToken
	c.mu.RUnlock()

	// One upsert endpoint; path, name and env all travel in the body.
	payload, err := json.Marshal(map[string]string{
		"path":  secretPath,
		"name":  secretName,
		"env":   c.environment,
		"value": secretValue,
	})
	if err != nil {
		return fmt.Errorf("kms set secret marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/kms/orgs/"+url.PathEscape(c.org)+"/secrets", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("kms set secret request build error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kms set secret request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return &StatusError{Op: "set secret", Code: resp.StatusCode, Body: string(respBody)}
	}

	return nil
}
