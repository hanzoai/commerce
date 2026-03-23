package referral

import (
	_ "embed"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/router"
)

//go:embed referral-program.json
var programJSON []byte

// programConfig is the parsed referral program configuration, loaded once.
var programConfig *programConfigData

type tier struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	MinReferrals int    `json:"minReferrals"`
	MaxReferrals *int   `json:"maxReferrals"` // nil means unlimited
	Rewards      struct {
		ReferrerCreditCents int     `json:"referrerCreditCents"`
		RefereeCreditCents  int     `json:"refereeCreditCents"`
		RevenueSharePercent float64 `json:"revenueSharePercent"`
		CreditCurrency      string  `json:"creditCurrency"`
	} `json:"rewards"`
	Limits struct {
		MaxTotalCreditsCents int `json:"maxTotalCreditsCents"`
		CreditExpiryDays     int `json:"creditExpiryDays"`
	} `json:"limits"`
}

type fraudConfig struct {
	RequireEmailVerification bool `json:"requireEmailVerification"`
	BlockSelfReferral        bool `json:"blockSelfReferral"`
	CooldownDays             int  `json:"cooldownDays"`
	MaxReferralsPerDay       int  `json:"maxReferralsPerDay"`
	BlacklistSameIP          bool `json:"blacklistSameIP"`
}

type programConfigData struct {
	Id      string      `json:"id"`
	Name    string      `json:"name"`
	Version int         `json:"version"`
	Active  bool        `json:"active"`
	Tiers   []tier      `json:"tiers"`
	Fraud   fraudConfig `json:"fraud"`
}

// TierForCount returns the tier matching the given referral count.
func (p *programConfigData) TierForCount(count int) tier {
	for i := len(p.Tiers) - 1; i >= 0; i-- {
		if count >= p.Tiers[i].MinReferrals {
			return p.Tiers[i]
		}
	}
	if len(p.Tiers) > 0 {
		return p.Tiers[0]
	}
	return tier{Id: "starter"}
}

// loadProgramConfig returns the parsed program config, loading from the
// embedded JSON on first call.
func loadProgramConfig() *programConfigData {
	if programConfig != nil {
		return programConfig
	}
	cfg := &programConfigData{}
	if err := json.DecodeBytes(programJSON, cfg); err != nil {
		// Return safe defaults
		return &programConfigData{
			Fraud: fraudConfig{MaxReferralsPerDay: 50},
			Tiers: []tier{{
				Id:           "starter",
				MinReferrals: 0,
			}},
		}
	}
	programConfig = cfg
	return programConfig
}

// Route registers the referral claim endpoint.
func Route(r router.Router, args ...gin.HandlerFunc) {
	tokenRequired := middleware.TokenRequired()

	api := r.Group("referral")
	api.Use(tokenRequired)
	api.POST("/claim", ClaimReferral)
}

