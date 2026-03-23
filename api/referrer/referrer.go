package referrer

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rand"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

func Route(r router.Router, args ...gin.HandlerFunc) {
	api := rest.New(referrer.Referrer{})

	api.Create = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))
		ref := referrer.New(db)

		// Decode request body
		if err := json.Decode(c.Request.Body, ref); err != nil {
			http.Fail(c, 400, "Failed decode request body", err)
			return
		}

		// Override userId from IAM if available
		if claims := iammiddleware.GetIAMClaims(c); claims != nil && ref.UserId == "" {
			ref.UserId = claims.Subject
		}

		// Auto-generate code if not provided
		if ref.Code == "" {
			ref.Code = generateCode()
		}

		// Ensure code is unique
		if _, ok, _ := referrer.Query(db).Filter("Code=", ref.Code).FirstKey(); ok {
			ref.Code = generateCode()
		}

		// Save client-related data about request used to create referrer
		ref.Client = client.New(c)

		// Check if this is blacklisted IP
		ref.Blacklisted = ref.Client.Blacklisted()

		// Check if any other referrers have been created with this IP address
		if _, ok, _ := referrer.Query(db).Filter("Client.Ip=", ref.Client.Ip).FirstKey(); ok {
			ref.Duplicate = true
		}

		if err := ref.Create(); err != nil {
			http.Fail(c, 500, "Failed to create referrer", err)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path+"/"+ref.Id())
			api.Render(c, 201, ref)
		}
	}

	api.Get = func(c *gin.Context) {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c))
		ref := referrer.New(db)

		id := c.Params.ByName(api.ParamId)

		if err := ref.GetById(id); err != nil {
			http.Fail(c, 404, "No Referrer found with id: "+id, err)
			return
		}

		if err := ref.LoadAffiliate(); err != nil {
			http.Fail(c, 500, "Referrer affiliate data could not be queries", err)
			return
		}

		api.Render(c, 200, ref)
	}

	// Custom endpoints: middleware is passed explicitly because rest.Handle
	// does not apply the group middleware to custom routes.
	api.GET("/me", append(args, getMyReferrer)...)
	api.GET("/code/:code", append(args, getByCode)...)

	api.Route(r, args...)
}

// getMyReferrer returns the current user's referrer record with stats and tier.
//
//	GET /api/v1/referrer/me
func getMyReferrer(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	userId := iamUserIdOrQuery(c)
	if userId == "" {
		http.Fail(c, 400, "userId is required (pass as query param or use IAM token)", nil)
		return
	}

	ref := referrer.New(db)
	key, ok, err := referrer.Query(db).Filter("UserId=", userId).First(ref)
	if err != nil {
		log.Error("Failed to query referrer: %v", err, c)
		http.Fail(c, 500, "failed to query referrer", err)
		return
	}
	if !ok {
		http.Fail(c, 404, "no referrer found for this user", nil)
		return
	}
	ref.Init(db)
	ref.SetKey(key)

	// Count referrals
	referralCount := 0
	allReferrals := make([]*referral.Referral, 0)
	if _, err := referral.Query(db).Filter("Referrer.Id=", ref.Id()).GetAll(&allReferrals); err == nil {
		referralCount = len(allReferrals)
	}

	c.JSON(200, gin.H{
		"referrer":      ref,
		"referralCount": referralCount,
		"code":          ref.Code,
		"shareUrl":      "https://hanzo.ai/signup?ref=" + ref.Code,
	})
}

// getByCode validates that a referral code exists.
//
//	GET /api/v1/referrer/code/:code
func getByCode(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		http.Fail(c, 400, "code is required", nil)
		return
	}

	if _, ok, err := referrer.Query(db).Filter("Code=", code).FirstKey(); err != nil {
		log.Error("Failed to query referrer by code: %v", err, c)
		http.Fail(c, 500, "failed to look up referral code", err)
		return
	} else if !ok {
		c.JSON(404, gin.H{"valid": false})
		return
	}

	c.JSON(200, gin.H{"valid": true})
}

// generateCode creates a short, uppercase, URL-friendly referral code.
func generateCode() string {
	id := rand.ShortId()
	clean := strings.NewReplacer("-", "", "_", "").Replace(id)
	if len(clean) > 8 {
		clean = clean[:8]
	}
	return strings.ToUpper(clean)
}

// iamUserIdOrQuery returns the IAM user ID from JWT claims or from query param.
func iamUserIdOrQuery(c *gin.Context) string {
	if claims := iammiddleware.GetIAMClaims(c); claims != nil {
		return claims.Subject
	}
	return strings.TrimSpace(c.Query("userId"))
}
