// Package iammiddleware provides Gin middleware for validating Hanzo IAM (hanzo.id) JWT tokens.
// It uses the existing auth.IAMClient for JWKS-based token validation and sets
// IAM claims in the Gin context for downstream handlers.
package iammiddleware

import (
	"context"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
)

var (
	iamClient *auth.IAMClient
	mu        sync.RWMutex
)

// Init initializes the IAM middleware with the given configuration.
// Must be called before IAMTokenRequired() middleware is used.
// Safe to call multiple times; last call wins.
func Init(cfg *auth.IAMConfig) error {
	mu.Lock()
	defer mu.Unlock()

	client, err := auth.NewIAMClient(cfg)
	if err != nil {
		return err
	}
	iamClient = client
	return nil
}

// IAMTokenRequired validates hanzo.id JWT tokens via JWKS.
// If a valid IAM token is found, it sets iam_* context keys and calls c.Next().
// If no Bearer token is present or validation fails, it falls through to the
// next middleware (legacy org-token auth) without aborting.
func IAMTokenRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		mu.RLock()
		client := iamClient
		mu.RUnlock()

		if client == nil {
			c.Next()
			return
		}

		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		claims, err := client.ValidateToken(context.Background(), token)
		if err != nil {
			// Not a valid IAM token — fall through to legacy middleware
			c.Next()
			return
		}

		// Valid IAM token — set context values
		c.Set("iam_claims", claims)
		c.Set("iam_user_id", claims.Subject)
		c.Set("iam_email", claims.Email)
		c.Set("iam_org", claims.Owner)
		c.Set("iam_roles", claims.Roles)
		c.Set("iam_authenticated", true)
		c.Next()
	}
}

// IsIAMAuthenticated checks whether the current request was authenticated via IAM.
func IsIAMAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("iam_authenticated")
	return exists
}

// GetIAMClaims returns the IAM claims from context, or nil if not IAM-authenticated.
func GetIAMClaims(c *gin.Context) *auth.IAMClaims {
	val, exists := c.Get("iam_claims")
	if !exists {
		return nil
	}
	claims, ok := val.(*auth.IAMClaims)
	if !ok {
		return nil
	}
	return claims
}
