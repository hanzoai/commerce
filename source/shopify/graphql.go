// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hanzoai/outbound"

	"github.com/hanzoai/commerce/source/hub"
)

var safeClient = outbound.Client(outbound.Options{Timeout: 20 * time.Second, Limit: 2 << 20})

// Driver implements hub.OutboundDriver and Shopify Admin GraphQL 2026-10 operations.
type Driver struct {
	HTTPClient *http.Client
}

func NewDriver(httpClient *http.Client) *Driver {
	return &Driver{HTTPClient: httpClient}
}

func (d *Driver) Provider() string { return hub.ProviderShopify }

func (d *Driver) client() *http.Client {
	if d != nil && d.HTTPClient != nil {
		return d.HTTPClient
	}
	return safeClient
}

func graphqlEndpoint(src *hub.StoreSource) string {
	if src.EndpointURL != "" {
		return strings.TrimRight(src.EndpointURL, "/") + "/admin/api/" + APIVersion + "/graphql.json"
	}
	return "https://" + src.Shop + "/admin/api/" + APIVersion + "/graphql.json"
}

func oauthEndpoint(shop, overrideBase string) string {
	if overrideBase != "" {
		return strings.TrimRight(overrideBase, "/") + "/admin/oauth/access_token"
	}
	return "https://" + shop + "/admin/oauth/access_token"
}

// ExchangeOfflineToken exchanges an App Bridge session JWT for an offline access token.
func (d *Driver) ExchangeOfflineToken(ctx context.Context, shop, overrideBase, clientID, clientSecret, sessionJWT string) (string, error) {
	payload, _ := json.Marshal(map[string]string{
		"client_id":            clientID,
		"client_secret":        clientSecret,
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"subject_token":        sessionJWT,
		"subject_token_type":   "urn:ietf:params:oauth:token-type:id_token",
		"requested_token_type": "urn:shopify:params:oauth:token-type:offline-access-token",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthEndpoint(shop, overrideBase), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := d.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("shopify token exchange status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", errors.New("shopify token exchange returned empty token")
	}
	return out.AccessToken, nil
}

// GraphQL executes a single Shopify Admin GraphQL query or mutation.
func (d *Driver) GraphQL(ctx context.Context, src *hub.StoreSource, accessToken, query string, variables map[string]any, out any) error {
	if accessToken == "" {
		return errors.New("shopify: missing offline access token")
	}
	reqBody, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlEndpoint(src), bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	resp, err := d.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("shopify graphql status %d", resp.StatusCode)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("shopify graphql: %s", env.Errors[0].Message)
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// ActivatePixel calls webPixelCreate or webPixelUpdate with the org's pk- ingest key.
func (d *Driver) ActivatePixel(ctx context.Context, src *hub.StoreSource, creds hub.SourceCredentials, pk string) (string, error) {
	if creds.AccessToken == "" {
		// In unconfigured/local mode without an offline token yet, record locally.
		return "gid://shopify/WebPixel/local", nil
	}
	settingsJSON, _ := json.Marshal(map[string]string{"key": pk})
	if src.PixelExternalId != "" {
		const mut = `mutation webPixelUpdate($id: ID!, $webPixel: WebPixelInput!) {
			webPixelUpdate(id: $id, webPixel: $webPixel) {
				webPixel { id }
				userErrors { field message }
			}
		}`
		var res struct {
			WebPixelUpdate struct {
				WebPixel   struct{ ID string } `json:"webPixel"`
				UserErrors []struct{ Message string } `json:"userErrors"`
			} `json:"webPixelUpdate"`
		}
		err := d.GraphQL(ctx, src, creds.AccessToken, mut, map[string]any{
			"id":       src.PixelExternalId,
			"webPixel": map[string]any{"settings": string(settingsJSON)},
		}, &res)
		if err != nil {
			return "", err
		}
		if len(res.WebPixelUpdate.UserErrors) > 0 {
			return "", errors.New(res.WebPixelUpdate.UserErrors[0].Message)
		}
		return res.WebPixelUpdate.WebPixel.ID, nil
	}
	const mut = `mutation webPixelCreate($webPixel: WebPixelInput!) {
		webPixelCreate(webPixel: $webPixel) {
			webPixel { id }
			userErrors { field message }
		}
	}`
	var res struct {
		WebPixelCreate struct {
			WebPixel   struct{ ID string } `json:"webPixel"`
			UserErrors []struct{ Message string } `json:"userErrors"`
		} `json:"webPixelCreate"`
	}
	err := d.GraphQL(ctx, src, creds.AccessToken, mut, map[string]any{
		"webPixel": map[string]any{"settings": string(settingsJSON)},
	}, &res)
	if err != nil {
		return "", err
	}
	if len(res.WebPixelCreate.UserErrors) > 0 {
		return "", errors.New(res.WebPixelCreate.UserErrors[0].Message)
	}
	return res.WebPixelCreate.WebPixel.ID, nil
}

// FetchCustomer queries a single customer and email consent from Shopify Admin GraphQL.
func (d *Driver) FetchCustomer(ctx context.Context, src *hub.StoreSource, creds hub.SourceCredentials, customerID string) (hub.CanonicalCustomer, error) {
	gid := customerID
	if !strings.HasPrefix(gid, "gid://") {
		gid = "gid://shopify/Customer/" + stripGID(customerID)
	}
	const q = `query getCustomer($id: ID!) {
		customer(id: $id) {
			id
			firstName
			lastName
			defaultEmailAddress {
				emailAddress
				marketingState
				marketingUpdatedAt
			}
		}
	}`
	var res struct {
		Customer *struct {
			ID                 string `json:"id"`
			FirstName          string `json:"firstName"`
			LastName           string `json:"lastName"`
			DefaultEmailAddress *struct {
				EmailAddress       string `json:"emailAddress"`
				MarketingState     string `json:"marketingState"`
				MarketingUpdatedAt string `json:"marketingUpdatedAt"`
			} `json:"defaultEmailAddress"`
		} `json:"customer"`
	}
	if err := d.GraphQL(ctx, src, creds.AccessToken, q, map[string]any{"id": gid}, &res); err != nil {
		return hub.CanonicalCustomer{}, err
	}
	if res.Customer == nil {
		return hub.CanonicalCustomer{}, errors.New("shopify: customer not found")
	}
	consent := "unknown"
	email := ""
	updatedAt := ""
	if res.Customer.DefaultEmailAddress != nil {
		email = strings.ToLower(strings.TrimSpace(res.Customer.DefaultEmailAddress.EmailAddress))
		switch strings.ToUpper(res.Customer.DefaultEmailAddress.MarketingState) {
		case "SUBSCRIBED":
			consent = "subscribed"
		case "UNSUBSCRIBED":
			consent = "unsubscribed"
		}
		updatedAt = res.Customer.DefaultEmailAddress.MarketingUpdatedAt
	}
	return hub.CanonicalCustomer{
		ExternalId:       stripGID(res.Customer.ID),
		Email:            email,
		FirstName:        res.Customer.FirstName,
		LastName:         res.Customer.LastName,
		MarketingConsent: consent,
		ConsentUpdatedAt: updatedAt,
	}, nil
}

// Deliver dispatches an OutboundJob to Shopify Admin GraphQL 2026-10.
func (d *Driver) Deliver(ctx context.Context, src *hub.StoreSource, creds hub.SourceCredentials, job *hub.OutboundJob) (hub.OutboundResult, error) {
	if creds.AccessToken == "" {
		return hub.OutboundResult{Queued: true}, nil
	}
	switch job.Operation {
	case hub.OpProductUpsert:
		var cp hub.CanonicalProduct
		if err := json.Unmarshal([]byte(job.Payload), &cp); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushProduct(ctx, src, creds.AccessToken, cp)
	case hub.OpInventorySet:
		var inv hub.CanonicalInventoryUpdate
		if err := json.Unmarshal([]byte(job.Payload), &inv); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushInventory(ctx, src, creds.AccessToken, inv)
	case hub.OpCouponUpsert:
		var cc hub.CanonicalCoupon
		if err := json.Unmarshal([]byte(job.Payload), &cc); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushCoupon(ctx, src, creds.AccessToken, cc)
	case hub.OpFulfillmentCreate:
		var cf hub.CanonicalFulfillment
		if err := json.Unmarshal([]byte(job.Payload), &cf); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushFulfillment(ctx, src, creds.AccessToken, cf)
	case hub.OpRefundCreate:
		var cr hub.CanonicalRefund
		if err := json.Unmarshal([]byte(job.Payload), &cr); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushRefund(ctx, src, creds.AccessToken, cr)
	case hub.OpCustomerConsent:
		var cust hub.CanonicalCustomer
		if err := json.Unmarshal([]byte(job.Payload), &cust); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushConsent(ctx, src, creds.AccessToken, cust)
	case hub.OpCampaignActivity:
		var act hub.CanonicalCampaignActivity
		if err := json.Unmarshal([]byte(job.Payload), &act); err != nil {
			return hub.OutboundResult{}, err
		}
		return d.pushCampaign(ctx, src, creds.AccessToken, act)
	default:
		return hub.OutboundResult{}, nil
	}
}

func (d *Driver) pushProduct(ctx context.Context, src *hub.StoreSource, token string, cp hub.CanonicalProduct) (hub.OutboundResult, error) {
	status := "ACTIVE"
	if cp.Hidden || !cp.Available {
		status = "DRAFT"
	}
	cur := cp.Currency
	if cur == "" {
		cur = src.Currency
	}
	variants := make([]map[string]any, 0, max(1, len(cp.Variants)))
	if len(cp.Variants) == 0 {
		variants = append(variants, map[string]any{
			"sku":         cp.SKU,
			"price":       FormatMoneyDecimal(cp.PriceCents, cur),
			"optionValues": []map[string]string{{"optionName": "Title", "name": "Default Title"}},
		})
	} else {
		for _, v := range cp.Variants {
			vName := v.Name
			if vName == "" {
				vName = v.SKU
			}
			variants = append(variants, map[string]any{
				"sku":          v.SKU,
				"price":        FormatMoneyDecimal(v.PriceCents, cur),
				"optionValues": []map[string]string{{"optionName": "Title", "name": vName}},
			})
		}
	}
	input := map[string]any{
		"title":           cp.Name,
		"handle":          cp.Slug,
		"descriptionHtml": cp.Description,
		"status":          status,
		"productOptions":  []map[string]any{{"name": "Title", "values": []map[string]string{{"name": "Default Title"}}}},
		"variants":        variants,
	}
	if cp.ExternalId != "" {
		gid := cp.ExternalId
		if !strings.HasPrefix(gid, "gid://") {
			gid = "gid://shopify/Product/" + gid
		}
		input["id"] = gid
	}
	const mut = `mutation productSet($input: ProductSetInput!, $synchronous: Boolean!) {
		productSet(input: $input, synchronous: $synchronous) {
			product { id }
			userErrors { field message }
		}
	}`
	var res struct {
		ProductSet struct {
			Product    struct{ ID string }        `json:"product"`
			UserErrors []struct{ Message string } `json:"userErrors"`
		} `json:"productSet"`
	}
	if err := d.GraphQL(ctx, src, token, mut, map[string]any{"input": input, "synchronous": true}, &res); err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.ProductSet.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.ProductSet.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.ProductSet.Product.ID)}, nil
}

func (d *Driver) pushInventory(ctx context.Context, src *hub.StoreSource, token string, inv hub.CanonicalInventoryUpdate) (hub.OutboundResult, error) {
	locGID := inv.ExternalLocationId
	if locGID == "" {
		locGID = "gid://shopify/Location/1"
	} else if !strings.HasPrefix(locGID, "gid://") {
		locGID = "gid://shopify/Location/" + locGID
	}
	itemGID := inv.ExternalVariantId
	if itemGID == "" {
		itemGID = "gid://shopify/InventoryItem/" + inv.SKU
	} else if !strings.HasPrefix(itemGID, "gid://") {
		itemGID = "gid://shopify/InventoryItem/" + itemGID
	}
	const mut = `mutation inventorySetQuantities($input: InventorySetQuantitiesInput!) {
		inventorySetQuantities(input: $input) {
			inventoryAdjustmentGroup { id }
			userErrors { field message }
		}
	}`
	var res struct {
		InventorySetQuantities struct {
			UserErrors []struct{ Message string } `json:"userErrors"`
		} `json:"inventorySetQuantities"`
	}
	err := d.GraphQL(ctx, src, token, mut, map[string]any{
		"input": map[string]any{
			"name":                  "available",
			"reason":                "correction",
			"ignoreCompareQuantity": true,
			"quantities": []map[string]any{{
				"inventoryItemId": itemGID,
				"locationId":      locGID,
				"quantity":        inv.Quantity,
				"sku":             inv.SKU,
			}},
		},
	}, &res)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.InventorySetQuantities.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.InventorySetQuantities.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(itemGID)}, nil
}

func (d *Driver) pushCoupon(ctx context.Context, src *hub.StoreSource, token string, cc hub.CanonicalCoupon) (hub.OutboundResult, error) {
	var value map[string]any
	if cc.Type == "flat" {
		cur := src.Currency
		if cur == "" {
			cur = "USD"
		}
		value = map[string]any{
			"discountAmount": map[string]any{
				"amount":            FormatMoneyDecimal( ParseMoneyCents(fmt.Sprintf("%.2f", float64(cc.Amount)/100.0), cur), cur),
				"appliesOnEachItem": false,
			},
		}
	} else {
		value = map[string]any{"percentage": float64(cc.Amount) / 100.0}
	}
	basic := map[string]any{
		"title":                cc.Name,
		"code":                 cc.Code,
		"startsAt":             time.Now().UTC().Format(time.RFC3339),
		"customerSelection":    map[string]any{"all": true},
		"customerGets":         map[string]any{"value": value, "items": map[string]any{"all": true}},
		"appliesOncePerCustomer": cc.Once,
	}
	if cc.Limit > 0 {
		basic["usageLimit"] = cc.Limit
	}
	const mut = `mutation discountCodeBasicCreate($basicCodeDiscount: DiscountCodeBasicInput!) {
		discountCodeBasicCreate(basicCodeDiscount: $basicCodeDiscount) {
			codeDiscountNode { id }
			userErrors { field message }
		}
	}`
	var res struct {
		DiscountCodeBasicCreate struct {
			CodeDiscountNode struct{ ID string }        `json:"codeDiscountNode"`
			UserErrors       []struct{ Message string } `json:"userErrors"`
		} `json:"discountCodeBasicCreate"`
	}
	if err := d.GraphQL(ctx, src, token, mut, map[string]any{"basicCodeDiscount": basic}, &res); err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.DiscountCodeBasicCreate.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.DiscountCodeBasicCreate.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.DiscountCodeBasicCreate.CodeDiscountNode.ID)}, nil
}

func (d *Driver) pushFulfillment(ctx context.Context, src *hub.StoreSource, token string, cf hub.CanonicalFulfillment) (hub.OutboundResult, error) {
	const mut = `mutation fulfillmentCreateV2($fulfillment: FulfillmentV2Input!) {
		fulfillmentCreateV2(fulfillment: $fulfillment) {
			fulfillment { id }
			userErrors { field message }
		}
	}`
	var res struct {
		FulfillmentCreateV2 struct {
			Fulfillment struct{ ID string }        `json:"fulfillment"`
			UserErrors  []struct{ Message string } `json:"userErrors"`
		} `json:"fulfillmentCreateV2"`
	}
	err := d.GraphQL(ctx, src, token, mut, map[string]any{
		"fulfillment": map[string]any{
			"notifyCustomer": cf.NotifyCustomer,
			"trackingInfo": map[string]any{
				"company": cf.Carrier,
				"number":  cf.TrackingNumber,
				"url":     cf.TrackingURL,
			},
			"lineItemsByFulfillmentOrder": []map[string]any{
				{"fulfillmentOrderId": "gid://shopify/FulfillmentOrder/" + stripGID(cf.OrderExternalId)},
			},
		},
	}, &res)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.FulfillmentCreateV2.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.FulfillmentCreateV2.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.FulfillmentCreateV2.Fulfillment.ID)}, nil
}

func (d *Driver) pushRefund(ctx context.Context, src *hub.StoreSource, token string, cr hub.CanonicalRefund) (hub.OutboundResult, error) {
	cur := cr.Currency
	if cur == "" {
		cur = src.Currency
	}
	const mut = `mutation refundCreate($input: RefundInput!) {
		refundCreate(input: $input) {
			refund { id }
			userErrors { field message }
		}
	}`
	var res struct {
		RefundCreate struct {
			Refund     struct{ ID string }        `json:"refund"`
			UserErrors []struct{ Message string } `json:"userErrors"`
		} `json:"refundCreate"`
	}
	err := d.GraphQL(ctx, src, token, mut, map[string]any{
		"input": map[string]any{
			"orderId": "gid://shopify/Order/" + stripGID(cr.OrderExternalId),
			"note":    cr.Note,
			"transactions": []map[string]any{{
				"amount": FormatMoneyDecimal(cr.AmountCents, cur),
				"kind":   "REFUND",
			}},
		},
	}, &res)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.RefundCreate.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.RefundCreate.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.RefundCreate.Refund.ID)}, nil
}

func (d *Driver) pushConsent(ctx context.Context, src *hub.StoreSource, token string, cust hub.CanonicalCustomer) (hub.OutboundResult, error) {
	if cust.ExternalId == "" {
		return hub.OutboundResult{}, nil
	}
	state := "UNSUBSCRIBED"
	if cust.MarketingConsent == "subscribed" {
		state = "SUBSCRIBED"
	}
	const mut = `mutation customerEmailMarketingConsentUpdate($input: CustomerEmailMarketingConsentUpdateInput!) {
		customerEmailMarketingConsentUpdate(input: $input) {
			customer { id }
			userErrors { field message }
		}
	}`
	var res struct {
		CustomerEmailMarketingConsentUpdate struct {
			Customer   struct{ ID string }        `json:"customer"`
			UserErrors []struct{ Message string } `json:"userErrors"`
		} `json:"customerEmailMarketingConsentUpdate"`
	}
	err := d.GraphQL(ctx, src, token, mut, map[string]any{
		"input": map[string]any{
			"customerId": "gid://shopify/Customer/" + stripGID(cust.ExternalId),
			"emailMarketingConsent": map[string]any{
				"marketingState":      state,
				"marketingOptInLevel": "SINGLE_OPT_IN",
			},
		},
	}, &res)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	if len(res.CustomerEmailMarketingConsentUpdate.UserErrors) > 0 {
		return hub.OutboundResult{}, errors.New(res.CustomerEmailMarketingConsentUpdate.UserErrors[0].Message)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.CustomerEmailMarketingConsentUpdate.Customer.ID)}, nil
}

func (d *Driver) pushCampaign(ctx context.Context, src *hub.StoreSource, token string, act hub.CanonicalCampaignActivity) (hub.OutboundResult, error) {
	const mut = `mutation marketingActivityUpsertExternal($input: MarketingActivityUpsertExternalInput!) {
		marketingActivityUpsertExternal(input: $input) {
			marketingActivity { id }
			userErrors { field message }
		}
	}`
	var res struct {
		MarketingActivityUpsertExternal struct {
			MarketingActivity struct{ ID string }        `json:"marketingActivity"`
			UserErrors        []struct{ Message string } `json:"userErrors"`
		} `json:"marketingActivityUpsertExternal"`
	}
	err := d.GraphQL(ctx, src, token, mut, map[string]any{
		"input": map[string]any{
			"remoteId":        act.CampaignId,
			"title":           act.Title,
			"remoteUrl":       act.URL,
			"tactic":          "NEWSLETTER",
			"marketingChannelType": "EMAIL",
		},
	}, &res)
	if err != nil {
		return hub.OutboundResult{}, err
	}
	if act.CustomerId != "" {
		const flowMut = `mutation flowTriggerReceive($handle: String!, $payload: JSON!) {
			flowTriggerReceive(handle: $handle, payload: $payload) {
				userErrors { field message }
			}
		}`
		_ = d.GraphQL(ctx, src, token, flowMut, map[string]any{
			"handle": "hanzo-campaign-sent",
			"payload": map[string]any{
				"customer_id": stripGID(act.CustomerId),
				"Campaign":    act.Title,
			},
		}, nil)
	}
	return hub.OutboundResult{ExternalId: stripGID(res.MarketingActivityUpsertExternal.MarketingActivity.ID)}, nil
}
