package referral

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/middleware"
	mdlreferral "github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

// loadProgramConfig returns the parsed referral program config.
func loadProgramConfig() *config.ReferralProgram {
	return config.GetReferralProgram()
}

// Route registers referral and referrer routes.
// This is the base layer: tracking referral codes, claims, credits, fraud.
// Affiliate/contributor routes are registered separately via api/affiliate.
func Route(r router.Router, args ...gin.HandlerFunc) {
	tokenRequired := middleware.TokenRequired()

	// --- Referral model auto-CRUD ---
	rest.New(mdlreferral.Referral{}).Route(r, args...)

	// --- Referral claim ---
	claimGroup := r.Group("referral")
	claimGroup.Use(tokenRequired)
	claimGroup.POST("/claim", ClaimReferral)

	// --- Referrer CRUD + custom endpoints ---
	referrerRest := rest.New(referrer.Referrer{})
	referrerRest.Create = referrerCreate(referrerRest)
	referrerRest.Get = referrerGet(referrerRest)
	referrerRest.GET("/me", append(args, getMyReferrer)...)
	referrerRest.GET("/code/:code", append(args, getByCode)...)
	referrerRest.Route(r, args...)
}
