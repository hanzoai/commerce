package store

import (
	"net/http"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/trial"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	storemodel "github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/models/subscription"
)

type accessState struct {
	Allowed bool   `json:"allowed"`
	StoreID string `json:"storeId,omitempty"`
	Status  string `json:"status"`
}

// HasAccess is the single store entitlement rule. Each store needs its own
// current $20 plan subscription; an org-wide balance or another store's plan
// cannot unlock it.
func HasAccess(db *datastore.Datastore, storeID string, at time.Time) (bool, string, error) {
	if strings.TrimSpace(storeID) == "" {
		return false, "store_required", nil
	}

	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("StoreId=", storeID).GetAll(&subs); err != nil {
		return false, "unavailable", err
	}
	for _, sub := range subs {
		slug := sub.PlanId
		if slug == "" {
			slug = sub.Plan.Slug
		}
		if slug != trial.PlanSlug {
			continue
		}
		switch sub.Status {
		case subscription.Trialing:
			if sub.TrialEnd.After(at) {
				return true, "trial", nil
			}
		case subscription.Active:
			if sub.PeriodEnd.After(at) {
				return true, "active", nil
			}
		}
	}
	return false, "payment_required", nil
}

func resolveStore(c *zip.Ctx, db *datastore.Datastore) (*storemodel.Store, error) {
	id := strings.TrimSpace(c.Header("X-Store-Id"))
	if id == "" {
		id = strings.TrimSpace(c.Param("storeid"))
	}
	if id != "" {
		s := storemodel.New(db)
		if err := s.GetById(id); err != nil {
			return nil, err
		}
		return s, nil
	}

	stores := make([]storemodel.Store, 0, 1)
	if _, err := storemodel.Query(db).Limit(1).GetAll(&stores); err != nil {
		return nil, err
	}
	if len(stores) == 0 {
		return nil, nil
	}
	s := storemodel.New(db)
	if err := s.GetById(stores[0].Id()); err != nil {
		return nil, err
	}
	return s, nil
}

func getAccess(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	s, err := resolveStore(c, db)
	if err != nil || s == nil {
		return c.JSON(http.StatusOK, accessState{Status: "store_required"})
	}
	allowed, status, err := HasAccess(db, s.Id(), time.Now())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, accessState{StoreID: s.Id(), Status: "unavailable"})
	}
	return c.JSON(http.StatusOK, accessState{Allowed: allowed, StoreID: s.Id(), Status: status})
}

func startTrial(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	s, err := resolveStore(c, db)
	if err != nil || s == nil {
		return c.JSON(http.StatusNotFound, accessState{Status: "store_required"})
	}
	result, err := trial.StartForStore(db, org.Name, s.Id(), false, org.TestMode())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, accessState{StoreID: s.Id(), Status: "unavailable"})
	}
	status := http.StatusOK
	if result.Started {
		status = http.StatusCreated
	}
	return c.JSON(status, result)
}

// RequireAccess protects merchant operations at the API boundary. Billing,
// store creation, trial activation, and access reads remain outside this gate.
func RequireAccess(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	s, err := resolveStore(c, db)
	if err != nil || s == nil {
		return c.JSON(http.StatusPaymentRequired, accessState{Status: "store_required"})
	}
	allowed, status, err := HasAccess(db, s.Id(), time.Now())
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, accessState{StoreID: s.Id(), Status: "unavailable"})
	}
	if !allowed {
		return c.JSON(http.StatusPaymentRequired, accessState{StoreID: s.Id(), Status: status})
	}
	return c.Next()
}
