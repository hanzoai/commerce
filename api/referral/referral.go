package referral

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/affiliate"
	"github.com/hanzoai/commerce/models/contributor"
	mdlreferral "github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

// loadProgramConfig returns the parsed referral program config from the
// shared config package.
func loadProgramConfig() *config.ReferralProgram {
	return config.GetReferralProgram()
}

// Route registers all referral, referrer, affiliate, and contributor routes.
func Route(r router.Router, args ...gin.HandlerFunc) {
	tokenRequired := middleware.TokenRequired()
	adminRequired := middleware.TokenRequired(permission.Admin)

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

	// --- Affiliate CRUD + custom endpoints ---
	affiliateRest := rest.New(affiliate.Affiliate{})
	affiliateRest.Create = affiliateCreate(affiliateRest)
	affiliateRest.GET("/:affiliateid/connect", tokenRequired, middleware.Namespace(), affiliateConnect)
	affiliateRest.GET("/:affiliateid/referrals", append(args, affiliateGetReferrals)...)
	affiliateRest.GET("/:affiliateid/referrers", append(args, affiliateGetReferrers)...)
	affiliateRest.GET("/:affiliateid/orders", append(args, affiliateGetOrders)...)
	affiliateRest.GET("/:affiliateid/transactions", append(args, affiliateGetTransactions)...)
	affiliateRest.Route(r, args...)

	// --- Contributor CRUD + custom endpoints ---
	contributorRest := rest.New(contributor.Contributor{})
	contributorRest.Create = contributorCreate(contributorRest)
	contributorRest.POST("/register", tokenRequired, registerContributor)
	contributorRest.GET("/by-login/:login", tokenRequired, contributorGetByLogin)
	contributorRest.POST("/sbom", adminRequired, createSBOMEntry)
	contributorRest.GET("/sbom", adminRequired, listSBOMEntries)
	contributorRest.GET("/sbom/:sbomid", adminRequired, getSBOMEntry)
	contributorRest.PUT("/sbom/:sbomid", adminRequired, updateSBOMEntry)
	contributorRest.POST("/payouts/calculate", adminRequired, calculatePayouts)
	contributorRest.GET("/payouts/preview", adminRequired, previewPayouts)
	contributorRest.GET("/:contributorid/earnings", tokenRequired, getEarnings)
	contributorRest.GET("/:contributorid/attributions", tokenRequired, getAttributions)
	contributorRest.Route(r, args...)
}
