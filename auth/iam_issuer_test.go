// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
)

// While the platform's issuer moves from hanzo.id to hanzo.ai, a resource server whose
// IAM_ACCEPTED_ISSUERS names both trusts both, and never an organization's own issuer,
// which sits one path segment under hanzo.ai.
func TestValidateTokenIssuerAllowlistHoldsBothPlatformIssuers(t *testing.T) {
	c, err := NewIAMClient(&IAMConfig{
		Issuer:          "http://iam.hanzo.svc.cluster.local",
		ClientID:        "hanzo-commerce",
		AcceptedIssuers: []string{"https://hanzo.id", "https://iam.hanzo.ai", "https://hanzo.ai"},
	})
	if err != nil {
		t.Fatal(err)
	}
	token := func(iss string) string {
		claims := &IAMClaims{StandardClaims: jwt.StandardClaims{Issuer: iss, ExpiresAt: time.Now().Add(time.Hour).Unix()}}
		s, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	// The issuer is the first check after parsing: what follows (audience, keys) is another
	// error, so the issuer's verdict is whether the error is ErrInvalidIssuer.
	for _, iss := range []string{"https://hanzo.id", "https://hanzo.ai"} {
		if _, err := c.ValidateToken(context.Background(), token(iss)); errors.Is(err, ErrInvalidIssuer) {
			t.Errorf("iss %s refused as an issuer: %v", iss, err)
		}
	}
	for _, iss := range []string{"https://hanzo.ai/acme", "https://hanzo.ai.evil.test", "https://evil.example"} {
		if _, err := c.ValidateToken(context.Background(), token(iss)); !errors.Is(err, ErrInvalidIssuer) {
			t.Errorf("iss %s was not refused as an issuer: %v", iss, err)
		}
	}
}
