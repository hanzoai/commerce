// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package woocommerce

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/source/hub"
)

// Service handles /v1/source/woocommerce/* endpoints and outbound WooCommerce sync.
type Service struct {
	Engine *hub.Engine
	Driver *Driver
	Now    func() time.Time
}

// NewService creates a WooCommerce source service and registers its OutboundDriver on engine.
func NewService(engine *hub.Engine, httpClient *http.Client) *Service {
	if engine == nil {
		engine = hub.Default()
	}
	drv := NewDriver(httpClient)
	engine.RegisterDriver(drv)
	return &Service{
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

// MountRoutes registers /v1/source/woocommerce/{connect,ingest,sync,webhook,export,erase,outbound}.
func (s *Service) MountRoutes(group *zip.Group) {
	wc := group.Group("/woocommerce")

	wc.Raw(http.MethodPost, "/connect", iammiddleware.IAMTokenRequired(), middleware.TokenRequired(), s.handleConnect)
	wc.Raw(http.MethodPost, "/ingest", s.handleIngest)
	wc.Raw(http.MethodPost, "/sync", s.handleIngest)
	wc.Raw(http.MethodPost, "/webhook", s.handleWebhook)
	wc.Raw(http.MethodPost, "/export", s.handleExport)
	wc.Raw(http.MethodPost, "/erase", s.handleErase)
	wc.Raw(http.MethodPost, "/outbound", s.handleOutbound)
}

func (s *Service) handleConnect(c *zip.Ctx) error {
	orgID := hub.ResolveRequestOrg(c)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing organization"})
	}
	var req struct {
		Shop            string `json:"shop"`
		SourceId        string `json:"source_id"`
		Domain          string `json:"domain"`
		Currency        string `json:"currency"`
		Locale          string `json:"locale"`
		Timezone        string `json:"timezone"`
		SyncMode        string `json:"sync_mode"`
		StockLocationId string `json:"stock_location_id"`
		EndpointURL     string `json:"endpoint_url"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
	}
	shop := strings.TrimSpace(req.Shop)
	if !idTokenRe.MatchString(shop) {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid shop id"})
	}

	ctx := c.Context()
	// Re-use existing secret on reconnect of the same (org, shop), or mint a fresh 64-hex secret.
	var hmacSecret string
	if existing, err := hub.LookupByShop(ctx, hub.ProviderWooCommerce, shop); err == nil {
		if existing.OrgId != orgID && existing.Status == hub.StatusActive {
			return c.JSON(http.StatusConflict, map[string]any{"error": "shop is connected to another org"})
		}
		if existing.OrgId == orgID && existing.SealedCredential != "" {
			if creds, oerr := hub.OpenCredentials(ctx, existing.OrgId, existing.SourceId, existing.SealedCredential); oerr == nil && len(creds.HMACSecret) == 64 {
				hmacSecret = creds.HMACSecret
			}
		}
	}
	if hmacSecret == "" {
		var err error
		hmacSecret, err = MintConnectionSecret()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to mint secret"})
		}
	}

	src, err := hub.RegisterSource(ctx, hub.RegisterInput{
		OrgId:           orgID,
		SourceId:        req.SourceId,
		Provider:        hub.ProviderWooCommerce,
		Shop:            shop,
		Domain:          req.Domain,
		Name:            req.Domain,
		Currency:        req.Currency,
		Locale:          req.Locale,
		Timezone:        req.Timezone,
		SyncMode:        req.SyncMode,
		StockLocationId: req.StockLocationId,
		EndpointURL:     req.EndpointURL,
		Credentials:     &hub.SourceCredentials{HMACSecret: hmacSecret},
	})
	if err != nil {
		if errors.Is(err, hub.ErrSourceConflict) {
			return c.JSON(http.StatusConflict, map[string]any{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"secret":    hmacSecret,
		"source_id": src.SourceId,
		"org":       src.OrgId,
		"sync_mode": src.SyncMode,
	})
}

func (s *Service) verifySignedPluginRequest(c *zip.Ctx) (*hub.StoreSource, string, string, error) {
	ctx := c.Context()
	shop := strings.TrimSpace(c.Header("X-Hanzo-Shop"))
	topic := strings.TrimSpace(c.Header("X-Hanzo-Topic"))
	delivery := strings.TrimSpace(c.Header("X-Hanzo-Delivery"))
	sigHdr := strings.TrimSpace(c.Header("X-Hanzo-Signature"))

	if shop == "" || sigHdr == "" {
		return nil, "", "", errors.New("missing signature headers")
	}
	src, err := hub.LookupByShop(ctx, hub.ProviderWooCommerce, shop)
	if err != nil || src.Status != hub.StatusActive {
		return nil, "", "", errors.New("unknown or inactive shop")
	}
	creds, err := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
	if err != nil || creds.HMACSecret == "" {
		return nil, "", "", errors.New("missing shop secret")
	}
	if !VerifySignatureHeader(creds.HMACSecret, sigHdr, shop, topic, delivery, c.Body(), s.now()) {
		return nil, "", "", errors.New("invalid signature")
	}
	return src, topic, delivery, nil
}

func (s *Service) handleIngest(c *zip.Ctx) error {
	src, topic, delivery, err := s.verifySignedPluginRequest(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": err.Error()})
	}
	return s.applyTopic(c, src, topic, delivery, c.Body())
}

func (s *Service) handleWebhook(c *zip.Ctx) error {
	// Support both Hanzo plugin signed requests (X-Hanzo-Signature) and native WC REST webhooks (X-WC-Webhook-Signature).
	if c.Header("X-Hanzo-Signature") != "" {
		return s.handleIngest(c)
	}
	ctx := c.Context()
	wcSig := c.Header("X-WC-Webhook-Signature")
	wcTopic := strings.ToLower(strings.TrimSpace(c.Header("X-WC-Webhook-Topic")))
	wcDelivery := strings.TrimSpace(c.Header("X-WC-Webhook-Delivery-ID"))
	shop := strings.TrimSpace(c.Header("X-Hanzo-Shop"))
	if shop == "" {
		shop = strings.TrimSpace(c.Query("shop"))
	}
	if shop == "" || wcSig == "" || wcTopic == "" {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "missing webhook signature"})
	}
	src, err := hub.LookupByShop(ctx, hub.ProviderWooCommerce, shop)
	if err != nil || src.Status != hub.StatusActive {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "unknown shop"})
	}
	creds, err := hub.OpenCredentials(ctx, src.OrgId, src.SourceId, src.SealedCredential)
	if err != nil || !VerifyWCWebhookSignature(creds.HMACSecret, wcSig, c.Body()) {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "invalid webhook hmac"})
	}
	mappedTopic := wcTopic
	switch wcTopic {
	case "product.created", "product.restored":
		mappedTopic = "product.updated"
	case "order.created":
		mappedTopic = "order.updated"
	case "customer.created":
		mappedTopic = "customer.updated"
	}
	return s.applyTopic(c, src, mappedTopic, wcDelivery, c.Body())
}

func (s *Service) applyTopic(c *zip.Ctx, src *hub.StoreSource, topic, delivery string, rawBody []byte) error {
	ctx := c.Context()
	var guard *idempotencykey.IdempotencyKey
	if delivery != "" {
		rec, replay, err := idempotencykey.Begin(hub.SystemDB(ctx), "woo-delivery", src.Shop+":"+delivery)
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

	switch topic {
	case "product.updated":
		rows, err := NormalizeProducts(rawBody, src.Currency)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		ingested, echoes := 0, 0
		for _, p := range rows {
			_, echo, err := s.Engine.IngestProduct(ctx, src.OrgId, src.SourceId, p)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			if echo {
				echoes++
			} else {
				ingested++
			}
		}
		return done(map[string]any{"ok": true, "ingested": ingested, "echoes": echoes})

	case "customer.updated":
		rows, err := NormalizeCustomers(rawBody)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		ingested, echoes := 0, 0
		for _, cust := range rows {
			_, echo, err := s.Engine.IngestCustomer(ctx, src.OrgId, src.SourceId, cust)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			if echo {
				echoes++
			} else {
				ingested++
			}
		}
		return done(map[string]any{"ok": true, "ingested": ingested, "echoes": echoes})

	case "order.updated":
		rows, err := NormalizeOrders(rawBody, src.Currency)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		ingested, echoes := 0, 0
		for _, ord := range rows {
			_, echo, err := s.Engine.IngestOrder(ctx, src.OrgId, src.SourceId, ord)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			if echo {
				echoes++
			} else {
				ingested++
			}
		}
		return done(map[string]any{"ok": true, "ingested": ingested, "echoes": echoes})

	case "product.deleted":
		var d struct{ ID any `json:"id"` }
		_ = json.Unmarshal(rawBody, &d)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityProduct, stringifyAny(d.ID))
		return done(map[string]any{"ok": true})

	case "order.deleted":
		var d struct{ ID any `json:"id"` }
		_ = json.Unmarshal(rawBody, &d)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityOrder, stringifyAny(d.ID))
		return done(map[string]any{"ok": true})

	case "customer.deleted":
		var d struct{ ID any `json:"id"` }
		_ = json.Unmarshal(rawBody, &d)
		_ = s.Engine.DeleteExternalEntity(ctx, src.OrgId, src.SourceId, hub.EntityCustomer, stringifyAny(d.ID))
		return done(map[string]any{"ok": true})

	case "customer.erased":
		var d struct {
			ID    any    `json:"id"`
			Email string `json:"email"`
		}
		_ = json.Unmarshal(rawBody, &d)
		n, _ := s.Engine.EraseCustomer(ctx, src.OrgId, src.SourceId, stringifyAny(d.ID), d.Email)
		return done(map[string]any{"ok": true, "erased": n})

	case "shop.removed":
		_ = hub.DisconnectSource(ctx, src.OrgId, src.SourceId, hub.StatusDisconnected)
		return done(map[string]any{"ok": true})

	default:
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "unsupported topic"})
	}
}

func (s *Service) handleExport(c *zip.Ctx) error {
	src, _, _, err := s.verifySignedPluginRequest(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": err.Error()})
	}
	var req struct {
		ID    any    `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
	}
	data, err := s.Engine.ExportCustomer(c.Context(), src.OrgId, src.SourceId, stringifyAny(req.ID), req.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, data)
}

func (s *Service) handleErase(c *zip.Ctx) error {
	src, _, _, err := s.verifySignedPluginRequest(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": err.Error()})
	}
	var req struct {
		ID    any    `json:"id"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
	}
	n, err := s.Engine.EraseCustomer(c.Context(), src.OrgId, src.SourceId, stringifyAny(req.ID), req.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "erased": n})
}

func (s *Service) handleOutbound(c *zip.Ctx) error {
	src, topic, _, err := s.verifySignedPluginRequest(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": err.Error()})
	}
	ctx := c.Context()
	var req struct {
		Limit int `json:"limit"`
		Ack   []struct {
			JobID      string `json:"job_id"`
			ExternalID string `json:"external_id"`
			Error      string `json:"error"`
		} `json:"ack"`
	}
	if len(c.Body()) > 0 {
		_ = json.Unmarshal(c.Body(), &req)
	}

	if topic == "outbound.ack" || len(req.Ack) > 0 {
		orgDB, err := hub.OrgDB(ctx, src.OrgId)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		}
		acked := 0
		for _, a := range req.Ack {
			job := hub.NewOutboundJob(orgDB)
			if err := job.Get(orgDB.NewKey(job.Kind(), a.JobID, 0, nil)); err != nil || job.SourceId != src.SourceId {
				continue
			}
			job.Init(orgDB)
			job.UseStringKey = true
			job.SetId(a.JobID)
			if a.Error != "" {
				job.Status = hub.JobFailed
				job.LastError = a.Error
				_ = job.Put()
				continue
			}
			if hub.MarkJobApplied(ctx, orgDB, src, job, a.ExternalID) == nil {
				acked++
			}
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "acked": acked})
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	pending, err := hub.ListPendingJobs(ctx, src.OrgId, src.SourceId, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	out := make([]OutboundEnvelope, 0, len(pending))
	for _, j := range pending {
		out = append(out, BuildEnvelope(src, j))
	}
	return c.JSON(http.StatusOK, map[string]any{"jobs": out})
}
