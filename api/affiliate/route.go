package affiliate

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	mdlaffiliate "github.com/hanzoai/commerce/models/affiliate"
	"github.com/hanzoai/commerce/models/contributor"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

// Route registers affiliate and contributor routes.
// This builds on top of the referral base layer: revenue share,
// commissions, payouts, OSS contributor attribution.
func Route(r router.Router, args ...gin.HandlerFunc) {
	tokenRequired := middleware.TokenRequired()
	adminRequired := middleware.TokenRequired(permission.Admin)

	// --- Affiliate CRUD + custom endpoints ---
	affiliateRest := rest.New(mdlaffiliate.Affiliate{})
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
