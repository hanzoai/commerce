// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/stocklocation"
)

// RegisterInput describes the parameters to create or update a connected StoreSource.
type RegisterInput struct {
	OrgId              string
	SourceId           string
	Provider           string
	Shop               string
	Domain             string
	Name               string
	Brand              string
	StockLocationId    string
	ExternalLocationId string
	PriceListId        string
	Currency           string
	Locale             string
	Timezone           string
	SyncMode           string
	Status             string
	PixelKey           string
	EndpointURL        string
	Credentials        *SourceCredentials
}

// RegisterSource creates or updates a StoreSource in (OrgId, SourceId), enforcing
// that an external (Provider, Shop) cannot be bound to two different orgs while
// allowing one org to connect arbitrarily many Shopify, WooCommerce, and Hanzo stores.
func RegisterSource(ctx context.Context, in RegisterInput) (*StoreSource, error) {
	in.OrgId = strings.TrimSpace(in.OrgId)
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Shop = strings.TrimSpace(in.Shop)
	if in.Provider == ProviderShopify {
		in.Shop = strings.ToLower(in.Shop)
	}
	if in.OrgId == "" {
		return nil, ErrBadOrg
	}
	if in.Provider == "" || in.Shop == "" {
		return nil, errors.New("source: provider and shop are required")
	}
	if !ValidSyncMode(in.SyncMode) {
		return nil, ErrBadSyncMode
	}
	if in.SourceId == "" {
		in.SourceId = MakeSourceID(in.Provider, in.Shop)
	}

	orgDB, err := OrgDB(ctx, in.OrgId)
	if err != nil {
		return nil, err
	}
	sysDB := SystemDB(ctx)

	// Enforce cross-org exclusivity on (provider, shop) in the system index.
	lkID := LookupID(in.Provider, in.Shop)
	lk := NewStoreSourceLookup(sysDB)
	lkErr := lk.Get(sysDB.NewKey(lk.Kind(), lkID, 0, nil))
	if lkErr == nil && lk.OrgId != "" && lk.OrgId != in.OrgId && lk.Status == StatusActive {
		return nil, ErrSourceConflict
	}

	srcID := StoreSourceStorageID(in.SourceId)
	src := NewStoreSource(orgDB)
	getErr := src.Get(orgDB.NewKey(src.Kind(), srcID, 0, nil))
	if getErr != nil && !errors.Is(getErr, datastore.ErrNoSuchEntity) {
		return nil, getErr
	}

	src.SetId(srcID)
	src.SourceId = in.SourceId
	src.OrgId = in.OrgId
	src.Provider = in.Provider
	src.Shop = in.Shop
	if in.Domain != "" {
		src.Domain = in.Domain
	} else if src.Domain == "" {
		src.Domain = in.Shop
	}
	if in.Name != "" {
		src.Name = in.Name
	} else if src.Name == "" {
		src.Name = in.Shop
	}
	if in.Brand != "" {
		src.Brand = in.Brand
	}
	if in.ExternalLocationId != "" {
		src.ExternalLocationId = in.ExternalLocationId
	}
	if in.PriceListId != "" {
		src.PriceListId = in.PriceListId
	}
	if in.Currency != "" {
		src.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	} else if src.Currency == "" {
		src.Currency = "USD"
	}
	if in.Locale != "" {
		src.Locale = in.Locale
	}
	if in.Timezone != "" {
		src.Timezone = in.Timezone
	}
	if in.SyncMode != "" || src.SyncMode == "" {
		src.SyncMode = NormalizeSyncMode(in.SyncMode)
	}
	if in.Status != "" {
		src.Status = in.Status
	} else if src.Status == "" || src.Status == StatusDisconnected || src.Status == StatusUninstalled {
		src.Status = StatusActive
	}
	if in.PixelKey != "" {
		src.PixelKey = in.PixelKey
	}
	if in.EndpointURL != "" {
		src.EndpointURL = in.EndpointURL
	}

	// Ensure every connected store is bound to a canonical StockLocation in the org.
	if in.StockLocationId != "" {
		src.StockLocationId = in.StockLocationId
	} else if src.StockLocationId == "" {
		loc := stocklocation.New(orgDB)
		loc.Name = src.Name
		if err := loc.Create(); err == nil {
			src.StockLocationId = loc.Id()
		}
	}

	if in.Credentials != nil {
		existing, _ := OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
		if in.Credentials.AccessToken != "" {
			existing.AccessToken = in.Credentials.AccessToken
		}
		if in.Credentials.HMACSecret != "" {
			existing.HMACSecret = in.Credentials.HMACSecret
		}
		if in.Credentials.ConsumerKey != "" {
			existing.ConsumerKey = in.Credentials.ConsumerKey
		}
		if in.Credentials.ConsumerSecret != "" {
			existing.ConsumerSecret = in.Credentials.ConsumerSecret
		}
		if in.Credentials.WebhookSecret != "" {
			existing.WebhookSecret = in.Credentials.WebhookSecret
		}
		sealed, err := SealCredentials(ctx, src.OrgId, src.SourceId, existing)
		if err != nil {
			return nil, err
		}
		src.SealedCredential = sealed
	}

	if err := src.Put(); err != nil {
		return nil, err
	}

	lk.SetId(lkID)
	lk.Provider = in.Provider
	lk.Shop = in.Shop
	lk.OrgId = in.OrgId
	lk.SourceId = in.SourceId
	lk.Status = src.Status
	if err := lk.Put(); err != nil {
		return nil, err
	}

	return src, nil
}

// GetSource loads a StoreSource by (orgId, sourceId).
func GetSource(ctx context.Context, orgId, sourceId string) (*StoreSource, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, err
	}
	src := NewStoreSource(orgDB)
	id := StoreSourceStorageID(sourceId)
	if err := src.Get(orgDB.NewKey(src.Kind(), id, 0, nil)); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, ErrSourceNotFound
		}
		return nil, err
	}
	src.Init(orgDB)
	src.UseStringKey = true
	src.SetId(id)
	return src, nil
}

// LookupByShop resolves (provider, shop) via the system lookup index and loads the org's StoreSource.
func LookupByShop(ctx context.Context, provider, shop string) (*StoreSource, error) {
	sysDB := SystemDB(ctx)
	lk := NewStoreSourceLookup(sysDB)
	id := LookupID(provider, shop)
	if err := lk.Get(sysDB.NewKey(lk.Kind(), id, 0, nil)); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, ErrSourceNotFound
		}
		return nil, err
	}
	if lk.OrgId == "" || lk.SourceId == "" {
		return nil, ErrSourceNotFound
	}
	return GetSource(ctx, lk.OrgId, lk.SourceId)
}

// ListSources returns all StoreSources in orgId sorted by SourceId.
func ListSources(ctx context.Context, orgId string) ([]*StoreSource, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, err
	}
	rows := make([]*StoreSource, 0)
	if _, err := orgDB.Query("store-source").GetAll(&rows); err != nil {
		return nil, err
	}
	out := make([]*StoreSource, 0, len(rows))
	for _, r := range rows {
		if r == nil || r.OrgId != orgId {
			continue
		}
		r.Init(orgDB)
		r.UseStringKey = true
		r.SetId(StoreSourceStorageID(r.SourceId))
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceId < out[j].SourceId })
	return out, nil
}

// DisconnectSource marks a StoreSource disconnected/uninstalled, drops its sealed credentials,
// and releases its global shop lookup so the shop can reconnect later.
func DisconnectSource(ctx context.Context, orgId, sourceId, status string) error {
	src, err := GetSource(ctx, orgId, sourceId)
	if err != nil {
		return err
	}
	if status == "" {
		status = StatusDisconnected
	}
	src.Status = status
	src.SealedCredential = ""
	src.LastSyncedAt = time.Now().UTC()
	if err := src.Put(); err != nil {
		return err
	}
	sysDB := SystemDB(ctx)
	lk := NewStoreSourceLookup(sysDB)
	id := LookupID(src.Provider, src.Shop)
	if err := lk.Get(sysDB.NewKey(lk.Kind(), id, 0, nil)); err == nil && lk.OrgId == orgId {
		lk.SetId(id)
		lk.Status = status
		_ = lk.Put()
	}
	return nil
}

// NewJobID generates a random outbound job identifier.
func NewJobID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "job_" + hex.EncodeToString(b[:])
}

// ListPendingJobs returns pending OutboundJobs for (orgId, sourceId) in creation order.
func ListPendingJobs(ctx context.Context, orgId, sourceId string, limit int) ([]*OutboundJob, error) {
	orgDB, err := OrgDB(ctx, orgId)
	if err != nil {
		return nil, err
	}
	all := make([]*OutboundJob, 0)
	q := orgDB.Query("source-outbound-job").Filter("SourceId=", sourceId).Filter("Status=", JobPending)
	if _, err := q.GetAll(&all); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	for _, j := range all {
		j.Init(orgDB)
		j.UseStringKey = true
		j.SetId(j.JobId)
	}
	return all, nil
}
