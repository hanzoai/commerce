// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package shopify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/source/hub"
)

var pkKeyRe = regexp.MustCompile(`^pk-[A-Za-z0-9_-]+$`)

// Service handles /v1/source/shopify/* endpoints and outbound Shopify sync.
type Service struct {
	Config Config
	Engine *hub.Engine
	Driver *Driver
	Now    func() time.Time
}

// NewService creates a Shopify source service and registers its OutboundDriver on engine.
func NewService(cfg Config, engine *hub.Engine, httpClient *http.Client) *Service {
	if engine == nil {
		engine = hub.Default()
	}
	drv := NewDriver(httpClient)
	engine.RegisterDriver(drv)
	return &Service{
		Config: cfg,
		Engine: engine,
		Driver: drv,
		Now:    time.Now,
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// MountRoutes registers /v1/source/shopify/{app,session,link,callback,pixel,webhook,flow}.
func (s *Service) MountRoutes(group *zip.Group) {
	sh := group.Group("/shopify")

	csp := func(c *zip.Ctx) {
		c.SetHeader("Content-Security-Policy", "frame-ancestors https://admin.shopify.com https://*.myshopify.com;")
	}

	sh.Raw(http.MethodGet, "/app", func(c *zip.Ctx) error {
		csp(c)
		c.SetHeader("Content-Type", "text/html; charset=utf-8")
		return c.Bytes(http.StatusOK, []byte(renderAppIndex(s.Config.clientID(c.Context()))))
	})
	sh.Raw(http.MethodGet, "/app/app.js", func(c *zip.Ctx) error {
		csp(c)
		c.SetHeader("Content-Type", "application/javascript; charset=utf-8")
		return c.Bytes(http.StatusOK, []byte(appJS))
	})
	sh.Raw(http.MethodGet, "/app/view.js", func(c *zip.Ctx) error {
		csp(c)
		c.SetHeader("Content-Type", "application/javascript; charset=utf-8")
		return c.Bytes(http.StatusOK, []byte(viewJS))
	})

	sh.Raw(http.MethodPost, "/session", s.handleSession)
	sh.Raw(http.MethodPost, "/link", s.handleLink)
	sh.Raw(http.MethodGet, "/callback", iammiddleware.IAMTokenRequired(), s.handleCallback)
	sh.Raw(http.MethodPost, "/callback", iammiddleware.IAMTokenRequired(), middleware.TokenRequired(), s.handleCallback)
	sh.Raw(http.MethodPost, "/pixel", s.handlePixel)
	sh.Raw(http.MethodPost, "/webhook", s.handleWebhook)
	sh.Raw(http.MethodPost, "/flow", s.handleFlow)
}

func (s *Service) verifyAppSession(c *zip.Ctx) (*SessionClaims, string, error) {
	ctx := c.Context()
	rawTok := strings.TrimSpace(strings.TrimPrefix(c.Header("Authorization"), "Bearer "))
	claims, err := VerifySessionToken(rawTok, s.Config.clientID(ctx), s.Config.clientSecrets(ctx), s.now())
	if err != nil {
		return nil, "", err
	}
	return claims, rawTok, nil
}

func (s *Service) sessionStatusJSON(ctx context.Context, shop string, src *hub.StoreSource) map[string]any {
	storeSlug := strings.TrimSuffix(shop, ".myshopify.com")
	clientID := s.Config.clientID(ctx)
	embedURL := ""
	if clientID != "" {
		embedURL = "https://" + shop + "/admin/themes/current/editor?context=apps&activateAppId=" + url.QueryEscape(clientID+"/hanzo")
	}
	plansURL := "https://admin.shopify.com/store/" + url.PathEscape(storeSlug) + "/charges/hanzo/pricing_plans"

	if src == nil || src.OrgId == "" || src.Status == hub.StatusUnlinked || src.Status == hub.StatusDisconnected || src.Status == hub.StatusUninstalled {
		return map[string]any{
			"shop":  shop,
			"org":   "",
			"pixel": false,
			"embed": embedURL,
			"plan":  map[string]any{"handle": "", "active": false},
			"plans": plansURL,
			"sync":  []any{},
		}
	}

	backfillStatus := src.LastBackfillStatus
	if backfillStatus == "" {
		backfillStatus = "COMPLETED"
	}
	syncRows := []map[string]any{
		{"kind": "product", "status": backfillStatus, "count": src.ProductsRead, "written": src.ProductsWritten},
		{"kind": "customer", "status": backfillStatus, "count": src.CustomersRead, "written": src.CustomersWritten},
		{"kind": "order", "status": backfillStatus, "count": src.OrdersRead, "written": src.OrdersWritten},
	}
	return map[string]any{
		"shop":              shop,
		"org":               src.OrgId,
		"source_id":         src.SourceId,
		"sync_mode":         src.SyncMode,
		"stock_location_id": src.StockLocationId,
		"currency":          src.Currency,
		"pixel":             src.PixelKey != "",
		"embed":             embedURL,
		"plan": map[string]any{
			"handle": src.PlanHandle,
			"active": src.PlanActive,
		},
		"plans": plansURL,
		"sync":  syncRows,
		"counters": map[string]int{
			"products_read":     src.ProductsRead,
			"products_written":  src.ProductsWritten,
			"customers_read":    src.CustomersRead,
			"customers_written": src.CustomersWritten,
			"orders_read":       src.OrdersRead,
			"orders_written":    src.OrdersWritten,
			"inventory_written": src.InventoryWritten,
			"coupons_written":   src.CouponsWritten,
		},
	}
}

func (s *Service) handleSession(c *zip.Ctx) error {
	claims, rawJWT, err := s.verifyAppSession(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"message": "Invalid Shopify session token."})
	}
	ctx := c.Context()
	src, _ := hub.LookupByShop(ctx, hub.ProviderShopify, claims.Shop)
	if src != nil && src.OrgId != "" {
		creds, _ := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
		if creds.AccessToken == "" {
			secretsList := s.Config.clientSecrets(ctx)
			if cid := s.Config.clientID(ctx); cid != "" && len(secretsList) > 0 {
				if tok, exErr := s.Driver.ExchangeOfflineToken(ctx, claims.Shop, src.EndpointURL, cid, secretsList[0], rawJWT); exErr == nil && tok != "" {
					src, _ = hub.RegisterSource(ctx, hub.RegisterInput{
						OrgId:       src.OrgId,
						SourceId:    src.SourceId,
						Provider:    hub.ProviderShopify,
						Shop:        claims.Shop,
						Credentials: &hub.SourceCredentials{AccessToken: tok},
					})
				}
			}
		}
	}
	return c.JSON(http.StatusOK, s.sessionStatusJSON(ctx, claims.Shop, src))
}

func (s *Service) handleLink(c *zip.Ctx) error {
	claims, _, err := s.verifyAppSession(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"message": "Invalid Shopify session token."})
	}
	secretsList := s.Config.clientSecrets(c.Context())
	if len(secretsList) == 0 {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"message": "Shopify app secret is not configured."})
	}
	state, err := SignLinkState(claims.Shop, secretsList[0], s.now())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"message": "Could not create link state."})
	}
	authURL := s.Config.IAMAuthorizeURL
	if authURL == "" {
		authURL = "https://hanzo.id/login/oauth/authorize"
	}
	iamClient := s.Config.IAMClientID
	if iamClient == "" {
		iamClient = "hanzo-shopify"
	}
	redirectURI := s.Config.publicBaseURL() + "/v1/source/shopify/callback"
	u, _ := url.Parse(authURL)
	q := u.Query()
	q.Set("client_id", iamClient)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return c.JSON(http.StatusOK, map[string]any{"url": u.String(), "state": state})
}

func (s *Service) handleCallback(c *zip.Ctx) error {
	ctx := c.Context()
	state := c.Query("state")
	var bodyReq struct {
		State              string `json:"state"`
		Org                string `json:"org"`
		SourceId           string `json:"sourceId"`
		SyncMode           string `json:"syncMode"`
		StockLocationId    string `json:"stockLocationId"`
		ExternalLocationId string `json:"externalLocationId"`
		Currency           string `json:"currency"`
		EndpointURL        string `json:"endpointUrl"`
		AccessToken        string `json:"accessToken"`
	}
	if len(c.Body()) > 0 {
		_ = json.Unmarshal(c.Body(), &bodyReq)
		if state == "" {
			state = bodyReq.State
		}
	}
	shop, err := VerifyLinkState(state, s.Config.clientSecrets(ctx), s.now())
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"message": "Invalid or expired link state."})
	}

	orgID := hub.ResolveRequestOrg(c)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]any{"message": "Sign in to a Hanzo organization to complete store linking."})
	}

	var creds *hub.SourceCredentials
	if bodyReq.AccessToken != "" {
		creds = &hub.SourceCredentials{AccessToken: bodyReq.AccessToken}
	}
	src, err := hub.RegisterSource(ctx, hub.RegisterInput{
		OrgId:              orgID,
		SourceId:           bodyReq.SourceId,
		Provider:           hub.ProviderShopify,
		Shop:               shop,
		SyncMode:           bodyReq.SyncMode,
		StockLocationId:    bodyReq.StockLocationId,
		ExternalLocationId: bodyReq.ExternalLocationId,
		Currency:           bodyReq.Currency,
		EndpointURL:        bodyReq.EndpointURL,
		Credentials:        creds,
	})
	if err != nil {
		if errors.Is(err, hub.ErrSourceConflict) {
			return c.JSON(http.StatusConflict, map[string]any{"message": "This Shopify store is already connected to another Hanzo organization."})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
	}
	src.LastBackfillStatus = "COMPLETED"
	src.SetId(hub.StoreSourceStorageID(src.SourceId))
	_ = src.Put()

	if c.Method() == http.MethodPost {
		return c.JSON(http.StatusOK, s.sessionStatusJSON(ctx, shop, src))
	}
	storeSlug := strings.TrimSuffix(shop, ".myshopify.com")
	return c.Redirect(http.StatusFound, "https://admin.shopify.com/store/"+url.PathEscape(storeSlug)+"/apps/hanzo")
}

func (s *Service) handlePixel(c *zip.Ctx) error {
	claims, _, err := s.verifyAppSession(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"message": "Invalid Shopify session token."})
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"message": "Invalid JSON body."})
	}
	pk := strings.TrimSpace(req.Key)
	if !pkKeyRe.MatchString(pk) {
		return c.JSON(http.StatusBadRequest, map[string]any{"message": "That is not a publishable pk- key."})
	}

	ctx := c.Context()
	src, err := hub.LookupByShop(ctx, hub.ProviderShopify, claims.Shop)
	if err != nil || src.OrgId == "" {
		return c.JSON(http.StatusConflict, map[string]any{"message": "Connect this store to a Hanzo account first."})
	}
	creds, _ := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
	pixelID, err := s.Driver.ActivatePixel(ctx, src, creds, pk)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]any{"message": err.Error()})
	}
	src.PixelKey = pk
	src.PixelExternalId = pixelID
	src.SetId(hub.StoreSourceStorageID(src.SourceId))
	_ = src.Put()

	return c.JSON(http.StatusOK, s.sessionStatusJSON(ctx, claims.Shop, src))
}

func (s *Service) candidateWebhookSecrets(ctx context.Context, src *hub.StoreSource) []string {
	list := s.Config.clientSecrets(ctx)
	if src != nil && src.SealedCredential != "" {
		if creds, err := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential); err == nil && creds.WebhookSecret != "" {
			list = append(list, creds.WebhookSecret)
		}
	}
	return list
}

func (s *Service) handleWebhook(c *zip.Ctx) error {
	ctx := c.Context()
	rawBody := c.Body()
	hmacHdr := c.Header("X-Shopify-Hmac-Sha256")
	shopHdr := strings.ToLower(strings.TrimSpace(c.Header("X-Shopify-Shop-Domain")))
	topic := strings.ToLower(strings.TrimSpace(c.Header("X-Shopify-Topic")))
	webhookID := strings.TrimSpace(c.Header("X-Shopify-Webhook-Id"))

	var src *hub.StoreSource
	if shopHdr != "" {
		src, _ = hub.LookupByShop(ctx, hub.ProviderShopify, shopHdr)
	}
	// Mandatory Shopify App Store compliance: 401 on missing or invalid HMAC!
	if !VerifyWebhookHMAC(hmacHdr, rawBody, s.candidateWebhookSecrets(ctx, src)...) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid webhook hmac"})
	}

	// Deduplicate webhook deliveries by X-Shopify-Webhook-Id in the system store.
	var guard *idempotencykey.IdempotencyKey
	if webhookID != "" {
		rec, replay, err := idempotencykey.Begin(hub.SystemDB(ctx), "shopify-webhook", shopHdr+":"+webhookID)
		if err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "guard unavailable"})
		}
		if replay {
			return c.JSON(http.StatusOK, map[string]any{"ok": true, "duplicate": true})
		}
		guard = rec
	}
	done := func(payload map[string]any) error {
		if guard != nil {
			_ = idempotencykey.Complete(guard, "ok")
		}
		return c.JSON(http.StatusOK, payload)
	}

	// Mandatory privacy compliance topics succeed with 200 even if the shop is already unlinked.
	switch topic {
	case "customers/data_request":
		if src != nil && src.OrgId != "" {
			var dr struct {
				Customer struct {
					ID    any    `json:"id"`
					Email string `json:"email"`
				} `json:"customer"`
			}
			_ = json.Unmarshal(rawBody, &dr)
			data, _ := s.Engine.ExportCustomer(ctx, src.OrgId, src.SourceId, stringifyID(dr.Customer.ID), dr.Customer.Email)
			return done(map[string]any{"ok": true, "export": data})
		}
		return done(map[string]any{"ok": true})

	case "customers/redact":
		if src != nil && src.OrgId != "" {
			var rr struct {
				Customer struct {
					ID    any    `json:"id"`
					Email string `json:"email"`
				} `json:"customer"`
			}
			_ = json.Unmarshal(rawBody, &rr)
			n, _ := s.Engine.EraseCustomer(ctx, src.OrgId, src.SourceId, stringifyID(rr.Customer.ID), rr.Customer.Email)
			return done(map[string]any{"ok": true, "erased": n})
		}
		return done(map[string]any{"ok": true})

	case "shop/redact", "app/uninstalled":
		if src != nil && src.OrgId != "" {
			_ = hub.DisconnectSource(ctx, src.OrgId, src.SourceId, hub.StatusUninstalled)
		}
		return done(map[string]any{"ok": true})
	}

	if src == nil || src.OrgId == "" || src.Status != hub.StatusActive {
		return done(map[string]any{"ok": true, "ignored": "unlinked"})
	}

	switch topic {
	case "app_subscriptions/update":
		var sub struct {
			AppSubscription struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"app_subscription"`
		}
		_ = json.Unmarshal(rawBody, &sub)
		src.PlanHandle = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(sub.AppSubscription.Name), " ", "_"))
		src.PlanActive = strings.EqualFold(sub.AppSubscription.Status, "ACTIVE")
		src.SetId(hub.StoreSourceStorageID(src.SourceId))
		_ = src.Put()
		return done(map[string]any{"ok": true})

	case "bulk_operations/finish":
		src.LastBackfillStatus = "COMPLETED"
		src.LastSyncedAt = s.now().UTC()
		src.SetId(hub.StoreSourceStorageID(src.SourceId))
		_ = src.Put()
		return done(map[string]any{"ok": true})

	case "products/create", "products/update":
		cp, err := NormalizeProduct(rawBody, src.Currency)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		_, echo, err := s.Engine.IngestProduct(ctx, src.OrgId, src.SourceId, cp)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return done(map[string]any{"ok": true, "echo": echo})

	case "products/delete":
		var del struct {
			ID any `json:"id"`
		}
		_ = json.Unmarshal(rawBody, &del)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityProduct, stringifyID(del.ID))
		return done(map[string]any{"ok": true})

	case "orders/paid", "orders/updated":
		co, err := NormalizeOrder(rawBody, src.Currency, topic == "orders/paid")
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		_, echo, err := s.Engine.IngestOrder(ctx, src.OrgId, src.SourceId, co)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return done(map[string]any{"ok": true, "echo": echo})

	case "orders/delete":
		var del struct {
			ID any `json:"id"`
		}
		_ = json.Unmarshal(rawBody, &del)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityOrder, stringifyID(del.ID))
		return done(map[string]any{"ok": true})

	case "customers/create", "customers/update", "customers_email_marketing_consent/update":
		cc, err := NormalizeCustomer(rawBody)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		_, echo, err := s.Engine.IngestCustomer(ctx, src.OrgId, src.SourceId, cc)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		return done(map[string]any{"ok": true, "echo": echo})

	case "customers/delete":
		var del struct {
			ID any `json:"id"`
		}
		_ = json.Unmarshal(rawBody, &del)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityCustomer, stringifyID(del.ID))
		return done(map[string]any{"ok": true})
	}

	return done(map[string]any{"ok": true})
}

func (s *Service) handleFlow(c *zip.Ctx) error {
	ctx := c.Context()
	rawBody := c.Body()
	hmacHdr := c.Header("X-Shopify-Hmac-Sha256")

	var payload struct {
		ShopDomain  string `json:"shopify_domain"`
		Handle      string `json:"handle"`
		ActionRunID string `json:"action_run_id"`
		Properties  struct {
			CustomerID string `json:"customer_id"`
		} `json:"properties"`
	}
	_ = json.Unmarshal(rawBody, &payload)

	var src *hub.StoreSource
	if payload.ShopDomain != "" {
		src, _ = hub.LookupByShop(ctx, hub.ProviderShopify, strings.ToLower(payload.ShopDomain))
	}
	if !VerifyWebhookHMAC(hmacHdr, rawBody, s.candidateWebhookSecrets(ctx, src)...) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid flow hmac"})
	}
	if payload.ActionRunID != "" {
		rec, replay, err := idempotencykey.Begin(hub.SystemDB(ctx), "shopify-flow", payload.ActionRunID)
		if err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "guard unavailable"})
		}
		if replay {
			return c.JSON(http.StatusOK, map[string]any{"ok": true, "duplicate": true})
		}
		defer func() { _ = idempotencykey.Complete(rec, "ok") }()
	}
	if src == nil || src.OrgId == "" || src.Status != hub.StatusActive {
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "ignored": "unlinked"})
	}
	if payload.Handle == "" || payload.Handle == "hanzo-send-customer" {
		creds, _ := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
		if creds.AccessToken != "" && payload.Properties.CustomerID != "" {
			if cc, err := s.Driver.FetchCustomer(ctx, src, creds, payload.Properties.CustomerID); err == nil {
				_, _, _ = s.Engine.IngestCustomer(ctx, src.OrgId, src.SourceId, cc)
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}
