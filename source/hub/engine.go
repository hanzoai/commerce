// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/models/collection"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/types"
)

// OutboundResult reports the outcome of an OutboundDriver push.
type OutboundResult struct {
	// Queued is true when the driver leaves the OutboundJob pending for plugin pull
	// (e.g. a WooCommerce store without direct REST credentials).
	Queued     bool
	ExternalId string
}

// OutboundDriver executes outbound sync jobs against an external storefront provider.
type OutboundDriver interface {
	Provider() string
	Deliver(ctx context.Context, src *StoreSource, creds SourceCredentials, job *OutboundJob) (OutboundResult, error)
}

// Engine coordinates inbound ingestion, loop-free hash checking, master SKU
// cross-store inventory unification, and multi-store outbound fan-out.
type Engine struct {
	mu      sync.RWMutex
	drivers map[string]OutboundDriver
	skuMu   sync.Mutex
}

// NewEngine constructs a new bidirectional multi-store Engine.
func NewEngine() *Engine {
	return &Engine{
		drivers: make(map[string]OutboundDriver),
	}
}

var defaultEngine = NewEngine()

// Default returns the process-wide multi-store sync Engine.
func Default() *Engine { return defaultEngine }

// RegisterDriver registers an OutboundDriver for its Provider().
func (e *Engine) RegisterDriver(d OutboundDriver) {
	if d == nil {
		return
	}
	e.mu.Lock()
	e.drivers[d.Provider()] = d
	e.mu.Unlock()
}

func (e *Engine) driverFor(provider string) OutboundDriver {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.drivers[provider]
}

var slugCleanRe = regexp.MustCompile(`[^a-z0-9-]+`)

func makeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugCleanRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "item"
	}
	return s
}

func canIngest(s *StoreSource) bool {
	return s != nil && s.Status == StatusActive && (s.SyncMode == SyncBidirectional || s.SyncMode == SyncIngestOnly)
}

func canPublish(s *StoreSource) bool {
	return s != nil && s.Status == StatusActive && (s.SyncMode == SyncBidirectional || s.SyncMode == SyncPublishOnly)
}

// IngestProduct normalizes an inbound product into Hanzo Commerce with echo suppression,
// syncs its variants & master SKU inventory, and fans out to sibling stores in the org.
func (e *Engine) IngestProduct(ctx context.Context, orgId, originSourceId string, in CanonicalProduct) (*product.Product, bool, error) {
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

	if in.SKU == "" && len(in.Variants) > 0 && in.Variants[0].SKU != "" {
		in.SKU = in.Variants[0].SKU
	}
	if in.Slug == "" {
		in.Slug = makeSlug(in.Name)
		if in.Slug == "item" && in.SKU != "" {
			in.Slug = makeSlug(in.SKU)
		}
	}
	if in.SKU == "" {
		in.SKU = strings.ToUpper(in.Slug)
	}
	if in.Currency == "" && origin != nil && origin.Currency != "" {
		in.Currency = origin.Currency
	}
	if in.Currency == "" {
		in.Currency = "USD"
	}
	if in.PriceCents == 0 && len(in.Variants) > 0 {
		in.PriceCents = in.Variants[0].PriceCents
	}

	hash := HashProduct(in)

	// Resolve canonical product ID by existing external mapping, or master SKU, or Slug.
	canonicalID := in.CanonicalId
	if canonicalID == "" && originSourceId != "" && in.ExternalId != "" {
		if cid, ok := ResolveCanonicalID(orgDB, originSourceId, EntityProduct, in.ExternalId); ok {
			canonicalID = cid
		}
	}
	existingProd := findProduct(orgDB, canonicalID, in.SKU, in.Slug)
	if existingProd != nil {
		canonicalID = existingProd.Id()
	}

	// Loop-free echo check: drop immediately if hash matches LastSyncedHash or PendingOutboundHash!
	if originSourceId != "" && canonicalID != "" {
		echo, err := CheckEcho(orgDB, orgId, originSourceId, EntityProduct, canonicalID, in.ExternalId, hash)
		if err != nil {
			return nil, false, err
		}
		if echo {
			return existingProd, true, nil
		}
	}

	prod := existingProd
	isNew := false
	if prod == nil {
		prod = product.New(orgDB)
		isNew = true
	} else {
		prod.Init(orgDB)
	}

	prod.Name = in.Name
	prod.Slug = in.Slug
	prod.SKU = in.SKU
	prod.UPC = in.UPC
	prod.Description = in.Description
	prod.Currency = currency.Type(strings.ToLower(in.Currency))
	prod.Price = in.PriceCents
	prod.Available = in.Available
	prod.Hidden = in.Hidden
	if in.ManageStock {
		prod.Inventory = in.Inventory
	}
	media := make([]types.Media, 0, len(in.Images))
	for _, img := range in.Images {
		if strings.TrimSpace(img) != "" {
			media = append(media, types.Media{Type: types.MediaImage, Url: img})
		}
	}
	if len(media) > 0 {
		prod.Media = media
		prod.Image = media[0]
	}

	if isNew {
		if err := prod.Create(); err != nil {
			return nil, false, err
		}
	}

	// Upsert child variants and unify master SKU inventory.
	locID := ""
	if origin != nil {
		locID = origin.StockLocationId
	}
	vars := make([]*variant.Variant, 0, len(in.Variants))
	for i, cv := range in.Variants {
		vSKU := strings.TrimSpace(cv.SKU)
		if vSKU == "" {
			vSKU = fmt.Sprintf("%s-%d", prod.SKU, i+1)
		}
		vName := strings.TrimSpace(cv.Name)
		if vName == "" {
			vName = prod.Name
		}
		v := findOrNewVariant(orgDB, prod.Id(), vSKU)
		v.ProductId = prod.Id()
		v.SKU = vSKU
		v.UPC = cv.Barcode
		v.Name = vName
		v.Currency = prod.Currency
		v.Price = cv.PriceCents
		v.MSRP = cv.MSRPCents
		v.Available = cv.Available
		if cv.ManageStock {
			v.Inventory = cv.Inventory
		}
		if cv.ImageURL != "" {
			v.Image = types.Media{Type: types.MediaImage, Url: cv.ImageURL}
		}
		if v.Key() == nil || v.Id_ == "" {
			_ = v.Create()
		} else {
			_ = v.Put()
		}
		vars = append(vars, v)
		if cv.ManageStock {
			e.upsertCanonicalInventory(orgDB, vSKU, vName, locID, cv.Inventory)
			if originSourceId != "" {
				_ = RecordInbound(orgDB, orgId, originSourceId, EntityInventory, vSKU, cv.ExternalId, HashInventory(vSKU, cv.Inventory))
			}
		}
	}
	if len(vars) == 0 && in.ManageStock && in.SKU != "" {
		e.upsertCanonicalInventory(orgDB, in.SKU, in.Name, locID, in.Inventory)
		if originSourceId != "" {
			_ = RecordInbound(orgDB, orgId, originSourceId, EntityInventory, in.SKU, in.ExternalId, HashInventory(in.SKU, in.Inventory))
		}
	}
	prod.Variants = vars
	if err := prod.Put(); err != nil {
		return nil, false, err
	}

	canonicalID = prod.Id()
	in.CanonicalId = canonicalID

	if originSourceId != "" {
		if err := RecordInbound(orgDB, orgId, originSourceId, EntityProduct, canonicalID, in.ExternalId, hash); err != nil {
			return nil, false, err
		}
		origin.ProductsRead++
		origin.LastSyncedAt = time.Now().UTC()
		_ = origin.Put()
	}

	if err := e.FanOutProduct(ctx, orgId, originSourceId, CanonicalFromProduct(prod, in.ManageStock, in.Variants)); err != nil {
		return prod, false, err
	}
	return prod, false, nil
}

// CanonicalFromProduct builds a CanonicalProduct from a persisted product.Product.
func CanonicalFromProduct(p *product.Product, parentManaged bool, hintVars []CanonicalVariant) CanonicalProduct {
	managedBySKU := map[string]bool{}
	for _, hv := range hintVars {
		if hv.SKU != "" {
			managedBySKU[hv.SKU] = hv.ManageStock
		}
	}
	vars := make([]CanonicalVariant, 0, len(p.Variants))
	for _, v := range p.Variants {
		if v == nil {
			continue
		}
		mg := true
		if m, ok := managedBySKU[v.SKU]; ok {
			mg = m
		}
		vars = append(vars, CanonicalVariant{
			SKU:         v.SKU,
			Name:        v.Name,
			PriceCents:  v.Price,
			MSRPCents:   v.MSRP,
			Available:   v.Available,
			ManageStock: mg,
			Inventory:   v.Inventory,
			ImageURL:    v.Image.Url,
			Barcode:     v.UPC,
		})
	}
	imgs := make([]string, 0, len(p.Media))
	for _, m := range p.Media {
		if m.Url != "" {
			imgs = append(imgs, m.Url)
		}
	}
	mg := parentManaged
	if !mg && len(vars) == 0 && p.Inventory > 0 {
		mg = true
	}
	return CanonicalProduct{
		CanonicalId: p.Id(),
		Slug:        p.Slug,
		SKU:         p.SKU,
		UPC:         p.UPC,
		Name:        p.Name,
		Description: p.Description,
		Currency:    strings.ToUpper(string(p.Currency)),
		PriceCents:  p.Price,
		Available:   p.Available,
		Hidden:      p.Hidden,
		ManageStock: mg,
		Inventory:   p.Inventory,
		Images:      imgs,
		Variants:    vars,
	}
}

// FanOutProduct dispatches product.upsert to every publish-enabled StoreSource in orgId
// except originSourceId, stamping PendingOutboundHash first to suppress echoes.
func (e *Engine) FanOutProduct(ctx context.Context, orgId, originSourceId string, cp CanonicalProduct) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashProduct(cp)
	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		ledger, _ := GetLedger(orgDB, dst.SourceId, EntityProduct, cp.CanonicalId)
		extID := ""
		if ledger != nil {
			if ledger.LastSyncedHash == hash {
				continue
			}
			extID = ledger.ExternalId
		}
		outCopy := cp
		outCopy.ExternalId = extID
		_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityProduct, cp.CanonicalId, extID, hash)
		for _, v := range cp.Variants {
			if v.ManageStock && v.SKU != "" {
				_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityInventory, v.SKU, "", HashInventory(v.SKU, v.Inventory))
			}
		}
		if err := e.enqueueAndDeliver(ctx, orgDB, dst, OpProductUpsert, EntityProduct, cp.CanonicalId, extID, cp.SKU, outCopy, hash); err != nil {
			dst.LastError = err.Error()
			_ = dst.Put()
		}
	}
	return nil
}

// IngestCollection upserts a CanonicalCollection and fans out to sibling stores.
func (e *Engine) IngestCollection(ctx context.Context, orgId, originSourceId string, in CanonicalCollection) (*collection.Collection, bool, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, false, err
	}
	if in.Slug == "" {
		in.Slug = makeSlug(in.Name)
	}
	hash := HashValue(struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
		Pub  bool   `json:"pub"`
	}{in.Slug, in.Name, in.Published})

	cid := in.CanonicalId
	if cid == "" && originSourceId != "" && in.ExternalId != "" {
		cid, _ = ResolveCanonicalID(orgDB, originSourceId, EntityCollection, in.ExternalId)
	}
	col := findCollection(orgDB, cid, in.Slug)
	if col != nil {
		cid = col.Id()
	}
	if originSourceId != "" && cid != "" {
		if echo, _ := CheckEcho(orgDB, orgId, originSourceId, EntityCollection, cid, in.ExternalId, hash); echo {
			return col, true, nil
		}
	}
	if col == nil {
		col = collection.New(orgDB)
		col.Slug = in.Slug
		col.Name = in.Name
		col.Description = in.Description
		col.Published = in.Published
		col.Available = in.Published
		if err := col.Create(); err != nil {
			return nil, false, err
		}
	} else {
		col.Init(orgDB)
		col.Name = in.Name
		col.Description = in.Description
		col.Published = in.Published
		col.Available = in.Published
		if err := col.Put(); err != nil {
			return nil, false, err
		}
	}
	if originSourceId != "" {
		_ = RecordInbound(orgDB, orgId, originSourceId, EntityCollection, col.Id(), in.ExternalId, hash)
	}
	return col, false, nil
}

// IngestCustomer upserts a customer + marketing consent into the org's user store
// and fans out consent changes when bidirectional.
func (e *Engine) IngestCustomer(ctx context.Context, orgId, originSourceId string, in CanonicalCustomer) (*user.User, bool, error) {
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
	email := strings.ToLower(strings.TrimSpace(in.Email))
	hash := HashValue(struct {
		Email   string `json:"email"`
		Consent string `json:"consent"`
	}{email, in.MarketingConsent})

	cid := in.CanonicalId
	if cid == "" && originSourceId != "" && in.ExternalId != "" {
		cid, _ = ResolveCanonicalID(orgDB, originSourceId, EntityCustomer, in.ExternalId)
	}
	u := findCustomer(orgDB, cid, email)
	if u != nil {
		cid = u.Id()
	}
	if originSourceId != "" && cid != "" {
		if echo, _ := CheckEcho(orgDB, orgId, originSourceId, EntityCustomer, cid, in.ExternalId, hash); echo {
			return u, true, nil
		}
	}
	isNew := false
	if u == nil {
		u = user.New(orgDB)
		isNew = true
	} else {
		u.Init(orgDB)
	}
	if email != "" {
		u.Email = email
	}
	if in.FirstName != "" {
		u.FirstName = in.FirstName
	} else if u.FirstName == "" {
		u.FirstName = "Customer"
	}
	if in.LastName != "" {
		u.LastName = in.LastName
	}
	if u.Metadata == nil {
		u.Metadata = make(types.Map)
	}
	if in.MarketingConsent != "" {
		u.Metadata["marketing_consent"] = in.MarketingConsent
	}
	if in.ConsentUpdatedAt != "" {
		u.Metadata["consent_updated_at"] = in.ConsentUpdatedAt
	}
	if originSourceId != "" && in.ExternalId != "" {
		u.Metadata["source_"+originSourceId+"_id"] = in.ExternalId
	}
	if isNew {
		if err := u.Create(); err != nil {
			return nil, false, err
		}
	} else {
		if err := u.Put(); err != nil {
			return nil, false, err
		}
	}
	if originSourceId != "" {
		_ = RecordInbound(orgDB, orgId, originSourceId, EntityCustomer, u.Id(), in.ExternalId, hash)
		origin.CustomersRead++
		origin.LastSyncedAt = time.Now().UTC()
		_ = origin.Put()
	}
	if in.MarketingConsent == "subscribed" || in.MarketingConsent == "unsubscribed" {
		in.CanonicalId = u.Id()
		_ = e.FanOutCustomerConsent(ctx, orgId, originSourceId, in)
	}
	return u, false, nil
}

// FanOutCustomerConsent pushes marketing consent updates to sibling stores.
func (e *Engine) FanOutCustomerConsent(ctx context.Context, orgId, originSourceId string, cc CanonicalCustomer) error {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return err
	}
	sources, err := ListSources(ctx, orgId)
	if err != nil {
		return err
	}
	hash := HashValue(struct {
		Email   string `json:"email"`
		Consent string `json:"consent"`
	}{strings.ToLower(strings.TrimSpace(cc.Email)), cc.MarketingConsent})

	for _, dst := range sources {
		if dst.SourceId == originSourceId || !canPublish(dst) {
			continue
		}
		extID := ""
		if cc.CanonicalId != "" {
			if l, _ := GetLedger(orgDB, dst.SourceId, EntityCustomer, cc.CanonicalId); l != nil {
				if l.LastSyncedHash == hash {
					continue
				}
				extID = l.ExternalId
			}
			_, _ = RecordOutboundPending(orgDB, orgId, dst.SourceId, EntityCustomer, cc.CanonicalId, extID, hash)
		}
		out := cc
		out.ExternalId = extID
		_ = e.enqueueAndDeliver(ctx, orgDB, dst, OpCustomerConsent, EntityConsent, cc.CanonicalId, extID, "", out, hash)
	}
	return nil
}
