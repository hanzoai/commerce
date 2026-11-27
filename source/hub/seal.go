// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hanzoai/commerce/secrets"
)

// SourceCredentials holds per-store secret material sealed inside StoreSource.SealedCredential.
type SourceCredentials struct {
	// Shopify offline Admin API token.
	AccessToken string `json:"accessToken,omitempty"`
	// WooCommerce per-connection HMAC secret (64 hex chars).
	HMACSecret string `json:"hmacSecret,omitempty"`
	// Optional WooCommerce REST v3 credentials for direct push.
	ConsumerKey    string `json:"consumerKey,omitempty"`
	ConsumerSecret string `json:"consumerSecret,omitempty"`
	// Optional per-source webhook secret override (custom Shopify app).
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

func deriveSealKey(ctx context.Context, orgId, sourceId string) []byte {
	var master string
	if v, err := secrets.Read(ctx, "orgs/hanzo/source/SEAL_KEY"); err == nil && v != "" {
		master = v
	}
	if master == "" {
		master = strings.TrimSpace(os.Getenv("COMMERCE_KMS_MASTER_KEY"))
	}
	if master == "" {
		master = strings.TrimSpace(os.Getenv("COMMERCE_SECRET"))
	}
	if master == "" {
		master = "commerce-dev-source-seal-key"
	}
	// HKDF-Extract + Expand (RFC 5869) using HMAC-SHA256 bound to orgId + sourceId.
	extract := hmac.New(sha256.New, []byte("hanzo-commerce-source-seal-v1"))
	extract.Write([]byte(master))
	prk := extract.Sum(nil)

	expand := hmac.New(sha256.New, prk)
	expand.Write([]byte(orgId + "\x00" + sourceId + "\x01"))
	return expand.Sum(nil)
}

// SealCredentials encrypts creds under an org+source-bound AES-256-GCM key.
func SealCredentials(ctx context.Context, orgId, sourceId string, creds SourceCredentials) (string, error) {
	plain, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	key := deriveSealKey(ctx, orgId, sourceId)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	aad := []byte(orgId + "\x00" + sourceId)
	ct := gcm.Seal(nonce, nonce, plain, aad)
	return "v1:" + base64.RawStdEncoding.EncodeToString(ct), nil
}

// OpenCredentials decrypts StoreSource.SealedCredential. Empty string yields zero credentials.
func OpenCredentials(ctx context.Context, orgId, sourceId, sealed string) (SourceCredentials, error) {
	if sealed == "" {
		return SourceCredentials{}, nil
	}
	if !strings.HasPrefix(sealed, "v1:") {
		return SourceCredentials{}, errors.New("source: unrecognized sealed credential format")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(sealed, "v1:"))
	if err != nil {
		return SourceCredentials{}, fmt.Errorf("source: invalid sealed credential encoding: %w", err)
	}
	key := deriveSealKey(ctx, orgId, sourceId)
	block, err := aes.NewCipher(key)
	if err != nil {
		return SourceCredentials{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SourceCredentials{}, err
	}
	if len(raw) < gcm.NonceSize() {
		return SourceCredentials{}, errors.New("source: truncated sealed credential")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	aad := []byte(orgId + "\x00" + sourceId)
	plain, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return SourceCredentials{}, errors.New("source: failed to unseal credential")
	}
	var out SourceCredentials
	if err := json.Unmarshal(plain, &out); err != nil {
		return SourceCredentials{}, err
	}
	return out, nil
}
