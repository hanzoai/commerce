package referral

import (
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	mdlreferral "github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/types/client"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

// referrerCreate returns the custom Create handler for referrer CRUD.
func referrerCreate(api *rest.Rest) func(*zip.Ctx) error {
	return func(c *zip.Ctx) error {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		ref := referrer.New(db)

		// Decode request body
		if err := json.DecodeBytes(c.Body(), ref); err != nil {
			return http.Fail(c, 400, "Failed decode request body", err)
		}

		// Override userId from IAM if the gateway authenticated the caller
		// AND the request body didn't supply one. claims is always non-nil;
		// an empty Subject leaves ref.UserId untouched.
		if ref.UserId == "" {
			if subject := iammiddleware.GetIAMClaims(c).Subject; subject != "" {
				ref.UserId = subject
			}
		}

		// Auto-generate code if not provided
		if ref.Code == "" {
			ref.Code = referrer.NewCode()
		}

		// Ensure code is unique
		if _, ok, _ := referrer.Query(db).Filter("Code=", ref.Code).FirstKey(); ok {
			ref.Code = referrer.NewCode()
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
			return http.Fail(c, 500, "Failed to create referrer", err)
		}
		c.SetHeader("Location", c.Path()+"/"+ref.Id())
		return api.Render(c, 201, ref)
	}
}

// referrerGet returns the custom Get handler for referrer CRUD.
func referrerGet(api *rest.Rest) func(*zip.Ctx) error {
	return func(c *zip.Ctx) error {
		org := middleware.GetOrganization(c)
		db := datastore.New(org.Namespaced(c.Context()))
		ref := referrer.New(db)

		id := c.Param(api.ParamId)

		if err := ref.GetById(id); err != nil {
			return http.Fail(c, 404, "No Referrer found with id: "+id, err)
		}

		if err := ref.LoadAffiliate(); err != nil {
			return http.Fail(c, 500, "Referrer affiliate data could not be queries", err)
		}

		return api.Render(c, 200, ref)
	}
}

// getMyReferrer returns the current user's referrer record with stats and tier.
//
//	GET /v1/referrer/me
func getMyReferrer(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	userId := iamUserIdOrQuery(c)
	if userId == "" {
		return http.Fail(c, 400, "userId is required (pass as query param or use IAM token)", nil)
	}

	ref := referrer.New(db)
	key, ok, err := referrer.Query(db).Filter("UserId=", userId).First(ref)
	if err != nil {
		log.Error("Failed to query referrer: %v", err, c)
		return http.Fail(c, 500, "failed to query referrer", err)
	}
	if !ok {
		return http.Fail(c, 404, "no referrer found for this user", nil)
	}
	ref.Init(db)
	ref.SetKey(key)

	// Count referrals
	referralCount := 0
	allReferrals := make([]*mdlreferral.Referral, 0)
	if _, err := mdlreferral.Query(db).Filter("Referrer.Id=", ref.Id()).GetAll(&allReferrals); err == nil {
		referralCount = len(allReferrals)
	}

	return c.JSON(200, map[string]any{
		"referrer":      ref,
		"referralCount": referralCount,
		"code":          ref.Code,
		"shareUrl":      "https://hanzo.ai/signup?ref=" + ref.Code,
	})
}

// getByCode validates that a referral code exists.
//
//	GET /v1/referrer/code/:code
func getByCode(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		return http.Fail(c, 400, "code is required", nil)
	}

	if _, ok, err := referrer.Query(db).Filter("Code=", code).FirstKey(); err != nil {
		log.Error("Failed to query referrer by code: %v", err, c)
		return http.Fail(c, 500, "failed to look up referral code", err)
	} else if !ok {
		return c.JSON(404, map[string]any{"valid": false})
	}

	return c.JSON(200, map[string]any{"valid": true})
}

// iamUserIdOrQuery returns the IAM user ID from JWT claims or from query
// param. claims is always non-nil; an empty Subject means the gateway
// did not authenticate the caller and we fall back to the explicit
// query parameter.
func iamUserIdOrQuery(c *zip.Ctx) string {
	if subject := iammiddleware.GetIAMClaims(c).Subject; subject != "" {
		return subject
	}
	return strings.TrimSpace(c.Query("userId"))
}
