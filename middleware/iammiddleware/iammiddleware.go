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
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/bit"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

// KVCache is the minimal interface required for org-lookup caching.
// *infra.KVClient satisfies this interface.
type KVCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

var (
	iamClient   *auth.IAMClient
	initAttempt bool // true once Init has been called, regardless of success
	mu          sync.RWMutex

	kvCache KVCache // optional KV client for org-lookup caching
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

// InitKV wires a KV client for caching IAM org lookups.
// Call from app.Bootstrap() after infra is connected.
// Passing nil is safe and disables KV caching.
func InitKV(kv KVCache) {
	mu.Lock()
	defer mu.Unlock()
	kvCache = kv
}

// orgCacheKey returns the KV key for an IAM owner → org ID mapping.
func orgCacheKey(owner string) string {
	return "iam:org_by_name:" + owner
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
			// Fall through to legacy auth so other middleware can handle it.
			log.Warn("IAM token validation failed: %v", err)
			c.Next()
			return
		}

		// Valid IAM token -- set context values
		c.Set("iam_claims", claims)
		c.Set("iam_user_id", claims.Subject)
		c.Set("iam_email", claims.Email)
		c.Set("iam_org", claims.Owner)
		c.Set("iam_roles", claims.Roles)
		c.Set("iam_tier", claims.Tier())
		c.Set("iam_authenticated", true)

		// Resolve organization from IAM "owner" claim so downstream handlers
		// get proper tenant scoping via middleware.GetOrganization(c).
		// IAM is the source of truth for org/identity — auto-create the
		// Commerce org record on first encounter.
		if claims.Owner == "" {
			jsonhttp.Fail(c, http.StatusUnauthorized,
				"IAM token missing owner claim", nil)
			return
		}

		// Use a dedicated context with timeout for the DB lookup.
		// The HTTP request context can be canceled by the browser (e.g. page
		// navigation or AbortController), which would cause org resolution
		// to fail with "context canceled" and leave downstream handlers
		// without an organization — triggering a MustGet panic.
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dbCancel()

		db := datastore.New(dbCtx)
		org := organization.New(db)
		org.Name = claims.Owner
		org.Enabled = true

		var found bool

		// Fast path: KV cache maps owner name → org ID (5-minute TTL).
		mu.RLock()
		kv := kvCache
		mu.RUnlock()

		if kv != nil {
			if cachedID, err := kv.Get(dbCtx, orgCacheKey(claims.Owner)); err == nil && cachedID != "" {
				if lookupErr := org.GetById(cachedID); lookupErr == nil {
					found = true
				}
			}
		}

		// Slow path: GetOrCreate by name (auto-provisions org on first encounter).
		if !found {
			if err := org.GetOrCreate("Name=", claims.Owner); err != nil {
				log.Warn("IAM org resolve/create for '%s' failed: %v", claims.Owner, err)
			} else {
				found = true
				// Populate KV cache for next request.
				if kv != nil {
					_ = kv.Set(dbCtx, orgCacheKey(claims.Owner), org.Id(), 5*time.Minute)
				}

				// If org was just created (CreatedAt within the last few
				// seconds), grant a $5 starter credit. Runs in a goroutine
				// so it never blocks the request.
				if time.Since(org.GetCreatedAt()) < 5*time.Second && claims.Subject != "" {
					nsDb := datastore.New(org.Namespaced(context.Background()))
					go credit.GrantIfEligible(nsDb, claims.Subject, "org-created")
				}
			}
		}

		if !found {
			// Org resolution failed — return a proper error instead of
			// falling through to handlers that will panic on MustGet.
			log.Warn("IAM org '%s' could not be resolved; returning 503", claims.Owner)
			jsonhttp.Fail(c, http.StatusServiceUnavailable,
				"Unable to retrieve organization associated with access token: org resolution failed", nil)
			return
		}

		// Set live mode based on IAM permissions (same as service token path)
		perms := iamPermissions(claims)
		if perms.Has(permission.Live) {
			org.Live = true
		}

		c.Set("organization", org)
		c.Set("active-organization", org.Id())
		c.Set("permissions", perms)

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

// GetIAMTier returns the user's billing tier from context.
// Returns an empty string if the request is not IAM-authenticated or no tier is set.
func GetIAMTier(c *gin.Context) string {
	val, exists := c.Get("iam_tier")
	if !exists {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}
