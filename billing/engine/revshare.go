package engine

import (
	"embed"
	"encoding/json"
	"math"
	"sync"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/fee"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/types/currency"
)

//go:embed referral-program.json
var referralProgramFS embed.FS

// referralProgramTier mirrors one tier from config/referral-program.json.
type referralProgramTier struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MinReferrals int    `json:"minReferrals"`
	MaxReferrals *int   `json:"maxReferrals"` // nil = unlimited
	Rewards      struct {
		RevenueSharePercent float64 `json:"revenueSharePercent"`
	} `json:"rewards"`
}

type referralProgramConfig struct {
	Tiers []referralProgramTier `json:"tiers"`
}

var (
	programOnce   sync.Once
	programConfig referralProgramConfig
)

func loadReferralProgram() referralProgramConfig {
	programOnce.Do(func() {
		data, err := referralProgramFS.ReadFile("referral-program.json")
		if err != nil {
			log.Error("Failed to read embedded referral-program.json: %v", err)
			return
		}
		if err := json.Unmarshal(data, &programConfig); err != nil {
			log.Error("Failed to parse referral-program.json: %v", err)
		}
	})
	return programConfig
}

// tierForReferralCount returns the tier matching the given referral count.
// Tiers are ordered by minReferrals ascending; the highest matching tier wins.
func tierForReferralCount(count int) referralProgramTier {
	cfg := loadReferralProgram()
	if len(cfg.Tiers) == 0 {
		return referralProgramTier{}
	}

	best := cfg.Tiers[0]
	for _, t := range cfg.Tiers {
		if count >= t.MinReferrals {
			best = t
		}
	}
	return best
}

// TrackRevenueShare checks if the paying user was referred and, if so, creates
// an affiliate Fee record for the referrer's revenue share. This is fire-and-forget;
// errors are logged but never propagated to the caller.
//
// Parameters:
//   - db: datastore scoped to the org namespace
//   - userID: the IAM user who incurred the usage charge
//   - chargeAmount: the usage charge in cents
//   - cur: the currency of the charge
//   - transactionID: the ID of the withdraw transaction (for Fee.PaymentId)
//   - test: true if the org is not live
func TrackRevenueShare(db *datastore.Datastore, userID string, chargeAmount currency.Cents, cur currency.Type, transactionID string, test bool) {
	// 1. Look up an active referral for this user.
	referrals := make([]*referral.Referral, 0, 1)
	q := referral.Query(db).
		Filter("UserId=", userID).
		Filter("Revoked=", false).
		Limit(1)

	if _, err := q.GetAll(&referrals); err != nil {
		log.Error("revshare: failed to query referral for user %s: %v", userID, err)
		return
	}
	if len(referrals) == 0 {
		return // not a referred user
	}
	ref := referrals[0]

	// 2. Look up the referrer to get affiliateId and count referrals for tier.
	referrerID := ref.Referrer.Id
	if referrerID == "" {
		return
	}

	rfr := referrer.New(db)
	if err := rfr.GetById(referrerID); err != nil {
		log.Error("revshare: failed to load referrer %s: %v", referrerID, err)
		return
	}

	// 3. Count total referrals to determine tier.
	referralCount, err := referral.Query(db).
		Filter("Referrer.Id=", referrerID).
		Count()
	if err != nil {
		log.Error("revshare: failed to count referrals for referrer %s: %v", referrerID, err)
		return
	}

	tier := tierForReferralCount(referralCount)
	if tier.Rewards.RevenueSharePercent <= 0 {
		return // this tier has no revenue share
	}

	// 4. Calculate commission amount (round down — platform keeps remainder).
	commissionAmount := currency.Cents(math.Floor(
		float64(chargeAmount) * tier.Rewards.RevenueSharePercent / 100.0,
	))
	if commissionAmount <= 0 {
		return
	}

	// 5. Create the affiliate Fee record.
	affiliateID := rfr.AffiliateId
	if affiliateID == "" {
		// Referrer has no affiliate account; log and skip.
		log.Debug("revshare: referrer %s has no affiliateId, skipping fee creation", referrerID)
		return
	}

	// Load the affiliate to get its key for parenting the fee.
	if err := rfr.LoadAffiliate(); err != nil {
		log.Error("revshare: failed to load affiliate %s: %v", affiliateID, err)
		return
	}

	fe := fee.New(db)
	fe.Name = "Referral revenue share"
	fe.Type = fee.Affiliate
	fe.AffiliateId = affiliateID
	fe.Parent = rfr.Affiliate.Key()
	fe.Currency = cur
	fe.Amount = commissionAmount
	fe.PaymentId = transactionID
	fe.Status = fee.Pending
	fe.Live = !test
	fe.Test = test

	if err := fe.Create(); err != nil {
		log.Error("revshare: failed to create fee for referrer %s, amount %d: %v", referrerID, commissionAmount, err)
		return
	}

	log.Info("revshare: created fee %s for referrer %s (affiliate %s), amount %d %s (%.2f%% of %d)",
		fe.Id(), referrerID, affiliateID, commissionAmount, cur, tier.Rewards.RevenueSharePercent, chargeAmount)
}
