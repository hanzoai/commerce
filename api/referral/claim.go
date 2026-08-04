package referral

import (
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/util/json/http"
)

type claimRequest struct {
	Code   string `json:"code"`
	UserId string `json:"userId"`
	Email  string `json:"email"`
}

// ClaimReferral processes a referral claim: validates the code, checks fraud
// rules, and records the referral. It moves no money and mints no credit.
//
//	POST /v1/referral/claim
func ClaimReferral(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req claimRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.Code == "" {
		return http.Fail(c, 400, "code is required", nil)
	}

	// Determine referee userId: prefer the gateway-authenticated user
	// (claims.Subject) over the request body to prevent a referee from
	// claiming on behalf of another user. claims is always non-nil;
	// an empty Subject means the request was anonymous and we fall back
	// to the request body.
	refereeUserId := req.UserId
	if subject := iammiddleware.GetIAMClaims(c).Subject; subject != "" {
		refereeUserId = subject
	}
	if refereeUserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	// 1. Look up referrer by code.
	ref := referrer.New(db)
	if key, ok, err := referrer.Query(db).Filter("Code=", req.Code).First(ref); err != nil {
		log.Error("Failed to query referrer by code: %v", err, c)
		return http.Fail(c, 500, "failed to look up referral code", err)
	} else if !ok {
		return http.Fail(c, 404, "invalid referral code", nil)
	} else {
		ref.Init(db)
		ref.SetKey(key)
	}

	// 2. Validate: not blacklisted.
	if ref.Blacklisted {
		return http.Fail(c, 403, "referral code is not available", nil)
	}

	// 3. Validate: not self-referral.
	if ref.UserId == refereeUserId {
		return http.Fail(c, 403, "cannot use your own referral code", nil)
	}

	// 4. Validate: not duplicate (same referee userId already claimed).
	existingReferrals := make([]*referral.Referral, 0)
	if _, err := referral.Query(db).Filter("UserId=", refereeUserId).Limit(1).GetAll(&existingReferrals); err != nil {
		log.Error("Failed to check existing referrals: %v", err, c)
		return http.Fail(c, 500, "failed to check existing referrals", err)
	}
	if len(existingReferrals) > 0 {
		return http.Fail(c, 409, "user has already claimed a referral", nil)
	}

	// 5. Validate: referrer daily limit.
	cfg := loadProgramConfig()
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayReferrals := make([]*referral.Referral, 0)
	if _, err := referral.Query(db).Filter("Referrer.Id=", ref.Id()).GetAll(&todayReferrals); err != nil {
		log.Error("Failed to count daily referrals: %v", err, c)
		return http.Fail(c, 500, "failed to check daily limit", err)
	}
	todayCount := 0
	for _, r := range todayReferrals {
		if r.CreatedAt.After(todayStart) {
			todayCount++
		}
	}
	if todayCount >= cfg.Fraud.MaxReferralsPerDay {
		return http.Fail(c, 429, "referrer has reached the daily referral limit", nil)
	}

	// 6. Determine the referrer's tier based on total referral count.
	allReferrals := make([]*referral.Referral, 0)
	referralCount := 0
	if _, err := referral.Query(db).Filter("Referrer.Id=", ref.Id()).GetAll(&allReferrals); err == nil {
		referralCount = len(allReferrals)
	}
	tier := cfg.TierForCount(referralCount)

	now := time.Now()

	// Claiming a code MINTS NOTHING. It used to write two CreditGrants here — a
	// code-driven, self-service credit mint gated only by TokenRequired. The
	// referral reward is EARNED, not granted at signup: billing/engine.TrackRevenueShare
	// accrues it as a fee.Fee (a tracked PAYABLE) when the referee actually spends.
	// A claim only records the relationship that makes that accrual possible.

	// 7. Create the Referral record.
	rfl := referral.New(db)
	rfl.Type = referral.NewUser
	rfl.UserId = refereeUserId
	rfl.Referrer = referral.Referrer{
		Id:          ref.Id(),
		UserId:      ref.UserId,
		AffiliateId: ref.AffiliateId,
	}

	if err := rfl.Create(); err != nil {
		log.Error("Failed to create referral record: %v", err, c)
		return http.Fail(c, 500, "failed to create referral record", err)
	}

	// 8. Update referrer state.
	if ref.FirstReferredAt.IsZero() {
		ref.FirstReferredAt = now
	}
	if err := ref.Update(); err != nil {
		log.Warn("Failed to update referrer state: %v", err, c)
	}

	log.Info("Referral claimed: referee=%s referrer=%s code=%s tier=%s", refereeUserId, ref.UserId, req.Code, tier.Id, c)

	return c.JSON(201, map[string]any{
		"referralId": rfl.Id(),
		"referrerId": ref.Id(),
		"refereeId":  refereeUserId,
		"tier":       tier.Id,
	})
}
