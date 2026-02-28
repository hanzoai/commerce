package test

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ginclient"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

// newClient creates a ginclient with IAM middleware and context propagation.
// The token parameter sets the Authorization Bearer header; empty string means no header.
func newClient(token string) *ginclient.Client {
	cl := ginclient.New(ctx)

	// Propagate test ae.Context into request context so IAM middleware's
	// datastore.New(c.Request.Context()) uses the test database.
	cl.Router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	cl.Use(iammiddleware.IAMTokenRequired())

	cl.Handle("GET", "/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	if token != "" {
		cl.Defaults(func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+token)
		})
	}

	return cl
}

var _ = Describe("middleware/iammiddleware", func() {

	// ── IAMTokenRequired fallthrough cases ──────────────────────────────

	Context("IAMTokenRequired fallthrough", func() {
		It("should fall through when no Authorization header is present", func() {
			cl := newClient("")
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})

		It("should fall through when Authorization is not Bearer", func() {
			cl := ginclient.New(ctx)
			cl.Router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			})
			cl.Use(iammiddleware.IAMTokenRequired())
			cl.Handle("GET", "/test", func(c *gin.Context) {
				c.String(200, "ok")
			})
			cl.Defaults(func(r *http.Request) {
				r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
			})
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})

		It("should fall through for malformed JWT token", func() {
			cl := newClient("not-a-valid-jwt")
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})

		It("should fall through for expired token", func() {
			claims := makeAdminClaims()
			claims.ExpiresAt = time.Now().Add(-time.Hour).Unix()
			token := signToken(claims)

			cl := newClient(token)
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})

		It("should fall through for wrong audience", func() {
			claims := makeAdminClaims()
			claims.Audience = auth.FlexAudience("wrong-client-id")
			token := signToken(claims)

			cl := newClient(token)
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})
	})

	// ── IAM context keys ────────────────────────────────────────────────

	Context("IAM context keys", func() {
		It("should set all IAM context keys for a valid token", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))

			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeTrue())

			iamClaims := iammiddleware.GetIAMClaims(cl.Context)
			Expect(iamClaims).NotTo(BeNil())
			Expect(iamClaims.Subject).To(Equal("user-admin-123"))
			Expect(iamClaims.Email).To(Equal("admin@test.com"))
			Expect(iamClaims.Owner).To(Equal(testOrgName))

			userId, _ := cl.Context.Get("iam_user_id")
			Expect(userId).To(Equal("user-admin-123"))

			email, _ := cl.Context.Get("iam_email")
			Expect(email).To(Equal("admin@test.com"))

			iamOrg, _ := cl.Context.Get("iam_org")
			Expect(iamOrg).To(Equal(testOrgName))

			roles, _ := cl.Context.Get("iam_roles")
			Expect(roles).To(Equal([]string{"admin"}))
		})
	})

	// ── Permission mapping ──────────────────────────────────────────────

	Context("permission mapping", func() {
		It("should grant Admin|Live for admin role", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, exists := cl.Context.Get("permissions")
			Expect(exists).To(BeTrue())
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Admin)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
		})

		It("should grant Admin|Live for owner role", func() {
			claims := makeMemberClaims()
			claims.Roles = []string{"owner"}
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, _ := cl.Context.Get("permissions")
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Admin)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
		})

		It("should grant Published|Live|ReadCoupon|ReadProduct for member role", func() {
			claims := makeMemberClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, exists := cl.Context.Get("permissions")
			Expect(exists).To(BeTrue())
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Published)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
			Expect(perms.Has(permission.ReadCoupon)).To(BeTrue())
			Expect(perms.Has(permission.ReadProduct)).To(BeTrue())
			Expect(perms.Has(permission.Admin)).To(BeFalse())
		})

		It("should grant Published|Live|ReadCoupon|ReadProduct for user role", func() {
			claims := makeMemberClaims()
			claims.Roles = []string{"user"}
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, _ := cl.Context.Get("permissions")
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Published)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
			Expect(perms.Has(permission.ReadCoupon)).To(BeTrue())
			Expect(perms.Has(permission.ReadProduct)).To(BeTrue())
		})

		It("should default to Published|Live for unrecognized role", func() {
			claims := makeMinimalClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, exists := cl.Context.Get("permissions")
			Expect(exists).To(BeTrue())
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Published)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
			Expect(perms.Has(permission.Admin)).To(BeFalse())
			Expect(perms.Has(permission.ReadCoupon)).To(BeFalse())
		})

		It("should accumulate permissions for IsAdmin + member role", func() {
			claims := makeMemberClaims()
			claims.IsAdmin = true
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			permsVal, _ := cl.Context.Get("permissions")
			perms := permsVal.(bit.Field)
			Expect(perms.Has(permission.Admin)).To(BeTrue())
			Expect(perms.Has(permission.Live)).To(BeTrue())
			Expect(perms.Has(permission.Published)).To(BeTrue())
			Expect(perms.Has(permission.ReadCoupon)).To(BeTrue())
			Expect(perms.Has(permission.ReadProduct)).To(BeTrue())
		})
	})

	// ── Organization resolution ─────────────────────────────────────────

	Context("organization resolution", func() {
		It("should resolve organization from Owner claim", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			orgVal, exists := cl.Context.Get("organization")
			Expect(exists).To(BeTrue())
			org := orgVal.(*organization.Organization)
			Expect(org.Name).To(Equal(testOrgName))
		})

		It("should set org.Live = true for IAM-authenticated user", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			orgVal, _ := cl.Context.Get("organization")
			org := orgVal.(*organization.Organization)
			Expect(org.Live).To(BeTrue())
		})

		It("should set active-organization context key", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			activeOrg, exists := cl.Context.Get("active-organization")
			Expect(exists).To(BeTrue())
			Expect(activeOrg).NotTo(BeEmpty())
		})

		It("should not abort when Owner does not match any organization", func() {
			claims := makeAdminClaims()
			claims.Owner = "nonexistent-org"
			token := signToken(claims)

			cl := newClient(token)
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))

			// IAM auth still succeeded
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeTrue())

			// But no org-specific permissions were set by IAM middleware
			_, permExists := cl.Context.Get("permissions")
			// Permissions should NOT be set by IAM middleware when org lookup fails
			// (the default from gincontext.SetDefaults does not set permissions)
			Expect(permExists).To(BeFalse())
		})

		It("should not abort when Owner claim is empty", func() {
			claims := makeAdminClaims()
			claims.Owner = ""
			token := signToken(claims)

			cl := newClient(token)
			w := cl.Get("/test", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeTrue())
		})
	})

	// ── Helper functions ────────────────────────────────────────────────

	Context("IsIAMAuthenticated", func() {
		It("should return true after IAM authentication", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeTrue())
		})

		It("should return false without IAM authentication", func() {
			cl := newClient("")
			cl.Get("/test", nil)

			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})
	})

	Context("GetIAMClaims", func() {
		It("should return claims after IAM authentication", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := newClient(token)
			cl.Get("/test", nil)

			iamClaims := iammiddleware.GetIAMClaims(cl.Context)
			Expect(iamClaims).NotTo(BeNil())
			Expect(iamClaims.Subject).To(Equal(claims.Subject))
			Expect(iamClaims.Email).To(Equal(claims.Email))
		})

		It("should return nil without IAM authentication", func() {
			cl := newClient("")
			cl.Get("/test", nil)

			iamClaims := iammiddleware.GetIAMClaims(cl.Context)
			Expect(iamClaims).To(BeNil())
		})
	})

	// ── Integration with TokenRequired ──────────────────────────────────

	Context("TokenRequired integration", func() {
		It("should bypass legacy token auth when IAM authenticated", func() {
			claims := makeAdminClaims()
			token := signToken(claims)

			cl := ginclient.New(ctx)
			cl.Router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			})
			// Chain: IAM middleware → TokenRequired → handler
			cl.Use(iammiddleware.IAMTokenRequired())
			cl.Use(middleware.TokenRequired())
			cl.Handle("GET", "/protected", func(c *gin.Context) {
				c.String(200, "ok")
			})
			cl.Defaults(func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+token)
			})

			// Without IAM auth, TokenRequired would 401 (no legacy access token).
			// With IAM auth, TokenRequired sees IsIAMAuthenticated=true and skips.
			w := cl.Get("/protected", nil)
			Expect(w.Code).To(Equal(200))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeTrue())
		})

		It("should fall through to legacy auth when IAM token is invalid", func() {
			cl := ginclient.New(ctx)
			cl.Router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			})
			cl.Use(iammiddleware.IAMTokenRequired())
			cl.Use(middleware.TokenRequired())
			cl.Handle("GET", "/protected", func(c *gin.Context) {
				c.String(200, "ok")
			})
			cl.IgnoreErrors(true)
			cl.Defaults(func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer invalid-jwt")
			})

			// IAM auth fails → falls through to TokenRequired → 401 (no legacy token)
			w := cl.Get("/protected", nil, 401)
			Expect(w.Code).To(Equal(401))
			Expect(iammiddleware.IsIAMAuthenticated(cl.Context)).To(BeFalse())
		})
	})
})
