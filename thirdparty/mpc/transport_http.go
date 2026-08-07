package mpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// httpTransport is the wire this rail has always used: JSON over HTTP to
// mpcd's REST surface.
//
// It plays two roles, and they are separate on purpose. As a Transport it
// carries keygen and health for a deployment that selected no other wire. As
// itself it also carries the TRANSACTIONAL calls — create, approve, refund,
// query — which are HTTP no matter which Transport was selected, because mpcd's
// ZAP surface registers no opcode for any of them. The processor therefore
// holds one of these unconditionally, and the fact that it may ALSO be the
// selected Transport is why there is only ever one implementation of "post JSON
// at mpcd" to keep correct.
type httpTransport struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// Keygen posts to mpcd's /keygen and returns the response body untouched.
//
// The wallet id is deliberately LEFT TO THE NODE on this wire: mpcd mints a
// fresh one whenever the request omits it, and a client-side id replayed into
// /keygen would re-key an existing wallet — silently moving an address that
// funds may already be in flight to. Letting the node mint makes that
// impossible by construction rather than by our care.
func (t *httpTransport) Keygen(ctx context.Context, orgID string) ([]byte, error) {
	var raw json.RawMessage
	if err := t.doJSON(ctx, http.MethodPost, t.endpoint+"/keygen",
		map[string]string{"org_id": orgID}, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Health probes mpcd's /healthz.
func (t *httpTransport) Health(ctx context.Context) error {
	resp, err := t.doRequest(ctx, http.MethodGet, t.endpoint+"/healthz", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mpc: GET %s/healthz returned %d", t.endpoint, resp.StatusCode)
	}
	return nil
}

func (t *httpTransport) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("mpc: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("mpc: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mpc: request to %s failed: %w", url, err)
	}
	return resp, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (t *httpTransport) doJSON(ctx context.Context, method, url string, reqBody, respBody interface{}) error {
	resp, err := t.doRequest(ctx, method, url, reqBody)
	if err != nil {
		return err
	}
	body, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("mpc: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mpc: %s %s returned %d: %s", method, url, resp.StatusCode, string(body))
	}
	if respBody != nil {
		if err := json.Unmarshal(body, respBody); err != nil {
			return fmt.Errorf("mpc: decode response: %w", err)
		}
	}
	return nil
}

// Ensure httpTransport satisfies the seam.
var _ Transport = (*httpTransport)(nil)
