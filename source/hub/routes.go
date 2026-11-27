// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package hub

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	pkgAuth "github.com/hanzoai/commerce/pkg/auth"
	"github.com/hanzoai/commerce/secret"
)

// ResolveRequestOrg extracts the authenticated organization slug from a request
// verified by TokenRequired / IAMTokenRequired. Never trusts an unauthenticated header.
func ResolveRequestOrg(c *zip.Ctx) string {
	if org, ok := middleware.GetOrganizationOK(c); ok && org != nil && org.Name != "" {
		return org.Name
	}
	if iammiddleware.IsIAMAuthenticated(c) {
		if o := strings.TrimSpace(pkgAuth.OrgID(c.Context())); o != "" && !secret.Like(o) {
			return o
		}
		if o := strings.TrimSpace(c.Header("X-Org-Id")); o != "" && !secret.Like(o) {
			return o
		}
	}
	return ""
}

// MountRoutes registers /v1/source/sources and /v1/source/publish/* on group.
func MountRoutes(group *zip.Group, engine *Engine) {
	if engine == nil {
		engine = Default()
	}
	authed := group.Group("")
	authed.Use(iammiddleware.IAMTokenRequired())
	authed.Use(middleware.TokenRequired())

	authed.Raw(http.MethodGet, "/sources", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		sources, err := ListSources(c.Context(), orgID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"org":     orgID,
			"count":   len(sources),
			"sources": sources,
		})
	})

	authed.Raw(http.MethodPost, "/sources", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		var req struct {
			SourceId           string `json:"sourceId"`
			Provider           string `json:"provider"`
			Shop               string `json:"shop"`
			Domain             string `json:"domain"`
			Name               string `json:"name"`
			Brand              string `json:"brand"`
			StockLocationId    string `json:"stockLocationId"`
			ExternalLocationId string `json:"externalLocationId"`
			PriceListId        string `json:"priceListId"`
			Currency           string `json:"currency"`
			Locale             string `json:"locale"`
			Timezone           string `json:"timezone"`
			SyncMode           string `json:"syncMode"`
			PixelKey           string `json:"pixelKey"`
			EndpointURL        string `json:"endpointUrl"`
			AccessToken        string `json:"accessToken"`
			HMACSecret         string `json:"hmacSecret"`
			ConsumerKey        string `json:"consumerKey"`
			ConsumerSecret     string `json:"consumerSecret"`
			WebhookSecret      string `json:"webhookSecret"`
		}
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
		}
		var creds *SourceCredentials
		if req.AccessToken != "" || req.HMACSecret != "" || req.ConsumerKey != "" || req.ConsumerSecret != "" || req.WebhookSecret != "" {
			creds = &SourceCredentials{
				AccessToken:    req.AccessToken,
				HMACSecret:     req.HMACSecret,
				ConsumerKey:    req.ConsumerKey,
				ConsumerSecret: req.ConsumerSecret,
				WebhookSecret:  req.WebhookSecret,
			}
		}
		src, err := RegisterSource(c.Context(), RegisterInput{
			OrgId:              orgID,
			SourceId:           req.SourceId,
			Provider:           req.Provider,
			Shop:               req.Shop,
			Domain:             req.Domain,
			Name:               req.Name,
			Brand:              req.Brand,
			StockLocationId:    req.StockLocationId,
			ExternalLocationId: req.ExternalLocationId,
			PriceListId:        req.PriceListId,
			Currency:           req.Currency,
			Locale:             req.Locale,
			Timezone:           req.Timezone,
			SyncMode:           req.SyncMode,
			PixelKey:           req.PixelKey,
			EndpointURL:        req.EndpointURL,
			Credentials:        creds,
		})
		if err != nil {
			if errors.Is(err, ErrSourceConflict) {
				return c.JSON(http.StatusConflict, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusCreated, src)
	})

	authed.Raw(http.MethodGet, "/sources/:sourceid", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		src, err := GetSource(c.Context(), orgID, c.Param("sourceid"))
		if err != nil {
			if errors.Is(err, ErrSourceNotFound) {
				return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		pending, _ := ListPendingJobs(c.Context(), orgID, src.SourceId, 20)
		return c.JSON(http.StatusOK, map[string]any{
			"source":      src,
			"pendingJobs": len(pending),
		})
	})

	authed.Raw(http.MethodPatch, "/sources/:sourceid", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		existing, err := GetSource(c.Context(), orgID, c.Param("sourceid"))
		if err != nil {
			if errors.Is(err, ErrSourceNotFound) {
				return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		var req struct {
			Domain             string `json:"domain"`
			Name               string `json:"name"`
			Brand              string `json:"brand"`
			StockLocationId    string `json:"stockLocationId"`
			ExternalLocationId string `json:"externalLocationId"`
			PriceListId        string `json:"priceListId"`
			Currency           string `json:"currency"`
			SyncMode           string `json:"syncMode"`
			PixelKey           string `json:"pixelKey"`
			EndpointURL        string `json:"endpointUrl"`
			AccessToken        string `json:"accessToken"`
			ConsumerKey        string `json:"consumerKey"`
			ConsumerSecret     string `json:"consumerSecret"`
		}
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
		}
		var creds *SourceCredentials
		if req.AccessToken != "" || req.ConsumerKey != "" || req.ConsumerSecret != "" {
			creds = &SourceCredentials{
				AccessToken:    req.AccessToken,
				ConsumerKey:    req.ConsumerKey,
				ConsumerSecret: req.ConsumerSecret,
			}
		}
		updated, err := RegisterSource(c.Context(), RegisterInput{
			OrgId:              orgID,
			SourceId:           existing.SourceId,
			Provider:           existing.Provider,
			Shop:               existing.Shop,
			Domain:             req.Domain,
			Name:               req.Name,
			Brand:              req.Brand,
			StockLocationId:    req.StockLocationId,
			ExternalLocationId: req.ExternalLocationId,
			PriceListId:        req.PriceListId,
			Currency:           req.Currency,
			SyncMode:           req.SyncMode,
			PixelKey:           req.PixelKey,
			EndpointURL:        req.EndpointURL,
			Credentials:        creds,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, updated)
	})

	authed.Raw(http.MethodDelete, "/sources/:sourceid", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		if err := DisconnectSource(c.Context(), orgID, c.Param("sourceid"), StatusDisconnected); err != nil {
			if errors.Is(err, ErrSourceNotFound) {
				return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"disconnected": true})
	})

	authed.Raw(http.MethodPost, "/sources/:sourceid/reconcile", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		counts, err := engine.ReconcileSource(c.Context(), orgID, c.Param("sourceid"))
		if err != nil {
			if errors.Is(err, ErrSourceNotFound) {
				return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"status": "reconciled", "pushed": counts})
	})

	authed.Raw(http.MethodPost, "/publish/:kind", func(c *zip.Ctx) error {
		orgID := ResolveRequestOrg(c)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
		}
		switch c.Param("kind") {
		case "product":
			var in CanonicalProduct
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product json"})
			}
			p, _, err := engine.IngestProduct(c.Context(), orgID, "", in)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, p)
		case "coupon":
			var in CanonicalCoupon
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid coupon json"})
			}
			cpn, _, err := engine.IngestCoupon(c.Context(), orgID, "", in)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, cpn)
		case "inventory":
			var in CanonicalInventoryUpdate
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid inventory json"})
			}
			qty, err := engine.SetSKUInventory(c.Context(), orgID, "", in.SKU, in.Quantity)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"sku": in.SKU, "quantity": qty})
		case "fulfillment":
			var in CanonicalFulfillment
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid fulfillment json"})
			}
			if err := engine.PublishFulfillment(c.Context(), orgID, "", in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"ok": true})
		case "refund":
			var in CanonicalRefund
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid refund json"})
			}
			if err := engine.PublishRefund(c.Context(), orgID, "", in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"ok": true})
		case "consent":
			var in CanonicalCustomer
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid consent json"})
			}
			u, _, err := engine.IngestCustomer(c.Context(), orgID, "", in)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, u)
		case "campaign":
			var in CanonicalCampaignActivity
			if err := json.Unmarshal(c.Body(), &in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid campaign json"})
			}
			if err := engine.PublishCampaignActivity(c.Context(), orgID, "", in); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"ok": true})
		default:
			return c.JSON(http.StatusNotFound, map[string]any{"error": "unknown publish kind"})
		}
	})
}
