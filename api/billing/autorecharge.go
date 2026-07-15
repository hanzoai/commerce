package billing

import (
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/autorecharge"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/kms"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// loadAutoRecharge returns the org's auto-recharge config (keyed by the org
// slug) or nil if none exists.
func loadAutoRecharge(db *datastore.Datastore, userId string) *autorecharge.AutoRecharge {
	rootKey := db.NewKey("synckey", "", 1, nil)
	cfgs := make([]*autorecharge.AutoRecharge, 0, 1)
	if _, err := autorecharge.Query(db).Ancestor(rootKey).Filter("UserId=", userId).GetAll(&cfgs); err != nil {
		return nil
	}
	if len(cfgs) == 0 {
		return nil
	}
	return cfgs[0]
}

// defaultPaymentMethod returns the customer's default payment method, or nil.
func defaultPaymentMethod(db *datastore.Datastore, customerId string) *paymentmethod.PaymentMethod {
	rootKey := db.NewKey("synckey", "", 1, nil)
	pms := make([]*paymentmethod.PaymentMethod, 0, 1)
	if _, err := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", customerId).
		Filter("IsDefault=", true).GetAll(&pms); err != nil {
		return nil
	}
	if len(pms) == 0 {
		return nil
	}
	return pms[0]
}

func autoRechargeResponse(cfg *autorecharge.AutoRecharge, userId string) map[string]any {
	if cfg == nil {
		return map[string]any{
			"userId":         userId,
			"enabled":        false,
			"thresholdCents": 0,
			"amountCents":    0,
			"currency":       "usd",
		}
	}
	return map[string]any{
		"userId":          cfg.UserId,
		"enabled":         cfg.Enabled,
		"thresholdCents":  cfg.ThresholdCents,
		"amountCents":     cfg.AmountCents,
		"currency":        cfg.Currency,
		"lastRechargedAt": cfg.LastRechargedAt,
	}
}

// GetAutoRecharge returns the org's auto-recharge config (or disabled defaults).
//
//	GET /v1/billing/auto-recharge
func GetAutoRecharge(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	cfg := loadAutoRecharge(db, org.Name)
	return c.JSON(200, autoRechargeResponse(cfg, org.Name))
}

type autoRechargeRequest struct {
	Enabled        bool   `json:"enabled"`
	ThresholdCents int64  `json:"thresholdCents"`
	AmountCents    int64  `json:"amountCents"`
	Currency       string `json:"currency,omitempty"`
}

// SetAutoRecharge upserts the org's auto-recharge config.
//
//	PUT /v1/billing/auto-recharge
//
// Enabling requires a default payment method on file (the card that will be
// charged off-session when the balance runs low).
func SetAutoRecharge(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req autoRechargeRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}

	if req.Enabled {
		if req.AmountCents <= 0 {
			return jsonhttp.Fail(c, 400, "amountCents must be positive", nil)
		}
		if req.ThresholdCents < 0 {
			return jsonhttp.Fail(c, 400, "thresholdCents must not be negative", nil)
		}
		if defaultPaymentMethod(db, org.Name) == nil {
			return jsonhttp.Fail(c, 400, "a default payment method is required to enable auto-recharge", nil)
		}
	}

	cur := strings.ToLower(strings.TrimSpace(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	cfg := loadAutoRecharge(db, org.Name)
	if cfg == nil {
		cfg = autorecharge.New(db)
		cfg.UserId = org.Name
	}
	cfg.Enabled = req.Enabled
	cfg.ThresholdCents = req.ThresholdCents
	cfg.AmountCents = req.AmountCents
	cfg.Currency = cur

	var err error
	if cfg.Id() == "" {
		err = cfg.Create()
	} else {
		err = cfg.Update()
	}
	if err != nil {
		log.Error("Failed to save auto-recharge config for org %q: %v", org.Name, err, c)
		return jsonhttp.Fail(c, 500, "failed to save auto-recharge config", err)
	}

	return c.JSON(200, autoRechargeResponse(cfg, org.Name))
}

type autoRechargeRunResult struct {
	OrgName       string `json:"orgName"`
	UserId        string `json:"userId"`
	Charged       bool   `json:"charged"`
	AmountCents   int64  `json:"amountCents,omitempty"`
	BalanceCents  int64  `json:"balanceCents,omitempty"`
	TransactionId string `json:"transactionId,omitempty"`
	Error         string `json:"error,omitempty"`
}

// RunAutoRechargeAllOrgs iterates every organization and, for those with
// auto-recharge enabled whose available balance is below the threshold, charges
// the default payment method and credits the balance. Intended to be invoked on
// a recurring schedule (CronJob) by the platform.
//
//	POST /v1/billing/auto-recharge/run-all
func RunAutoRechargeAllOrgs(c *zip.Ctx) error {
	var kmsClient *kms.CachedClient
	if v := c.Locals("kms"); v != nil {
		kmsClient, _ = v.(*kms.CachedClient)
	}

	rootDb := datastore.New(c.Context())
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(rootDb).GetAll(&orgs); err != nil {
		log.Error("Failed to list organizations for auto-recharge: %v", err, c)
		return jsonhttp.Fail(c, 500, "failed to list organizations", err)
	}

	results := make([]autoRechargeRunResult, 0)
	charged := 0

	for _, org := range orgs {
		db := datastore.New(org.Namespaced(c.Context()))

		cfg := loadAutoRecharge(db, org.Name)
		if cfg == nil || !cfg.Enabled || cfg.AmountCents <= 0 {
			continue
		}

		cur := currency.Type(strings.ToLower(cfg.Currency))
		if cur == "" {
			cur = "usd"
		}

		// Available = balance - holds. Skip if still above the threshold.
		var available int64
		if datas, err := util.GetTransactionsByCurrency(org.Namespaced(c.Context()), org.Name, "iam-user", cur, org.TestMode()); err == nil {
			if data, ok := datas.Data[cur]; ok {
				available = int64(data.Balance) - int64(data.Holds)
			}
		}
		if available >= cfg.ThresholdCents {
			continue
		}

		pm := defaultPaymentMethod(db, org.Name)
		if pm == nil {
			results = append(results, autoRechargeRunResult{
				OrgName: org.Name, UserId: org.Name, Charged: false,
				Error: "no default payment method",
			})
			continue
		}

		// Hydrate this org's payment credentials so ProcessorsForOrg sees real
		// Square creds for the off-session charge.
		if kmsClient != nil {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q during auto-recharge: %v", org.Name, err, c)
			}
		}

		desc := fmt.Sprintf("Auto-recharge %d %s for %s", cfg.AmountCents, cur, org.Name)
		txID, balanceCents, err := chargeAndCredit(c, org, db, pm, cfg.AmountCents, cur, org.Name, desc)
		if err != nil {
			log.Error("Auto-recharge charge failed for org %q: %v", org.Name, err, c)
			results = append(results, autoRechargeRunResult{
				OrgName: org.Name, UserId: org.Name, Charged: false, Error: err.Error(),
			})
			continue
		}

		cfg.LastRechargedAt = time.Now().UTC().Format(time.RFC3339)
		_ = cfg.Update()

		charged++
		results = append(results, autoRechargeRunResult{
			OrgName:       org.Name,
			UserId:        org.Name,
			Charged:       true,
			AmountCents:   cfg.AmountCents,
			BalanceCents:  int64(balanceCents),
			TransactionId: txID,
		})
	}

	return c.JSON(200, map[string]any{
		"orgs":    len(orgs),
		"charged": charged,
		"results": results,
	})
}
