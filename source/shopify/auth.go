// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package shopify implements Hanzo for Shopify (/v1/source/shopify/*):
// App Bridge session JWT verification, managed-installation token exchange,
// HMAC-verified webhooks (including mandatory privacy compliance topics),
// Shopify Flow actions & triggers, web pixel activation, and outbound
// Shopify Admin GraphQL 2026-10 bidirectional sync.
package shopify

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/secrets"
)

const APIVersion = "2026-10"

var (
	ErrMissingToken = errors.New("shopify: missing session token")
	ErrBadToken     = errors.New("shopify: invalid session token")
	ErrBadHMAC      = errors.New("shopify: invalid webhook hmac")
	ErrBadShop      = errors.New("shopify: invalid shop domain")
)

var shopDomainRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$`)

// Config holds optional deployment overrides; empty fields read live from KMS/env.
type Config struct {
	ClientID         string
	ClientSecret     string
	ClientSecretPrev string
	AppID            string
	IAMAuthorizeURL  string
	IAMClientID      string
	PublicBaseURL    string
}

func (cfg Config) clientID(ctx context.Context) string {
	if cfg.ClientID != "" {
		return cfg.ClientID
	}
	if v := secrets.String(ctx, "orgs/hanzo/shopify/CLIENT_ID"); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("SHOPIFY_CLIENT_ID"))
}

func (cfg Config) clientSecrets(ctx context.Context) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		for _, existing := range out {
			if existing == s {
				return
			}
		}
		out = append(out, s)
	}
	add(cfg.ClientSecret)
	add(cfg.ClientSecretPrev)
	if v, err := secrets.Read(ctx, "orgs/hanzo/shopify/CLIENT_SECRET"); err == nil {
		add(v)
	}
	if v, err := secrets.Read(ctx, "orgs/hanzo/shopify/CLIENT_SECRET_PREV"); err == nil {
		add(v)
	}
	add(os.Getenv("SHOPIFY_CLIENT_SECRET"))
	add(os.Getenv("SHOPIFY_CLIENT_SECRET_PREV"))
	return out
}

func (cfg Config) publicBaseURL() string {
	if cfg.PublicBaseURL != "" {
		return strings.TrimRight(cfg.PublicBaseURL, "/")
	}
	if v := strings.TrimSpace(os.Getenv("HANZO_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.hanzo.ai"
}

// NormalizeShop validates and normalizes a Shopify store domain (e.g. "acme.myshopify.com").
func NormalizeShop(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimSuffix(s, "/")
	if !shopDomainRe.MatchString(s) || strings.Contains(s, "..") {
		return "", ErrBadShop
	}
	return s, nil
}

// SessionClaims holds the verified claims of a Shopify App Bridge session JWT.
type SessionClaims struct {
	Issuer   string `json:"iss"`
	Dest     string `json:"dest"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	Exp      int64  `json:"exp"`
	Nbf      int64  `json:"nbf"`
	Iat      int64  `json:"iat"`
	Jti      string `json:"jti"`
	Sid      string `json:"sid"`
	Shop     string `json:"-"`
}

// VerifySessionToken verifies a Shopify App Bridge session JWT (HS256) against
// clientSecrets and returns the verified shop domain.
func VerifySessionToken(rawToken, expectedClientID string, clientSecrets []string, now time.Time) (*SessionClaims, error) {
	rawToken = strings.TrimSpace(strings.TrimPrefix(rawToken, "Bearer "))
	if rawToken == "" || len(clientSecrets) == 0 {
		return nil, ErrMissingToken
	}
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, ErrBadToken
	}
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrBadToken
	}
	var hdr struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil || hdr.Alg != "HS256" {
		return nil, ErrBadToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrBadToken
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	verified := false
	for _, sec := range clientSecrets {
		if sec == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(sec))
		mac.Write(signingInput)
		if hmac.Equal(sig, mac.Sum(nil)) {
			verified = true
			break
		}
	}
	if !verified {
		return nil, ErrBadToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrBadToken
	}
	var claims SessionClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrBadToken
	}
	if expectedClientID != "" && claims.Audience != expectedClientID {
		return nil, ErrBadToken
	}
	unix := now.Unix()
	if claims.Exp <= 0 || unix > claims.Exp+15 {
		return nil, ErrBadToken
	}
	if claims.Nbf > 0 && unix+15 < claims.Nbf {
		return nil, ErrBadToken
	}

	destURL, err := url.Parse(claims.Dest)
	if err != nil || destURL.Scheme != "https" || destURL.Host == "" {
		return nil, ErrBadToken
	}
	shop, err := NormalizeShop(destURL.Host)
	if err != nil {
		return nil, ErrBadToken
	}
	if claims.Issuer != "" {
		issURL, err := url.Parse(claims.Issuer)
		if err != nil || !strings.EqualFold(issURL.Host, shop) {
			return nil, ErrBadToken
		}
	}
	claims.Shop = shop
	return &claims, nil
}

// SignSessionToken mints a Shopify App Bridge HS256 JWT (used by tests).
func SignSessionToken(claims SessionClaims, secret string) (string, error) {
	hdr, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	pay, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	in := base64.RawURLEncoding.EncodeToString(hdr) + "." + base64.RawURLEncoding.EncodeToString(pay)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(in))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return in + "." + sig, nil
}

// ComputeWebhookHMAC computes base64(HMAC-SHA256(secret, body)).
func ComputeWebhookHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookHMAC checks X-Shopify-Hmac-Sha256 over raw body against candidate secrets.
func VerifyWebhookHMAC(headerB64 string, body []byte, candidateSecrets ...string) bool {
	headerB64 = strings.TrimSpace(headerB64)
	if headerB64 == "" {
		return false
	}
	got, err := base64.StdEncoding.DecodeString(headerB64)
	if err != nil || len(got) != sha256.Size {
		return false
	}
	for _, sec := range candidateSecrets {
		if sec == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(sec))
		mac.Write(body)
		if hmac.Equal(got, mac.Sum(nil)) {
			return true
		}
	}
	return false
}

// SignLinkState mints an HMAC-signed OAuth state token binding (shop, exp).
func SignLinkState(shop, secret string, now time.Time) (string, error) {
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	exp := now.Add(15 * time.Minute).Unix()
	msg := fmt.Sprintf("%s|%d|%s", shop, exp, hex.EncodeToString(nonce[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(msg + "|" + sig)), nil
}

// VerifyLinkState verifies a state token minted by SignLinkState and returns the shop domain.
func VerifyLinkState(state string, secrets []string, now time.Time) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(state))
	if err != nil {
		return "", ErrBadToken
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", ErrBadToken
	}
	shop, expStr, nonce, sigHex := parts[0], parts[1], parts[2], parts[3]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || now.Unix() > exp {
		return "", ErrBadToken
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", ErrBadToken
	}
	msg := []byte(shop + "|" + expStr + "|" + nonce)
	ok := false
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		mac := hmac.New(sha256.New, []byte(sec))
		mac.Write(msg)
		if hmac.Equal(sig, mac.Sum(nil)) {
			ok = true
			break
		}
	}
	if !ok {
		return "", ErrBadToken
	}
	return NormalizeShop(shop)
}
