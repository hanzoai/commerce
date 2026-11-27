// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Package source mounts Hanzo Commerce's multi-store bidirectional storefront
// connectors under /v1/source/*:
//   - /v1/source/sources       (1 Org : N StoreSource CRUD, health & reconcile)
//   - /v1/source/publish/:kind (canonical catalog/coupon/inventory/fulfillment/refund/consent fan-out)
//   - /v1/source/shopify/*     (App Home, session JWT, IAM link/callback, pixel, webhooks, Flow, GraphQL 2026-10)
//   - /v1/source/woocommerce/* (connect, signed ingest/sync/webhook, GDPR export/erase, outbound push/pull)
package source

import (
	"context"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/shopify"
	"github.com/hanzoai/commerce/source/woocommerce"
)

// Bundle holds a wired multi-store Engine plus its Shopify and WooCommerce services.
type Bundle struct {
	Engine      *hub.Engine
	Shopify     *shopify.Service
	WooCommerce *woocommerce.Service
}

var defaultBundle = NewBundle(shopify.Config{}, hub.Default(), nil)

// Default returns the process-wide multi-store Bundle.
func Default() *Bundle { return defaultBundle }

// NewBundle constructs an isolated Bundle (used both in production and in multi-store tests).
func NewBundle(shCfg shopify.Config, engine *hub.Engine, httpClient *http.Client) *Bundle {
	if engine == nil {
		engine = hub.NewEngine()
	}
	return &Bundle{
		Engine:      engine,
		Shopify:     shopify.NewService(shCfg, engine, httpClient),
		WooCommerce: woocommerce.NewService(engine, httpClient),
	}
}

// Mount registers /source/* onto a /v1 group.
func (b *Bundle) Mount(v1 *zip.Group) {
	grp := v1.Group("/source")
	grp.Use(middleware.RequestContext())
	grp.Use(middleware.AccessControl("*"))

	hub.MountRoutes(grp, b.Engine)
	b.Shopify.MountRoutes(grp)
	b.WooCommerce.MountRoutes(grp)
}

// Route mounts the default multi-store Bundle onto the /v1 router group.
func Route(v1 *zip.Group) {
	defaultBundle.Mount(v1)
}

// SyncCanonicalProductByID loads a persisted canonical Product in orgID and fans it
// out to all publish-enabled StoreSources in that org.
func SyncCanonicalProductByID(ctx context.Context, engine *hub.Engine, orgID, productID string) {
	if engine == nil {
		engine = hub.Default()
	}
	orgID = strings.TrimSpace(orgID)
	productID = strings.TrimSpace(productID)
	if orgID == "" || productID == "" {
		return
	}
	orgDB, err := hub.OrgDB(ctx, orgID)
	if err != nil {
		return
	}
	p := product.New(orgDB)
	if err := p.GetById(productID); err != nil {
		return
	}
	cp := hub.CanonicalFromProduct(p, true, nil)
	_ = engine.FanOutProduct(ctx, orgID, "", cp)
}

// SyncCanonicalCouponByID loads a persisted canonical Coupon in orgID and fans it
// out to all publish-enabled StoreSources in that org.
func SyncCanonicalCouponByID(ctx context.Context, engine *hub.Engine, orgID, couponID string) {
	if engine == nil {
		engine = hub.Default()
	}
	orgID = strings.TrimSpace(orgID)
	couponID = strings.TrimSpace(couponID)
	if orgID == "" || couponID == "" {
		return
	}
	orgDB, err := hub.OrgDB(ctx, orgID)
	if err != nil {
		return
	}
	c := coupon.New(orgDB)
	if err := c.GetById(couponID); err != nil {
		return
	}
	_ = engine.FanOutCoupon(ctx, orgID, "", hub.CanonicalCoupon{
		CanonicalId: c.Id(),
		Code:        c.Code_,
		Name:        c.Name,
		Type:        string(c.Type),
		Amount:      c.Amount,
		Limit:       c.Limit,
		Once:        c.Once,
		Enabled:     c.Enabled,
	})
}

// ObserveCouponREST is route middleware on /v1/coupon that fans out newly created or
// updated canonical coupons to all connected bidirectional/publish_only stores in the org.
func ObserveCouponREST(c *zip.Ctx) error {
	method := c.Method()
	if err := c.Next(); err != nil {
		return err
	}
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
		return nil
	}
	if st := c.Fiber().Response().StatusCode(); st < 200 || st >= 300 {
		return nil
	}
	p := c.Path()
	if strings.HasSuffix(p, "/validate") || strings.HasSuffix(p, "/redeem") || strings.Contains(p, "/code") {
		return nil
	}
	orgID := hub.ResolveRequestOrg(c)
	if orgID == "" {
		return nil
	}
	couponID := strings.TrimSpace(c.Param("couponid"))
	if couponID == "" {
		loc := c.Fiber().GetRespHeader("Location")
		if i := strings.LastIndexByte(loc, '/'); i >= 0 {
			couponID = strings.TrimSpace(loc[i+1:])
		}
	}
	if couponID != "" {
		SyncCanonicalCouponByID(c.Context(), defaultBundle.Engine, orgID, couponID)
	}
	return nil
}
