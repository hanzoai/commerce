// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

// CanonicalVariant is the provider-neutral variant representation hashed by SyncLedger.
type CanonicalVariant struct {
	ExternalId string            `json:"externalId,omitempty"`
	SKU        string            `json:"sku"`
	Name       string            `json:"name"`
	PriceCents currency.Cents    `json:"priceCents"`
	MSRPCents  currency.Cents    `json:"msrpCents,omitempty"`
	Available  bool              `json:"available"`
	ManageStock bool             `json:"manageStock"`
	Inventory  int               `json:"inventory"`
	ImageURL   string            `json:"imageUrl,omitempty"`
	Barcode    string            `json:"barcode,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

// CanonicalProduct is the provider-neutral product representation.
type CanonicalProduct struct {
	CanonicalId string             `json:"canonicalId,omitempty"`
	ExternalId  string             `json:"externalId,omitempty"`
	Slug        string             `json:"slug"`
	SKU         string             `json:"sku"`
	UPC         string             `json:"upc,omitempty"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Currency    string             `json:"currency"`
	PriceCents  currency.Cents     `json:"priceCents"`
	Available   bool               `json:"available"`
	Hidden      bool               `json:"hidden"`
	ManageStock bool               `json:"manageStock"`
	Inventory   int                `json:"inventory"`
	Images      []string           `json:"images,omitempty"`
	Variants    []CanonicalVariant `json:"variants,omitempty"`
}

// CanonicalCollection is the provider-neutral collection representation.
type CanonicalCollection struct {
	CanonicalId string   `json:"canonicalId,omitempty"`
	ExternalId  string   `json:"externalId,omitempty"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Published   bool     `json:"published"`
	ProductSKUs []string `json:"productSkus,omitempty"`
}

// CanonicalCustomer is the provider-neutral customer & marketing consent representation.
type CanonicalCustomer struct {
	CanonicalId      string `json:"canonicalId,omitempty"`
	ExternalId       string `json:"externalId,omitempty"`
	Email            string `json:"email"`
	FirstName        string `json:"firstName,omitempty"`
	LastName         string `json:"lastName,omitempty"`
	MarketingConsent string `json:"marketingConsent,omitempty"` // "subscribed" | "unsubscribed" | "unknown"
	ConsentUpdatedAt string `json:"consentUpdatedAt,omitempty"`
}

// CanonicalOrderLine is one line item of a CanonicalOrder.
type CanonicalOrderLine struct {
	ExternalId        string         `json:"externalId,omitempty"`
	ExternalProductId string         `json:"externalProductId,omitempty"`
	ExternalVariantId string         `json:"externalVariantId,omitempty"`
	SKU               string         `json:"sku"`
	Name              string         `json:"name,omitempty"`
	Quantity          int            `json:"quantity"`
	UnitPriceCents    currency.Cents `json:"unitPriceCents"`
	TotalCents        currency.Cents `json:"totalCents"`
}

// CanonicalOrder is the provider-neutral order representation.
type CanonicalOrder struct {
	CanonicalId      string               `json:"canonicalId,omitempty"`
	ExternalId       string               `json:"externalId,omitempty"`
	Number           string               `json:"number,omitempty"`
	Status           string               `json:"status"` // "open" | "completed" | "cancelled" | "on-hold"
	Paid             bool                 `json:"paid"`
	Currency         string               `json:"currency"`
	TotalCents       currency.Cents       `json:"totalCents"`
	DiscountCents    currency.Cents       `json:"discountCents"`
	RefundedCents    currency.Cents       `json:"refundedCents"`
	CustomerExternal string               `json:"customerExternal,omitempty"`
	Email            string               `json:"email,omitempty"`
	CouponCodes      []string             `json:"couponCodes,omitempty"`
	Lines            []CanonicalOrderLine `json:"lines,omitempty"`
	MarketingConsent string               `json:"marketingConsent,omitempty"`
	ConsentUpdatedAt string               `json:"consentUpdatedAt,omitempty"`
}

// CanonicalCoupon is the provider-neutral coupon/discount representation.
type CanonicalCoupon struct {
	CanonicalId string `json:"canonicalId,omitempty"`
	ExternalId  string `json:"externalId,omitempty"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "flat" | "percent" | "free-shipping"
	Amount      int    `json:"amount"`
	Limit       int    `json:"limit,omitempty"`
	Once        bool   `json:"once,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// CanonicalFulfillment represents a fulfillment/tracking write-back.
type CanonicalFulfillment struct {
	OrderCanonicalId string `json:"orderCanonicalId,omitempty"`
	OrderExternalId  string `json:"orderExternalId,omitempty"`
	Carrier          string `json:"carrier,omitempty"`
	TrackingNumber   string `json:"trackingNumber"`
	TrackingURL      string `json:"trackingUrl,omitempty"`
	NotifyCustomer   bool   `json:"notifyCustomer,omitempty"`
}

// CanonicalRefund represents a refund write-back.
type CanonicalRefund struct {
	OrderCanonicalId string         `json:"orderCanonicalId,omitempty"`
	OrderExternalId  string         `json:"orderExternalId,omitempty"`
	AmountCents      currency.Cents `json:"amountCents"`
	Currency         string         `json:"currency,omitempty"`
	Note             string         `json:"note,omitempty"`
}

// CanonicalInventoryUpdate represents a master SKU stock set operation.
type CanonicalInventoryUpdate struct {
	SKU                string `json:"sku"`
	Quantity           int    `json:"quantity"`
	StockLocationId    string `json:"stockLocationId,omitempty"`
	ExternalLocationId string `json:"externalLocationId,omitempty"`
	ExternalVariantId  string `json:"externalVariantId,omitempty"`
}

// HashProduct computes a deterministic SHA-256 digest of a product's sync-relevant
// fields (excluding volatile external IDs and modified timestamps) so echoes of
// outbound writes produce the exact same hash when normalized back from Shopify or WooCommerce.
func HashProduct(p CanonicalProduct) string {
	type normVar struct {
		SKU       string         `json:"sku"`
		Price     currency.Cents `json:"price"`
		Available bool           `json:"available"`
		Manage    bool           `json:"manage"`
		Inventory int            `json:"inventory"`
	}
	vars := make([]normVar, 0, len(p.Variants))
	for _, v := range p.Variants {
		inv := v.Inventory
		if !v.ManageStock {
			inv = 0
		}
		vars = append(vars, normVar{
			SKU:       strings.TrimSpace(v.SKU),
			Price:     v.PriceCents,
			Available: v.Available,
			Manage:    v.ManageStock,
			Inventory: inv,
		})
	}
	sort.Slice(vars, func(i, j int) bool {
		if vars[i].SKU != vars[j].SKU {
			return vars[i].SKU < vars[j].SKU
		}
		return vars[i].Price < vars[j].Price
	})
	inv := p.Inventory
	if !p.ManageStock {
		inv = 0
	}
	norm := struct {
		SKU       string         `json:"sku"`
		Name      string         `json:"name"`
		Currency  string         `json:"currency"`
		Price     currency.Cents `json:"price"`
		Available bool           `json:"available"`
		Hidden    bool           `json:"hidden"`
		Manage    bool           `json:"manage"`
		Inventory int            `json:"inventory"`
		Variants  []normVar      `json:"variants"`
	}{
		SKU:       strings.TrimSpace(p.SKU),
		Name:      strings.TrimSpace(p.Name),
		Currency:  strings.ToLower(strings.TrimSpace(p.Currency)),
		Price:     p.PriceCents,
		Available: p.Available,
		Hidden:    p.Hidden,
		Manage:    p.ManageStock,
		Inventory: inv,
		Variants:  vars,
	}
	return sha256JSON(norm)
}

// HashCoupon computes a deterministic SHA-256 digest of a coupon's canonical state.
func HashCoupon(c CanonicalCoupon) string {
	norm := struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Amount  int    `json:"amount"`
		Limit   int    `json:"limit"`
		Enabled bool   `json:"enabled"`
	}{
		Code:    strings.ToUpper(strings.TrimSpace(c.Code)),
		Type:    strings.ToLower(strings.TrimSpace(c.Type)),
		Amount:  c.Amount,
		Limit:   c.Limit,
		Enabled: c.Enabled,
	}
	return sha256JSON(norm)
}

// HashInventory computes a deterministic SHA-256 digest for (SKU, Quantity).
func HashInventory(sku string, qty int) string {
	return sha256JSON(struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	}{
		SKU: strings.TrimSpace(sku),
		Qty: qty,
	})
}

// HashValue computes a deterministic SHA-256 hex digest of any JSON-marshalable struct.
func HashValue(v any) string { return sha256JSON(v) }

func sha256JSON(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func ledgerStorageID(sourceId, entityKind, canonicalId string) string {
	sum := sha256.Sum256([]byte(sourceId + "\x00" + entityKind + "\x00" + canonicalId))
	return "sld_" + hex.EncodeToString(sum[:16])
}

func extMapStorageID(sourceId, entityKind, externalId string) string {
	sum := sha256.Sum256([]byte(sourceId + "\x00" + entityKind + "\x00" + externalId))
	return "sem_" + hex.EncodeToString(sum[:16])
}

// ResolveCanonicalID looks up the canonical ID previously mapped to (sourceId, entityKind, externalId).
func ResolveCanonicalID(db *datastore.Datastore, sourceId, entityKind, externalId string) (string, bool) {
	if externalId == "" {
		return "", false
	}
	m := NewSyncExternalMap(db)
	id := extMapStorageID(sourceId, entityKind, externalId)
	if err := m.Get(db.NewKey(m.Kind(), id, 0, nil)); err != nil {
		return "", false
	}
	return m.CanonicalId, m.CanonicalId != ""
}

// GetLedger loads the SyncLedger row for (sourceId, entityKind, canonicalId), or nil if none exists.
func GetLedger(db *datastore.Datastore, sourceId, entityKind, canonicalId string) (*SyncLedger, error) {
	l := NewSyncLedger(db)
	id := ledgerStorageID(sourceId, entityKind, canonicalId)
	err := l.Get(db.NewKey(l.Kind(), id, 0, nil))
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

// CheckEcho reports whether incomingHash matches LastSyncedHash or PendingOutboundHash
// on (sourceId, entityKind, canonicalId). When matched, it promotes PendingOutboundHash
// to LastSyncedHash and returns true so the caller drops the echo with zero ping-pong.
func CheckEcho(db *datastore.Datastore, orgId, sourceId, entityKind, canonicalId, externalId, incomingHash string) (bool, error) {
	if canonicalId == "" || incomingHash == "" {
		return false, nil
	}
	l, err := GetLedger(db, sourceId, entityKind, canonicalId)
	if err != nil || l == nil {
		return false, err
	}
	if incomingHash != l.LastSyncedHash && incomingHash != l.PendingOutboundHash {
		return false, nil
	}
	l.SetId(ledgerStorageID(sourceId, entityKind, canonicalId))
	l.OrgId = orgId
	l.SourceId = sourceId
	l.EntityKind = entityKind
	l.CanonicalId = canonicalId
	if externalId != "" {
		l.ExternalId = externalId
	}
	l.LastSyncedHash = incomingHash
	if l.PendingOutboundHash == incomingHash {
		l.PendingOutboundHash = ""
	}
	_ = l.Put()
	return true, nil
}

// RecordInbound updates SyncLedger + SyncExternalMap after an inbound entity is committed.
func RecordInbound(db *datastore.Datastore, orgId, sourceId, entityKind, canonicalId, externalId, hash string) error {
	id := ledgerStorageID(sourceId, entityKind, canonicalId)
	l, err := GetLedger(db, sourceId, entityKind, canonicalId)
	if err != nil {
		return err
	}
	if l == nil {
		l = NewSyncLedger(db)
	}
	l.SetId(id)
	l.OrgId = orgId
	l.SourceId = sourceId
	l.EntityKind = entityKind
	l.CanonicalId = canonicalId
	if externalId != "" {
		l.ExternalId = externalId
	}
	l.LastSyncedHash = hash
	if l.PendingOutboundHash == hash {
		l.PendingOutboundHash = ""
	}
	if err := l.Put(); err != nil {
		return err
	}
	return bindExternalMap(db, orgId, sourceId, entityKind, canonicalId, l.ExternalId)
}

// RecordOutboundPending stamps PendingOutboundHash before an outbound push is sent,
// so even a racing webhook arriving before the HTTP response returns is dropped as an echo.
func RecordOutboundPending(db *datastore.Datastore, orgId, sourceId, entityKind, canonicalId, externalId, hash string) (*SyncLedger, error) {
	id := ledgerStorageID(sourceId, entityKind, canonicalId)
	l, err := GetLedger(db, sourceId, entityKind, canonicalId)
	if err != nil {
		return nil, err
	}
	if l == nil {
		l = NewSyncLedger(db)
	}
	l.SetId(id)
	l.OrgId = orgId
	l.SourceId = sourceId
	l.EntityKind = entityKind
	l.CanonicalId = canonicalId
	if externalId != "" {
		l.ExternalId = externalId
	}
	l.PendingOutboundHash = hash
	if err := l.Put(); err != nil {
		return nil, err
	}
	return l, nil
}

// RecordOutboundApplied commits LastSyncedHash and binds any newly returned ExternalId.
func RecordOutboundApplied(db *datastore.Datastore, orgId, sourceId, entityKind, canonicalId, externalId, hash string) error {
	id := ledgerStorageID(sourceId, entityKind, canonicalId)
	l, err := GetLedger(db, sourceId, entityKind, canonicalId)
	if err != nil {
		return err
	}
	if l == nil {
		l = NewSyncLedger(db)
	}
	l.SetId(id)
	l.OrgId = orgId
	l.SourceId = sourceId
	l.EntityKind = entityKind
	l.CanonicalId = canonicalId
	if externalId != "" {
		l.ExternalId = externalId
	}
	l.LastSyncedHash = hash
	if l.PendingOutboundHash == hash {
		l.PendingOutboundHash = ""
	}
	if err := l.Put(); err != nil {
		return err
	}
	return bindExternalMap(db, orgId, sourceId, entityKind, canonicalId, l.ExternalId)
}

func bindExternalMap(db *datastore.Datastore, orgId, sourceId, entityKind, canonicalId, externalId string) error {
	if externalId == "" || canonicalId == "" {
		return nil
	}
	m := NewSyncExternalMap(db)
	m.SetId(extMapStorageID(sourceId, entityKind, externalId))
	m.OrgId = orgId
	m.SourceId = sourceId
	m.EntityKind = entityKind
	m.ExternalId = externalId
	m.CanonicalId = canonicalId
	return m.Put()
}
