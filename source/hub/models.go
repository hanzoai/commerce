// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package hub implements the multi-store StoreSource registry (1 Org : N
// Connected Stores), loop-free bidirectional SyncLedger, master SKU cross-store
// inventory unification, and outbound sync job queue for Hanzo Commerce.
package hub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/secret"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[StoreSource]("store-source", orm.WithStringKey[StoreSource]())
	orm.Register[StoreSourceLookup]("store-source-lookup", orm.WithStringKey[StoreSourceLookup]())
	orm.Register[SyncLedger]("source-sync-ledger", orm.WithStringKey[SyncLedger]())
	orm.Register[SyncExternalMap]("source-external-map", orm.WithStringKey[SyncExternalMap]())
	orm.Register[OutboundJob]("source-outbound-job", orm.WithStringKey[OutboundJob]())
}

// Provider kinds.
const (
	ProviderShopify     = "shopify"
	ProviderWooCommerce = "woocommerce"
	ProviderHanzo       = "hanzo"
)

// SyncMode controls directional sync per connected StoreSource.
const (
	SyncBidirectional = "bidirectional"
	SyncIngestOnly    = "ingest_only"
	SyncPublishOnly   = "publish_only"
)

// Source lifecycle statuses.
const (
	StatusActive       = "active"
	StatusUnlinked     = "unlinked"
	StatusDisconnected = "disconnected"
	StatusUninstalled  = "uninstalled"
)

// Entity kinds tracked in SyncLedger.
const (
	EntityProduct     = "product"
	EntityCollection  = "collection"
	EntityCustomer    = "customer"
	EntityOrder       = "order"
	EntityCoupon      = "coupon"
	EntityInventory   = "inventory"
	EntityFulfillment = "fulfillment"
	EntityRefund      = "refund"
	EntityConsent     = "consent"
	EntityCampaign    = "campaign"
)

// Outbound operation names.
const (
	OpProductUpsert    = "product.upsert"
	OpProductDelete    = "product.delete"
	OpCollectionUpsert = "collection.upsert"
	OpInventorySet     = "inventory.set"
	OpCouponUpsert     = "coupon.upsert"
	OpFulfillmentCreate = "fulfillment.create"
	OpRefundCreate     = "refund.create"
	OpCustomerConsent  = "customer.consent"
	OpCampaignActivity = "campaign.activity"
)

// Outbound job statuses.
const (
	JobPending = "pending"
	JobApplied = "applied"
	JobFailed  = "failed"
)

var (
	ErrSourceConflict = errors.New("source: shop is already connected to another org")
	ErrSourceNotFound = errors.New("source: store source not found")
	ErrBadOrg         = errors.New("source: invalid organization")
	ErrBadSyncMode    = errors.New("source: invalid sync mode")
)

var cleanIDRe = regexp.MustCompile(`[^a-z0-9_-]+`)

// ValidSyncMode reports whether m is a supported SyncMode. Empty defaults to bidirectional.
func ValidSyncMode(m string) bool {
	switch m {
	case "", SyncBidirectional, SyncIngestOnly, SyncPublishOnly:
		return true
	default:
		return false
	}
}

// NormalizeSyncMode returns m or SyncBidirectional when empty.
func NormalizeSyncMode(m string) string {
	if m == "" {
		return SyncBidirectional
	}
	return m
}

// MakeSourceID builds a deterministic source_id for (provider, shop) when none is supplied.
func MakeSourceID(provider, shop string) string {
	base := strings.ToLower(strings.TrimSpace(shop))
	base = strings.TrimSuffix(base, ".myshopify.com")
	base = cleanIDRe.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		sum := sha256.Sum256([]byte(provider + ":" + shop))
		base = hex.EncodeToString(sum[:8])
	}
	if len(base) > 48 {
		base = base[:48]
	}
	return provider + "_" + base
}

// OrgDB returns the per-org namespaced datastore for orgId, failing closed on
// empty, credential-shaped, or reserved org names.
func OrgDB(ctx context.Context, orgId string) (*datastore.Datastore, error) {
	orgId = strings.TrimSpace(orgId)
	if orgId == "" || secret.Like(orgId) {
		return nil, ErrBadOrg
	}
	switch strings.ToLower(orgId) {
	case "system", "admin", "default":
		return nil, ErrBadOrg
	}
	if ctx == nil {
		ctx = context.Background()
	}
	nctx := nscontext.WithNamespace(ctx, orgId)
	db := datastore.NewNamespaced(nctx)
	if db.DB() == nil {
		return nil, fmt.Errorf("%w: %s", ErrBadOrg, orgId)
	}
	db.SetNamespace(orgId)
	return db, nil
}

// SystemDB returns the system-namespace datastore used for global shop->(org,source)
// lookup and webhook replay guards.
func SystemDB(ctx context.Context) *datastore.Datastore {
	if ctx == nil {
		ctx = context.Background()
	}
	db := datastore.New(ctx)
	db.SetNamespace("system")
	return db
}

// StoreSource represents one connected storefront in an org (1 Org : N Stores).
// Stored in the org's own datastore under deterministic key "src_<sourceId>".
type StoreSource struct {
	mixin.Model[StoreSource]

	SourceId string `json:"sourceId"`
	OrgId    string `json:"orgId"`
	Provider string `json:"provider"`
	Shop     string `json:"shop"`
	Domain   string `json:"domain,omitempty"`
	Name     string `json:"name,omitempty"`
	Brand    string `json:"brand,omitempty"`

	StockLocationId    string `json:"stockLocationId,omitempty"`
	ExternalLocationId string `json:"externalLocationId,omitempty"`
	PriceListId        string `json:"priceListId,omitempty"`
	Currency           string `json:"currency,omitempty"`
	Locale             string `json:"locale,omitempty"`
	Timezone           string `json:"timezone,omitempty"`

	SyncMode string `json:"syncMode"`
	Status   string `json:"status"`

	PixelKey        string `json:"pixelKey,omitempty"`
	PixelExternalId string `json:"pixelExternalId,omitempty"`
	PlanHandle      string `json:"planHandle,omitempty"`
	PlanActive      bool   `json:"planActive,omitempty"`
	EndpointURL     string `json:"endpointUrl,omitempty"`

	// SealedCredential holds AES-256-GCM encrypted per-store secret material
	// (Shopify offline token, Woo per-shop HMAC secret, optional REST credentials).
	// Never serialized to API JSON responses.
	SealedCredential string `json:"-" datastore:",noindex"`

	ProductsRead     int `json:"productsRead"`
	ProductsWritten  int `json:"productsWritten"`
	CustomersRead    int `json:"customersRead"`
	CustomersWritten int `json:"customersWritten"`
	OrdersRead       int `json:"ordersRead"`
	OrdersWritten    int `json:"ordersWritten"`
	CouponsWritten   int `json:"couponsWritten"`
	InventoryWritten int `json:"inventoryWritten"`

	LastBackfillStatus string    `json:"lastBackfillStatus,omitempty"`
	LastSyncedAt       time.Time `json:"lastSyncedAt,omitempty"`
	LastError          string    `json:"lastError,omitempty" datastore:",noindex"`
}

func (s *StoreSource) Load(ps []datastore.Property) error { return datastore.LoadStruct(s, ps) }
func (s *StoreSource) Save() ([]datastore.Property, error) { return datastore.SaveStruct(s) }

func NewStoreSource(db *datastore.Datastore) *StoreSource {
	s := new(StoreSource)
	s.Init(db)
	s.UseStringKey = true
	return s
}

func StoreSourceStorageID(sourceId string) string {
	return "src_" + strings.TrimSpace(sourceId)
}

// StoreSourceLookup maps (provider, shop) -> (orgId, sourceId) in the "system"
// namespace so sessionless webhooks/App Bridge JWTs resolve their owning org in
// O(1) while enforcing that one external shop cannot be bound to two orgs at once.
type StoreSourceLookup struct {
	mixin.Model[StoreSourceLookup]

	Provider string `json:"provider"`
	Shop     string `json:"shop"`
	OrgId    string `json:"orgId"`
	SourceId string `json:"sourceId"`
	Status   string `json:"status"`
}

func (l *StoreSourceLookup) Load(ps []datastore.Property) error { return datastore.LoadStruct(l, ps) }
func (l *StoreSourceLookup) Save() ([]datastore.Property, error) { return datastore.SaveStruct(l) }

func NewStoreSourceLookup(db *datastore.Datastore) *StoreSourceLookup {
	l := new(StoreSourceLookup)
	l.Init(db)
	l.UseStringKey = true
	return l
}

func LookupID(provider, shop string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(provider)) + "\x00" + strings.ToLower(strings.TrimSpace(shop))))
	return "slk_" + hex.EncodeToString(sum[:16])
}

// SyncLedger tracks per-(orgId, sourceId, entityKind, canonicalId) sync state
// and content hashes to guarantee zero webhook ping-pong loops.
type SyncLedger struct {
	mixin.Model[SyncLedger]

	OrgId               string `json:"orgId"`
	SourceId            string `json:"sourceId"`
	EntityKind          string `json:"entityKind"`
	CanonicalId         string `json:"canonicalId"`
	ExternalId          string `json:"externalId"`
	LastSyncedHash      string `json:"lastSyncedHash"`
	PendingOutboundHash string `json:"pendingOutboundHash,omitempty"`
}

func (l *SyncLedger) Load(ps []datastore.Property) error { return datastore.LoadStruct(l, ps) }
func (l *SyncLedger) Save() ([]datastore.Property, error) { return datastore.SaveStruct(l) }

func NewSyncLedger(db *datastore.Datastore) *SyncLedger {
	l := new(SyncLedger)
	l.Init(db)
	l.UseStringKey = true
	return l
}

// SyncExternalMap provides O(1) reverse lookup (sourceId, entityKind, externalId) -> canonicalId.
type SyncExternalMap struct {
	mixin.Model[SyncExternalMap]

	OrgId       string `json:"orgId"`
	SourceId    string `json:"sourceId"`
	EntityKind  string `json:"entityKind"`
	ExternalId  string `json:"externalId"`
	CanonicalId string `json:"canonicalId"`
}

func (m *SyncExternalMap) Load(ps []datastore.Property) error { return datastore.LoadStruct(m, ps) }
func (m *SyncExternalMap) Save() ([]datastore.Property, error) { return datastore.SaveStruct(m) }

func NewSyncExternalMap(db *datastore.Datastore) *SyncExternalMap {
	m := new(SyncExternalMap)
	m.Init(db)
	m.UseStringKey = true
	return m
}

// OutboundJob represents a queued or completed outbound write to a StoreSource.
type OutboundJob struct {
	mixin.Model[OutboundJob]

	JobId       string `json:"jobId"`
	OrgId       string `json:"orgId"`
	SourceId    string `json:"sourceId"`
	Provider    string `json:"provider"`
	Operation   string `json:"operation"`
	EntityKind  string `json:"entityKind"`
	CanonicalId string `json:"canonicalId"`
	ExternalId  string `json:"externalId,omitempty"`
	SKU         string `json:"sku,omitempty"`
	Payload     string `json:"payload" datastore:",noindex"`
	PayloadHash string `json:"payloadHash"`
	Status      string `json:"status"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"lastError,omitempty" datastore:",noindex"`
}

func (j *OutboundJob) Load(ps []datastore.Property) error { return datastore.LoadStruct(j, ps) }
func (j *OutboundJob) Save() ([]datastore.Property, error) { return datastore.SaveStruct(j) }

func NewOutboundJob(db *datastore.Datastore) *OutboundJob {
	j := new(OutboundJob)
	j.Init(db)
	j.UseStringKey = true
	return j
}
