// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/inventory"
	"github.com/hanzoai/commerce/models/inventorylevel"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/fulfillment"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/types"
)

// IngestCoupon upserts a CanonicalCoupon and fans out to connected stores.
func (e *Engine) IngestCoupon(ctx context.Context, orgId, originSourceId string, in CanonicalCoupon) (*coupon.Coupon, bool, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, false, err
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return nil, false, errors.New("source: coupon code is required")
	}
	in.Code = code
	if in.Name == "" {
		in.Name = code
	}
	if in.Type == "" {
		in.Type = string(coupon.Percent)
	}
	hash := HashCoupon(in)

	cid := in.CanonicalId
	if cid == "" && originSourceId != "" && in.ExternalId != "" {
		cid, _ = ResolveCanonicalID(orgDB, originSourceId, EntityCoupon, in.ExternalId)
	}
	cpn := findCoupon(orgDB, cid, code)
	if cpn != nil {
		cid = cpn.Id()
	}
	if originSourceId != "" && cid != "" {
		if echo, _ := CheckEcho(orgDB, orgId, originSourceId, EntityCoupon, cid, in.ExternalId, hash); echo {
			return cpn, true, nil
		}
	}
	isNew := false
	if cpn == nil {
		cpn = coupon.New(orgDB)
		isNew = true
	} else {
		cpn.Init(orgDB)
	}
	cpn.Code_ = code
	cpn.Name = in.Name
	cpn.Type = coupon.Type(in.Type)
	cpn.Amount = in.Amount
	cpn.Limit = in.Limit
	cpn.Once = in.Once
	cpn.Enabled = in.Enabled
	if isNew {
		if err := cpn.Create(); err != nil {
			return nil, false, err
		}
	} else {
		if err := cpn.Put(); err != nil {
			return nil, false, err
		}
	}
	in.CanonicalId = cpn.Id()
	if originSourceId != "" {
		_ = RecordInbound(orgDB, orgId, originSourceId, EntityCoupon, cpn.Id(), in.ExternalId, hash)
	}
	if err := e.FanOutCoupon(ctx, orgId, originSourceId, in); err != nil {
		return cpn, false, err
	}
	return cpn, false, nil
}

// FanOutCoupon dispatches coupon.upsert to all publish-enabled sibling stores.
func (e *Engine) FanOutCoupon(ctx context.Context, orgId, originSourceId string, cc CanonicalCoupon) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashCoupon(cc)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		extID := ""
		if l, _ := GetLedger(orgDB, dst.SourceId, EntityCoupon, cc.CanonicalId); l != nil {
			if l.LastSyncedHash == hash {
				continue
			}
			extID = l.ExternalId
		}
		out := cc
		out.ExternalId = extID
		_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityCoupon, cc.CanonicalId, extID, hash)
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpCouponUpsert, EntityCoupon, cc.CanonicalId, extID, "", out, hash)
	}
	return nil
}

// IngestOrder upserts a CanonicalOrder, records guest consent when present, and on a
// paid order atomically decrements master SKU inventory and pushes updated stock to
// every other connected store in the org.
func (e *Engine) IngestOrder(ctx context.Context, orgId, originSourceId string, in CanonicalOrder) (*order.Order, bool, error) {
	var origin *StoreSource
	if originSourceId != "" {
		var err error
		origin, err = GetSource(ctx, orgId, originSourceId)
		if err != nil {
			return nil, false, err
		}
		if !canIngest(origin) {
			return nil, true, nil
		}
	}
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, false, err
	}

	hash := HashValue(struct {
		Status   string         `json:"status"`
		Paid     bool           `json:"paid"`
		Total    currency.Cents `json:"total"`
		Refunded currency.Cents `json:"refunded"`
	}{in.Status, in.Paid, in.TotalCents, in.RefundedCents})

	cid := in.CanonicalId
	if cid == "" && originSourceId != "" && in.ExternalId != "" {
		cid, _ = ResolveCanonicalID(orgDB, originSourceId, EntityOrder, in.ExternalId)
	}
	var ord *order.Order
	if cid != "" {
		o := order.New(orgDB)
		if err := o.GetById(cid); err == nil {
			ord = o
		}
	}
	if originSourceId != "" && ord != nil {
		if echo, _ := CheckEcho(orgDB, orgId, originSourceId, EntityOrder, ord.Id(), in.ExternalId, hash); echo {
			return ord, true, nil
		}
	}

	isNew := false
	if ord == nil {
		ord = order.New(orgDB)
		isNew = true
	} else {
		ord.Init(orgDB)
	}
	ord.StoreId = originSourceId
	if in.Email != "" {
		ord.Email = strings.ToLower(strings.TrimSpace(in.Email))
	}
	if in.Currency == "" && origin != nil && origin.Currency != "" {
		in.Currency = origin.Currency
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	ord.Currency = currency.Type(strings.ToLower(in.Currency))
	ord.Total = in.TotalCents
	ord.Discount = in.DiscountCents
	ord.Subtotal = in.TotalCents
	ord.Refunded = in.RefundedCents
	ord.CouponCodes = in.CouponCodes
	switch strings.ToLower(in.Status) {
	case "completed":
		ord.Status = order.Completed
	case "cancelled", "canceled":
		ord.Status = order.Cancelled
	case "on-hold":
		ord.Status = order.OnHold
	default:
		ord.Status = order.Open
	}
	if in.Paid {
		ord.Paid = in.TotalCents
		ord.PaymentStatus = payment.Paid
	}
	items := make([]lineitem.LineItem, 0, len(in.Lines))
	for _, ln := range in.Lines {
		li := lineitem.LineItem{
			VariantSKU:  ln.SKU,
			ProductSKU:  ln.SKU,
			ExternalSKU: ln.SKU,
			ProductName: ln.Name,
			VariantName: ln.Name,
			Quantity:    ln.Quantity,
		}
		li.Currency = ord.Currency
		li.Price = ln.UnitPriceCents
		items = append(items, li)
	}
	ord.Items = items
	if ord.Metadata == nil {
		ord.Metadata = make(types.Map)
	}
	if in.ExternalId != "" {
		ord.Metadata["external_id"] = in.ExternalId
	}
	if in.Number != "" {
		ord.Metadata["external_number"] = in.Number
	}
	if in.CustomerExternal == "" || in.CustomerExternal == "0" {
		if in.ExternalId != "" {
			ord.Metadata["guest_key"] = "guest:" + in.ExternalId
		}
	}
	if in.MarketingConsent != "" {
		ord.Metadata["marketing_consent"] = in.MarketingConsent
	}
	if in.ConsentUpdatedAt != "" {
		ord.Metadata["consent_updated_at"] = in.ConsentUpdatedAt
	}

	if isNew {
		if err := ord.Create(); err != nil {
			return nil, false, err
		}
	} else {
		if err := ord.Put(); err != nil {
			return nil, false, err
		}
	}

	if originSourceId != "" {
		_ = RecordInbound(orgDB, orgId, originSourceId, EntityOrder, ord.Id(), in.ExternalId, hash)
		origin.OrdersRead++
		origin.LastSyncedAt = time.Now().UTC()
		_ = origin.Put()
	}

	// Master SKU cross-store inventory sync: decrement SKU stock ONCE per paid order
	// and push the resulting quantity to every sibling store in the org!
	if in.Paid && len(in.Lines) > 0 {
		guardKey := ord.Id()
		if originSourceId != "" && in.ExternalId != "" {
			guardKey = originSourceId + ":" + in.ExternalId
		}
		rec, replay, gerr := idempotencykey.Begin(orgDB, "source-order-stock", guardKey)
		if gerr == nil && !replay {
			for _, ln := range in.Lines {
				if strings.TrimSpace(ln.SKU) == "" || ln.Quantity <= 0 {
					continue
				}
				_, _ = e.AdjustSKUInventory(ctx, orgId, originSourceId, ln.SKU, -ln.Quantity)
			}
			_ = idempotencykey.Complete(rec, "applied")
		}
	}

	return ord, false, nil
}

// AdjustSKUInventory atomically adjusts canonical stock for master SKU by delta and
// fans out inventory.set (with echo-suppression hashes) to all connected stores.
func (e *Engine) AdjustSKUInventory(ctx context.Context, orgId, originSourceId, sku string, delta int) (int, error) {
	e.skuMu.Lock()
	defer e.skuMu.Unlock()

	sku = strings.TrimSpace(sku)
	if sku == "" {
		return 0, errors.New("source: sku is required")
	}
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return 0, err
	}

	newQty, touchedProducts := e.applySKUDelta(orgDB, sku, delta, false, 0)
	if err := e.fanOutSKU(ctx, orgDB, orgId, originSourceId, sku, newQty, touchedProducts); err != nil {
		return newQty, err
	}
	return newQty, nil
}

// SetSKUInventory sets canonical stock for master SKU to an exact quantity and fans out.
func (e *Engine) SetSKUInventory(ctx context.Context, orgId, originSourceId, sku string, qty int) (int, error) {
	e.skuMu.Lock()
	defer e.skuMu.Unlock()

	sku = strings.TrimSpace(sku)
	if sku == "" {
		return 0, errors.New("source: sku is required")
	}
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return 0, err
	}
	if qty < 0 {
		qty = 0
	}
	newQty, touchedProducts := e.applySKUDelta(orgDB, sku, 0, true, qty)
	if err := e.fanOutSKU(ctx, orgDB, orgId, originSourceId, sku, newQty, touchedProducts); err != nil {
		return newQty, err
	}
	return newQty, nil
}

func (e *Engine) applySKUDelta(orgDB *datastore.Datastore, sku string, delta int, absolute bool, absQty int) (int, []*product.Product) {
	newQty := absQty
	foundQty := false

	items := make([]*inventory.InventoryItem, 0)
	_, _ = orgDB.Query("inventoryitem").Filter("SKU=", sku).GetAll(&items)
	for _, it := range items {
		it.Init(orgDB)
		levels := make([]*inventorylevel.InventoryLevel, 0)
		_, _ = orgDB.Query("inventorylevel").Filter("InventoryItemId=", it.Id()).GetAll(&levels)
		for _, lv := range levels {
			lv.Init(orgDB)
			_ = lv.SetKey(lv.Id_)
			if absolute {
				lv.StockedQuantity = absQty
			} else {
				lv.StockedQuantity += delta
				if lv.StockedQuantity < 0 {
					lv.StockedQuantity = 0
				}
			}
			if !foundQty {
				newQty = lv.StockedQuantity
				foundQty = true
			}
			_ = lv.Put()
		}
	}

	vars := make([]*variant.Variant, 0)
	_, _ = orgDB.Query("variant").Filter("SKU=", sku).GetAll(&vars)
	for _, v := range vars {
		v.Init(orgDB)
		_ = v.SetKey(v.Id_)
		if absolute {
			v.Inventory = absQty
		} else if foundQty {
			v.Inventory = newQty
		} else {
			v.Inventory += delta
			if v.Inventory < 0 {
				v.Inventory = 0
			}
			newQty = v.Inventory
			foundQty = true
		}
		_ = v.Put()
	}

	prods := make([]*product.Product, 0)
	_, _ = orgDB.Query("product").GetAll(&prods)
	touched := make([]*product.Product, 0)
	for _, p := range prods {
		if p == nil {
			continue
		}
		changed := false
		if p.SKU == sku && len(p.Variants) == 0 {
			if absolute {
				p.Inventory = absQty
			} else if foundQty {
				p.Inventory = newQty
			} else {
				p.Inventory += delta
				if p.Inventory < 0 {
					p.Inventory = 0
				}
				newQty = p.Inventory
				foundQty = true
			}
			changed = true
		}
		for _, pv := range p.Variants {
			if pv != nil && pv.SKU == sku {
				if absolute {
					pv.Inventory = absQty
				} else if foundQty {
					pv.Inventory = newQty
				} else {
					pv.Inventory += delta
					if pv.Inventory < 0 {
						pv.Inventory = 0
					}
					newQty = pv.Inventory
					foundQty = true
				}
				changed = true
			}
		}
		if changed {
			bound := product.New(orgDB)
			if err := bound.GetById(p.Id_); err == nil {
				sumInv := 0
				for i := range bound.Variants {
					if bound.Variants[i] != nil {
						if bound.Variants[i].SKU == sku {
							bound.Variants[i].Inventory = newQty
							bound.Variants[i].Available = newQty > 0
						}
						sumInv += bound.Variants[i].Inventory
					}
				}
				if len(bound.Variants) > 0 {
					bound.Inventory = sumInv
				} else {
					bound.Inventory = newQty
				}
				_ = bound.Put()
				touched = append(touched, bound)
			}
		}
	}
	if !foundQty && !absolute {
		if delta > 0 {
			newQty = delta
		} else {
			newQty = 0
		}
	}
	return newQty, touched
}

func (e *Engine) fanOutSKU(ctx context.Context, orgDB *datastore.Datastore, orgId, originSourceId, sku string, qty int, touched []*product.Product) error {
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	invHash := HashInventory(sku, qty)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		extVarID := ""
		if l, _ := GetLedger(orgDB, dst.SourceId, EntityInventory, sku); l != nil {
			if l.LastSyncedHash == invHash {
				continue
			}
			extVarID = l.ExternalId
		}
		_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityInventory, sku, extVarID, invHash)
		for _, tp := range touched {
			pHash := HashProduct(CanonicalFromProduct(tp, true, nil))
			extProdID := ""
			if pl, _ := GetLedger(orgDB, dst.SourceId, EntityProduct, tp.Id()); pl != nil {
				extProdID = pl.ExternalId
			}
			_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityProduct, tp.Id(), extProdID, pHash)
		}
		upd := CanonicalInventoryUpdate{
			SKU:                sku,
			Quantity:           qty,
			StockLocationId:    dst.StockLocationId,
			ExternalLocationId: dst.ExternalLocationId,
			ExternalVariantId:  extVarID,
		}
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpInventorySet, EntityInventory, sku, extVarID, sku, upd, invHash)
	}
	return nil
}

// PublishFulfillment records tracking on the canonical Order and pushes fulfillment.create
// to the store(s) holding the external order.
func (e *Engine) PublishFulfillment(ctx context.Context, orgId, originSourceId string, in CanonicalFulfillment) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	if in.OrderCanonicalId != "" {
		ord := order.New(orgDB)
		if err := ord.GetById(in.OrderCanonicalId); err == nil {
			ord.Fulfillment.Status = fulfillment.Tracked
			ord.Fulfillment.Carrier = in.Carrier
			ord.Fulfillment.Trackings = append(ord.Fulfillment.Trackings, fulfillment.Tracking{
				Number:    in.TrackingNumber,
				Carrier:   in.Carrier,
				Url:       in.TrackingURL,
				CreatedAt: time.Now().UTC(),
			})
			_ = ord.Put()
		}
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashValue(in)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		extOrderID := in.OrderExternalId
		if extOrderID == "" && in.OrderCanonicalId != "" {
			if l, _ := GetLedger(orgDB, dst.SourceId, EntityOrder, in.OrderCanonicalId); l != nil {
				extOrderID = l.ExternalId
			}
		}
		if extOrderID == "" {
			continue
		}
		out := in
		out.OrderExternalId = extOrderID
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpFulfillmentCreate, EntityFulfillment, in.OrderCanonicalId, extOrderID, "", out, hash)
	}
	return nil
}

// PublishRefund records a refund and pushes refund.create to the target store(s).
func (e *Engine) PublishRefund(ctx context.Context, orgId, originSourceId string, in CanonicalRefund) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	if in.OrderCanonicalId != "" {
		ord := order.New(orgDB)
		if err := ord.GetById(in.OrderCanonicalId); err == nil {
			ord.Refunded += in.AmountCents
			_ = ord.Put()
		}
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashValue(in)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		extOrderID := in.OrderExternalId
		if extOrderID == "" && in.OrderCanonicalId != "" {
			if l, _ := GetLedger(orgDB, dst.SourceId, EntityOrder, in.OrderCanonicalId); l != nil {
				extOrderID = l.ExternalId
			}
		}
		if extOrderID == "" {
			continue
		}
		out := in
		out.OrderExternalId = extOrderID
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpRefundCreate, EntityRefund, in.OrderCanonicalId, extOrderID, "", out, hash)
	}
	return nil
}
