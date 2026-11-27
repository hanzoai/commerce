// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package woocommerce

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hanzoai/outbound"

	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/shopify"
)

var safeClient = outbound.Client(outbound.Options{Timeout: 20 * time.Second, Limit: 2 << 20})

// OutboundEnvelope is the JSON item shape delivered to hanzo/woocommerce via
// either direct signed webhook push (/wp-json/hanzo/v1/apply) or plugin pull
// (/v1/source/woocommerce/outbound).
type OutboundEnvelope struct {
	JobID       string          `json:"job_id"`
	Operation   string          `json:"operation"`
	EntityKind  string          `json:"entity_kind"`
	CanonicalID string          `json:"canonical_id"`
	ExternalID  string          `json:"external_id,omitempty"`
	SKU         string          `json:"sku,omitempty"`
	SyncedHash  string          `json:"synced_hash"`
	Data        json.RawMessage `json:"data"`
}

// BuildEnvelope converts a hub.OutboundJob into the WooCommerce plugin's wire shape
// (with formatted decimal prices matching WooCommerce REST v3).
func BuildEnvelope(src *hub.StoreSource, job *hub.OutboundJob) OutboundEnvelope {
	cur := src.Currency
	if cur == "" {
		cur = "USD"
	}
	data := json.RawMessage(job.Payload)

	switch job.Operation {
	case hub.OpProductUpsert:
		var cp hub.CanonicalProduct
		if json.Unmarshal([]byte(job.Payload), &cp) == nil {
			if cp.Currency != "" {
				cur = cp.Currency
			}
			vars := make([]map[string]any, 0, len(cp.Variants))
			for _, v := range cp.Variants {
				vars = append(vars, map[string]any{
					"external_id":    v.ExternalId,
					"sku":            v.SKU,
					"name":           v.Name,
					"regular_price":  shopify.FormatMoneyDecimal(v.PriceCents, cur),
					"manage_stock":   v.ManageStock,
					"stock_quantity": v.Inventory,
					"status":          map[bool]string{true: "publish", false: "private"}[v.Available],
				})
			}
			pType := "simple"
			if len(vars) > 0 {
				pType = "variable"
			}
			pStatus := "publish"
			if cp.Hidden || !cp.Available {
				pStatus = "draft"
			}
			shaped, _ := json.Marshal(map[string]any{
				"external_id":    cp.ExternalId,
				"name":           cp.Name,
				"slug":           cp.Slug,
				"type":           pType,
				"status":         pStatus,
				"sku":            cp.SKU,
				"regular_price":  shopify.FormatMoneyDecimal(cp.PriceCents, cur),
				"manage_stock":   cp.ManageStock && len(vars) == 0,
				"stock_quantity": cp.Inventory,
				"currency":       cur,
				"variations":     vars,
			})
			data = shaped
		}
	case hub.OpCouponUpsert:
		var cc hub.CanonicalCoupon
		if json.Unmarshal([]byte(job.Payload), &cc) == nil {
			wcType := "percent"
			amt := fmt.Sprintf("%d", cc.Amount)
			switch cc.Type {
			case "flat":
				wcType = "fixed_cart"
				amt = shopify.FormatMoneyDecimal(shopify.ParseMoneyCents(fmt.Sprintf("%.2f", float64(cc.Amount)/100.0), cur), cur)
			case "free-shipping":
				wcType = "free_shipping"
				amt = "0"
			}
			shaped, _ := json.Marshal(map[string]any{
				"external_id":    cc.ExternalId,
				"code":           strings.ToLower(cc.Code),
				"description":    cc.Name,
				"discount_type":  wcType,
				"amount":         amt,
				"usage_limit":    cc.Limit,
				"individual_use": cc.Once,
				"enabled":        cc.Enabled,
			})
			data = shaped
		}
	}

	return OutboundEnvelope{
		JobID:       job.JobId,
		Operation:   job.Operation,
		EntityKind:  job.EntityKind,
		CanonicalID: job.CanonicalId,
		ExternalID:  job.ExternalId,
		SKU:         job.SKU,
		SyncedHash:  job.PayloadHash,
		Data:        data,
	}
}

// Driver implements hub.OutboundDriver for WooCommerce stores.
type Driver struct {
	HTTPClient *http.Client
}

func NewDriver(httpClient *http.Client) *Driver {
	return &Driver{HTTPClient: httpClient}
}

func (d *Driver) Provider() string { return hub.ProviderWooCommerce }

func (d *Driver) client() *http.Client {
	if d != nil && d.HTTPClient != nil {
		return d.HTTPClient
	}
	return safeClient
}

// Deliver pushes immediately when EndpointURL or Woo REST credentials are configured,
// and otherwise leaves the job in the signed pull queue (/v1/source/woocommerce/outbound).
func (d *Driver) Deliver(ctx context.Context, src *hub.StoreSource, creds hub.SourceCredentials, job *hub.OutboundJob) (hub.OutboundResult, error) {
	if src.EndpointURL != "" && creds.HMACSecret != "" {
		return d.pushSignedPluginCallback(ctx, src, creds.HMACSecret, job)
	}
	if creds.ConsumerKey != "" && creds.ConsumerSecret != "" && src.Domain != "" {
		return d.pushWCRestV3(ctx, src, creds, job)
	}
	return hub.OutboundResult{Queued: true}, nil
}

func (d *Driver) pushSignedPluginCallback(ctx context.Context, src *hub.StoreSource, secret string, job *hub.OutboundJob) (hub.OutboundResult, error) {
	env := BuildEnvelope(src, job)
	body, err := json.Marshal(map[string]any{"jobs": []OutboundEnvelope{env}})
	if err != nil {
		return hub.OutboundResult{}, err
	}
	sum := sha256.Sum256(append([]byte("outbound.apply\n"), body...))
	delivery := hex.EncodeToString(sum[:])
	nowUnix := time.Now().Unix()

	target := strings.TrimRight(src.EndpointURL, "/")
	if !strings.Contains(target, "/wp-json/") {
		target += "/wp-json/hanzo/v1/apply"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return hub.OutboundResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hanzo-Shop", src.Shop)
	req.Header.Set("X-Hanzo-Topic", "outbound.apply")
	req.Header.Set("X-Hanzo-Delivery", delivery)
	req.Header.Set("X-Hanzo-Signature", FormatSignatureHeader(secret, nowUnix, src.Shop, "outbound.apply", delivery, body))

	resp, err := d.client().Do(req)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return hub.OutboundResult{}, fmt.Errorf("woocommerce apply callback status %d", resp.StatusCode)
	}
	var res struct {
		Applied []struct {
			JobID      string `json:"job_id"`
			ExternalID string `json:"external_id"`
		} `json:"applied"`
	}
	_ = json.Unmarshal(respBytes, &res)
	extID := job.ExternalId
	for _, a := range res.Applied {
		if a.JobID == job.JobId && a.ExternalID != "" {
			extID = a.ExternalID
		}
	}
	return hub.OutboundResult{ExternalId: extID}, nil
}

func (d *Driver) pushWCRestV3(ctx context.Context, src *hub.StoreSource, creds hub.SourceCredentials, job *hub.OutboundJob) (hub.OutboundResult, error) {
	base := src.EndpointURL
	if base == "" {
		base = "https://" + src.Domain
	}
	base = strings.TrimRight(base, "/") + "/wp-json/wc/v3"
	env := BuildEnvelope(src, job)

	var method, path string
	payload := env.Data
	switch job.Operation {
	case hub.OpProductUpsert:
		if job.ExternalId != "" {
			method, path = http.MethodPut, "/products/"+job.ExternalId
		} else {
			method, path = http.MethodPost, "/products"
		}
	case hub.OpInventorySet:
		if job.ExternalId == "" {
			return hub.OutboundResult{Queued: true}, nil
		}
		var inv hub.CanonicalInventoryUpdate
		_ = json.Unmarshal([]byte(job.Payload), &inv)
		payload, _ = json.Marshal(map[string]any{"manage_stock": true, "stock_quantity": inv.Quantity})
		method, path = http.MethodPut, "/products/"+job.ExternalId
	case hub.OpCouponUpsert:
		if job.ExternalId != "" {
			method, path = http.MethodPut, "/coupons/"+job.ExternalId
		} else {
			method, path = http.MethodPost, "/coupons"
		}
	case hub.OpFulfillmentCreate:
		if job.ExternalId == "" {
			return hub.OutboundResult{Queued: true}, nil
		}
		var cf hub.CanonicalFulfillment
		_ = json.Unmarshal([]byte(job.Payload), &cf)
		payload, _ = json.Marshal(map[string]any{
			"status": "completed",
			"meta_data": []map[string]string{
				{"key": "_hanzo_tracking_number", "value": cf.TrackingNumber},
				{"key": "_hanzo_tracking_carrier", "value": cf.Carrier},
				{"key": "_hanzo_tracking_url", "value": cf.TrackingURL},
			},
		})
		method, path = http.MethodPut, "/orders/"+job.ExternalId
	default:
		return hub.OutboundResult{Queued: true}, nil
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
	if err != nil {
		return hub.OutboundResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(creds.ConsumerKey, creds.ConsumerSecret)
	resp, err := d.client().Do(req)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return hub.OutboundResult{}, fmt.Errorf("woocommerce rest v3 status %d", resp.StatusCode)
	}
	var obj struct {
		ID any `json:"id"`
	}
	_ = json.Unmarshal(raw, &obj)
	return hub.OutboundResult{ExternalId: stringifyAny(obj.ID)}, nil
}
