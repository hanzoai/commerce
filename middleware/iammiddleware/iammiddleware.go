// Package iammiddleware provides Gin middleware for validating Hanzo IAM (hanzo.id) JWT tokens.
// It uses the existing auth.IAMClient for JWKS-based token validation and sets
// IAM claims in the Gin context for downstream handlers.
package iammiddleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/bit"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

var (
	iamClient   *auth.IAMClient
	initAttempt bool // true once Init has been called, regardless of success
	mu          sync.RWMutex
)

// Init initializes the IAM middleware with the given configuration.
// Must be called before IAMTokenRequired() middleware is used.
// Safe to call multiple times; last call wins.
func Init(cfg *auth.IAMConfig) error {
	mu.Lock()
	defer mu.Unlock()

	initAttempt = true
	client, err := auth.NewIAMClient(cfg)
	if err != nil {
		return err
	}
	iamClient = client
	return nil
}

// IAMTokenRequired validates hanzo.id JWT tokens via JWKS.
// If a valid IAM token is found, it resolves the org from the token's "owner"
// claim and sets both IAM context keys and the standard "organization" +
// "permissions" keys that downstream handlers expect.
//
// Auth guard behavior:
//   - IAM enabled but client initialization failed: 503 Service Unavailable
//   - Bearer token present but invalid: 401 Unauthorized (no fallthrough)
//   - No Bearer token present: fall through to legacy org-token auth
func IAMTokenRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		mu.RLock()
		client := iamClient
		wasInitAttempted := initAttempt
		mu.RUnlock()

		// If Init was called but client is nil, initialization failed.
		// The IAM subsystem is expected to be available but is not -- return 503.
		if client == nil {
			if wasInitAttempted {
				jsonhttp.Fail(c, http.StatusServiceUnavailable,
					"IAM authentication service is unavailable", nil)
				return
			}
			// Init was never called -- IAM is not configured. Fall through
			// to legacy auth.
			c.Next()
			return
		}

		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			// No Bearer token present -- fall through to legacy auth.
			c.Next()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")

		// If token matches COMMERCE_SERVICE_TOKEN, skip IAM validation and
		// fall through to legacy auth so service-to-service calls work.
		if svcToken := os.Getenv("COMMERCE_SERVICE_TOKEN"); svcToken != "" && token == svcToken {
			c.Next()
			return
		}

		claims, err := client.ValidateToken(context.Background(), token)
		if err != nil {
			// A Bearer token was presented but failed IAM validation.
			// Return 401 immediately -- do not fall through to legacy auth,
			// because a Bearer JWT should only be validated by IAM.
			log.Warn("IAM token validation failed: %v", err)
			jsonhttp.Fail(c, http.StatusUnauthorized,
				"Invalid or expired authentication token", err)
			return
		}

		// Valid IAM token -- set context values
		c.Set("iam_claims", claims)
		c.Set("iam_user_id", claims.Subject)
		c.Set("iam_email", claims.Email)
		c.Set("iam_org", claims.Owner)
		c.Set("iam_roles", claims.Roles)
		c.Set("iam_authenticated", true)

		// Resolve organization from IAM "owner" claim so downstream handlers
		// get proper tenant scoping via middleware.GetOrganization(c).
		if claims.Owner != "" {
			ctx := c.Request.Context()
			db := datastore.New(ctx)
			org := organization.New(db)

			// Look up org by name (the IAM "owner" field is the org name)
			ok, lookupErr := org.Query().Filter("Name=", claims.Owner).Get()
			if lookupErr != nil || !ok {
				log.Warn("IAM token owner '%s' does not match any organization: %v", claims.Owner, lookupErr)
				// Do not abort -- the user is authenticated but the org
				// lookup failed. Downstream handlers will see IAM claims
				// but will lack org scoping.
			} else {
				// Set live mode based on IAM permissions (same as service token path)
				perms := iamPermissions(claims)
				if perms.Has(permission.Live) {
					org.Live = true
				}

				c.Set("organization", org)
				c.Set("active-organization", org.Id())
				c.Set("permissions", perms)
			}
		}

		c.Next()
	}
}

// iamPermissions converts IAM roles/claims to legacy permission bits.
func iamPermissions(claims *auth.IAMClaims) bit.Field {
	perms := permission.None

	if claims.IsAdmin {
		perms |= permission.Admin | permission.Live
	}

	// Map standard roles
	for _, role := range claims.Roles {
		switch role {
		case "admin", "owner":
			perms |= permission.Admin | permission.Live
		case "member", "user":
			perms |= permission.Published | permission.Live |
				permission.ReadCoupon | permission.ReadProduct
		}
	}

	// Default: at minimum grant Published if authenticated
	if perms == permission.None {
		perms = permission.Published | permission.Live
	}

	return bit.Field(perms)
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
