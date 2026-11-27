// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/collection"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/inventory"
	"github.com/hanzoai/commerce/models/inventorylevel"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/models/variant"
)

// CanonicalCampaignActivity represents an external marketing campaign activity.
type CanonicalCampaignActivity struct {
	CampaignId string `json:"campaignId"`
	Title      string `json:"title"`
	Channel    string `json:"channel,omitempty"`
	URL        string `json:"url,omitempty"`
	CustomerId string `json:"customerId,omitempty"`
}

// PublishCampaignActivity pushes a Hanzo campaign activity (and optional Flow trigger) to sibling stores.
func (e *Engine) PublishCampaignActivity(ctx context.Context, orgId, originSourceId string, act CanonicalCampaignActivity) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashValue(act)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpCampaignActivity, EntityCampaign, act.CampaignId, "", "", act, hash)
	}
	return nil
}

// DeleteExternalEntity archives/deletes the canonical entity mapped to (sourceId, entityKind, externalId).
func (e *Engine) DeleteExternalEntity(ctx context.Context, orgId, originSourceId, entityKind, externalId string) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	cid, ok := ResolveCanonicalID(orgDB, originSourceId, entityKind, externalId)
	if !ok || cid == "" {
		return nil
	}
	switch entityKind {
	case EntityProduct:
		p := product.New(orgDB)
		if err := p.GetById(cid); err == nil {
			p.Available = false
			p.Hidden = true
			_ = p.Put()
		}
	case EntityOrder:
		o := order.New(orgDB)
		if err := o.GetById(cid); err == nil {
			o.Status = order.Cancelled
			_ = o.Put()
		}
	case EntityCustomer:
		u := user.New(orgDB)
		if err := u.GetById(cid); err == nil {
			_ = u.Delete()
		}
	}
	return nil
}

// EraseCustomer scrubs PII and consent for a customer external ID ("25", "guest:1027") or email.
func (e *Engine) EraseCustomer(ctx context.Context, orgId, sourceId, idOrGuest, email string) (int, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return 0, err
	}
	erased := 0
	email = strings.ToLower(strings.TrimSpace(email))

	if strings.HasPrefix(idOrGuest, "guest:") {
		extOrderID := strings.TrimPrefix(idOrGuest, "guest:")
		if cid, ok := ResolveCanonicalID(orgDB, sourceId, EntityOrder, extOrderID); ok {
			o := order.New(orgDB)
			if err := o.GetById(cid); err == nil {
				o.Email = ""
				if o.Metadata != nil {
					delete(o.Metadata, "marketing_consent")
					delete(o.Metadata, "consent_updated_at")
					o.Metadata["erased"] = true
				}
				_ = o.Put()
				erased++
			}
		}
	} else if idOrGuest != "" && sourceId != "" {
		if cid, ok := ResolveCanonicalID(orgDB, sourceId, EntityCustomer, idOrGuest); ok {
			u := user.New(orgDB)
			if err := u.GetById(cid); err == nil {
				if email == "" {
					email = u.Email
				}
				_ = u.Delete()
				erased++
			}
		}
	}

	if email != "" {
		users := make([]*user.User, 0)
		_, _ = orgDB.Query("user").Filter("Email=", email).GetAll(&users)
		for _, r := range users {
			u := user.New(orgDB)
			if err := u.GetById(r.Id_); err == nil {
				_ = u.Delete()
				erased++
			}
		}
		orders := make([]*order.Order, 0)
		_, _ = orgDB.Query("order").Filter("Email=", email).GetAll(&orders)
		for _, r := range orders {
			o := order.New(orgDB)
			if err := o.GetById(r.Id_); err == nil {
				o.Email = ""
				if o.Metadata != nil {
					delete(o.Metadata, "marketing_consent")
					o.Metadata["erased"] = true
				}
				_ = o.Put()
				erased++
			}
		}
	}
	return erased, nil
}

// ExportCustomer gathers all synced customer + order records for a GDPR data request.
func (e *Engine) ExportCustomer(ctx context.Context, orgId, sourceId, idOrGuest, email string) (map[string]any, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	outCust := make([]map[string]any, 0)
	outOrders := make([]map[string]any, 0)

	if idOrGuest != "" && sourceId != "" && !strings.HasPrefix(idOrGuest, "guest:") {
		if cid, ok := ResolveCanonicalID(orgDB, sourceId, EntityCustomer, idOrGuest); ok {
			u := user.New(orgDB)
			if err := u.GetById(cid); err == nil {
				if email == "" {
					email = u.Email
				}
				outCust = append(outCust, map[string]any{
					"id":       u.Id(),
					"email":    u.Email,
					"metadata": u.Metadata,
				})
			}
		}
	}
	if email != "" && len(outCust) == 0 {
		users := make([]*user.User, 0)
		_, _ = orgDB.Query("user").Filter("Email=", email).GetAll(&users)
		for _, u := range users {
			outCust = append(outCust, map[string]any{
				"id":       u.Id_,
				"email":    u.Email,
				"metadata": u.Metadata,
			})
		}
	}
	if email != "" {
		orders := make([]*order.Order, 0)
		_, _ = orgDB.Query("order").Filter("Email=", email).GetAll(&orders)
		for _, o := range orders {
			outOrders = append(outOrders, map[string]any{
				"id":       o.Id_,
				"total":    o.Total,
				"currency": o.Currency,
				"status":   o.Status,
			})
		}
	}
	if strings.HasPrefix(idOrGuest, "guest:") && sourceId != "" {
		extOrderID := strings.TrimPrefix(idOrGuest, "guest:")
		if cid, ok := ResolveCanonicalID(orgDB, sourceId, EntityOrder, extOrderID); ok {
			o := order.New(orgDB)
			if err := o.GetById(cid); err == nil {
				outOrders = append(outOrders, map[string]any{
					"id":       o.Id(),
					"total":    o.Total,
					"currency": o.Currency,
					"status":   o.Status,
				})
			}
		}
	}
	return map[string]any{
		"org":       orgId,
		"customers": outCust,
		"orders":    outOrders,
	}, nil
}

// ReconcileSource pushes all canonical products & coupons in orgId to a publish-enabled StoreSource.
func (e *Engine) ReconcileSource(ctx context.Context, orgId, sourceId string) (map[string]int, error) {
	src, err := GetSource(ctx, orgId, sourceId)
	if err != nil {
		return nil, err
	}
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{"products": 0, "coupons": 0}
	if !canPublish(src) {
		return counts, nil
	}
	prods := make([]*product.Product, 0)
	_, _ = orgDB.Query("product").GetAll(&prods)
	for _, p := range prods {
		bound := product.New(orgDB)
		if err := bound.GetById(p.Id_); err != nil {
			continue
		}
		cp := CanonicalFromProduct(bound, true, nil)
		hash := HashProduct(cp)
		extID := ""
		if l, _ := GetLedger(orgDB, src.SourceId, EntityProduct, cp.CanonicalId); l != nil {
			if l.LastSyncedHash == hash {
				continue
			}
			extID = l.ExternalId
		}
		cp.ExternalId = extID
		_, _ = RecordOutboundPending(orgDB, orgId, src.SourceId, EntityProduct, cp.CanonicalId, extID, hash)
		if err := e.enqueueAndDeliver(ctx, orgDB, src, OpProductUpsert, EntityProduct, cp.CanonicalId, extID, cp.SKU, cp, hash); err == nil {
			counts["products"]++
		}
	}
	cpns := make([]*coupon.Coupon, 0)
	_, _ = orgDB.Query("coupon").GetAll(&cpns)
	for _, c := range cpns {
		cc := CanonicalCoupon{
			CanonicalId: c.Id_,
			Code:        c.Code_,
			Name:        c.Name,
			Type:        string(c.Type),
			Amount:      c.Amount,
			Limit:       c.Limit,
			Once:        c.Once,
			Enabled:     c.Enabled,
		}
		hash := HashCoupon(cc)
		extID := ""
		if l, _ := GetLedger(orgDB, src.SourceId, EntityCoupon, cc.CanonicalId); l != nil {
			if l.LastSyncedHash == hash {
				continue
			}
			extID = l.ExternalId
		}
		cc.ExternalId = extID
		_, _ = RecordOutboundPending(orgDB, orgId, src.SourceId, EntityCoupon, cc.CanonicalId, extID, hash)
		if err := e.enqueueAndDeliver(ctx, orgDB, src, OpCouponUpsert, EntityCoupon, cc.CanonicalId, extID, "", cc, hash); err == nil {
			counts["coupons"]++
		}
	}
	src.LastBackfillStatus = "COMPLETED"
	src.LastSyncedAt = time.Now().UTC()
	_ = src.Put()
	return counts, nil
}

func (e *Engine) enqueueAndDeliver(ctx context.Context, orgDB *datastore.Datastore, dst *StoreSource, op, entityKind, canonicalID, extID, sku string, payload any, hash string) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	job := NewOutboundJob(orgDB)
	job.JobId = NewJobID()
	job.SetId(job.JobId)
	job.OrgId = dst.OrgId
	job.SourceId = dst.SourceId
	job.Provider = dst.Provider
	job.Operation = op
	job.EntityKind = entityKind
	job.CanonicalId = canonicalID
	job.ExternalId = extID
	job.SKU = sku
	job.Payload = string(raw)
	job.PayloadHash = hash
	job.Status = JobPending
	if err := job.Put(); err != nil {
		return err
	}

	drv := e.driverFor(dst.Provider)
	if drv == nil {
		return nil
	}
	creds, err := OpenCredentials(ctx, dst.OrgId, dst.SourceId, dst.SealedCredential)
	if err != nil {
		return err
	}
	job.Attempts++
	res, err := drv.Deliver(ctx, dst, creds, job)
	if err != nil {
		job.Status = JobFailed
		job.LastError = err.Error()
		_ = job.Put()
		return err
	}
	if res.Queued {
		job.Status = JobPending
		_ = job.Put()
		return nil
	}
	return MarkJobApplied(ctx, orgDB, dst, job, res.ExternalId)
}

// MarkJobApplied marks an OutboundJob applied, commits LastSyncedHash in SyncLedger,
// and increments the StoreSource's outbound counters.
func MarkJobApplied(ctx context.Context, orgDB *datastore.Datastore, dst *StoreSource, job *OutboundJob, externalId string) error {
	_ = ctx
	if externalId != "" {
		job.ExternalId = externalId
	}
	job.Status = JobApplied
	job.LastError = ""
	job.SetId(job.JobId)
	if err := job.Put(); err != nil {
		return err
	}
	if job.CanonicalId != "" {
		_ = RecordOutboundApplied(orgDB, dst.OrgId, dst.SourceId, job.EntityKind, job.CanonicalId, job.ExternalId, job.PayloadHash)
	}
	switch job.Operation {
	case OpProductUpsert, OpProductDelete, OpCollectionUpsert:
		dst.ProductsWritten++
	case OpInventorySet:
		dst.InventoryWritten++
	case OpCouponUpsert:
		dst.CouponsWritten++
	case OpFulfillmentCreate, OpRefundCreate:
		dst.OrdersWritten++
	case OpCustomerConsent, OpCampaignActivity:
		dst.CustomersWritten++
	}
	dst.LastSyncedAt = time.Now().UTC()
	dst.LastError = ""
	dst.SetId(StoreSourceStorageID(dst.SourceId))
	return dst.Put()
}

func (e *Engine) upsertCanonicalInventory(orgDB *datastore.Datastore, sku, title, locationID string, qty int) {
	items := make([]*inventory.InventoryItem, 0)
	_, _ = orgDB.Query("inventoryitem").Filter("SKU=", sku).GetAll(&items)
	var item *inventory.InventoryItem
	if len(items) > 0 {
		item = inventory.New(orgDB)
		if err := item.GetById(items[0].Id_); err != nil {
			item = nil
		}
	}
	if item == nil {
		item = inventory.New(orgDB)
		item.SKU = sku
		item.Title = title
		_ = item.Create()
	}
	if locationID == "" {
		locationID = "default"
	}
	levels := make([]*inventorylevel.InventoryLevel, 0)
	_, _ = orgDB.Query("inventorylevel").Filter("InventoryItemId=", item.Id()).Filter("LocationId=", locationID).GetAll(&levels)
	if len(levels) > 0 {
		lv := inventorylevel.New(orgDB)
		if err := lv.GetById(levels[0].Id_); err == nil {
			lv.StockedQuantity = qty
			_ = lv.Put()
			return
		}
	}
	lv := inventorylevel.New(orgDB)
	lv.InventoryItemId = item.Id()
	lv.LocationId = locationID
	lv.StockedQuantity = qty
	_ = lv.Create()
}

func findProduct(orgDB *datastore.Datastore, canonicalID, sku, slug string) *product.Product {
	if canonicalID != "" {
		p := product.New(orgDB)
		if err := p.GetById(canonicalID); err == nil {
			return p
		}
	}
	if sku != "" {
		rows := make([]*product.Product, 0)
		_, _ = orgDB.Query("product").Filter("SKU=", sku).GetAll(&rows)
		if len(rows) > 0 {
			p := product.New(orgDB)
			if err := p.GetById(rows[0].Id_); err == nil {
				return p
			}
		}
	}
	if slug != "" {
		rows := make([]*product.Product, 0)
		_, _ = orgDB.Query("product").Filter("Slug=", slug).GetAll(&rows)
		if len(rows) > 0 {
			p := product.New(orgDB)
			if err := p.GetById(rows[0].Id_); err == nil {
				return p
			}
		}
	}
	return nil
}

func findOrNewVariant(orgDB *datastore.Datastore, prodID, sku string) *variant.Variant {
	rows := make([]*variant.Variant, 0)
	_, _ = orgDB.Query("variant").Filter("SKU=", sku).GetAll(&rows)
	for _, r := range rows {
		if r.ProductId == prodID || prodID == "" {
			v := variant.New(orgDB)
			if err := v.GetById(r.Id_); err == nil {
				return v
			}
		}
	}
	return variant.New(orgDB)
}

func findCollection(orgDB *datastore.Datastore, canonicalID, slug string) *collection.Collection {
	if canonicalID != "" {
		c := collection.New(orgDB)
		if err := c.GetById(canonicalID); err == nil {
			return c
		}
	}
	if slug != "" {
		rows := make([]*collection.Collection, 0)
		_, _ = orgDB.Query("collection").Filter("Slug=", slug).GetAll(&rows)
		if len(rows) > 0 {
			c := collection.New(orgDB)
			if err := c.GetById(rows[0].Id_); err == nil {
				return c
			}
		}
	}
	return nil
}

func findCustomer(orgDB *datastore.Datastore, canonicalID, email string) *user.User {
	if canonicalID != "" {
		u := user.New(orgDB)
		if err := u.GetById(canonicalID); err == nil {
			return u
		}
	}
	if email != "" {
		rows := make([]*user.User, 0)
		_, _ = orgDB.Query("user").Filter("Email=", email).GetAll(&rows)
		if len(rows) > 0 {
			u := user.New(orgDB)
			if err := u.GetById(rows[0].Id_); err == nil {
				return u
			}
		}
	}
	return nil
}

func findCoupon(orgDB *datastore.Datastore, canonicalID, code string) *coupon.Coupon {
	if canonicalID != "" {
		c := coupon.New(orgDB)
		if err := c.GetById(canonicalID); err == nil {
			return c
		}
	}
	if code != "" {
		rows := make([]*coupon.Coupon, 0)
		_, _ = orgDB.Query("coupon").Filter("Code_=", code).GetAll(&rows)
		if len(rows) > 0 {
			c := coupon.New(orgDB)
			if err := c.GetById(rows[0].Id_); err == nil {
				return c
			}
		}
	}
	return nil
}
