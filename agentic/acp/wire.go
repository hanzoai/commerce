// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import "encoding/json"

// Wire types of the Agentic Checkout API 2026-04-17 (spec/2026-04-17/json-schema/
// schema.agentic_checkout.json). Optional fields whose zero value is meaningful are pointers or
// omitzero slices, so a document round-trips.

// Version is the API version this package serves (the API-Version header).
const Version = "2026-04-17"

// Address is a physical address (schema Address).
type Address struct {
	Name       string  `json:"name"`
	LineOne    string  `json:"line_one"`
	LineTwo    *string `json:"line_two,omitempty"`
	City       string  `json:"city"`
	State      string  `json:"state"`
	Country    string  `json:"country"`
	PostalCode string  `json:"postal_code"`
	Company    *string `json:"company,omitempty"`
}

// FulfillmentDetails is the contact and address a fulfillment goes to.
type FulfillmentDetails struct {
	Name        string   `json:"name,omitempty"`
	PhoneNumber string   `json:"phone_number,omitempty"`
	Email       string   `json:"email,omitempty"`
	Address     *Address `json:"address,omitempty"`
}

// CompanyInfo is a business buyer's company.
type CompanyInfo struct {
	Name       string `json:"name"`
	TaxID      string `json:"tax_id,omitempty"`
	Department string `json:"department,omitempty"`
	CostCenter string `json:"cost_center,omitempty"`
}

// LoyaltyInfo is a buyer's loyalty membership.
type LoyaltyInfo struct {
	Tier          string `json:"tier,omitempty"`
	PointsBalance *int64 `json:"points_balance,omitempty"`
	MemberSince   string `json:"member_since,omitempty"`
}

// TaxExemption is a buyer's tax exemption certificate.
type TaxExemption struct {
	CertificateID   string   `json:"certificate_id"`
	CertificateType string   `json:"certificate_type"`
	ExemptRegions   []string `json:"exempt_regions,omitzero"`
	ExpiresAt       string   `json:"expires_at,omitempty"`
}

// Buyer is the person the checkout is for.
type Buyer struct {
	FirstName            string        `json:"first_name,omitempty"`
	LastName             string        `json:"last_name,omitempty"`
	FullName             string        `json:"full_name,omitempty"`
	Email                string        `json:"email"`
	PhoneNumber          string        `json:"phone_number,omitempty"`
	CustomerID           string        `json:"customer_id,omitempty"`
	AccountType          string        `json:"account_type,omitempty"`
	AuthenticationStatus string        `json:"authentication_status,omitempty"`
	Company              *CompanyInfo  `json:"company,omitempty"`
	Loyalty              *LoyaltyInfo  `json:"loyalty,omitempty"`
	TaxExemption         *TaxExemption `json:"tax_exemption,omitempty"`
}

// Interventions are the buyer interventions an agent supports (request) or the seller and
// agent both support (response): capability negotiation.
type Interventions struct {
	Supported           []string `json:"supported,omitzero"`
	Required            []string `json:"required,omitzero"`
	Enforcement         string   `json:"enforcement,omitempty"`
	DisplayContext      string   `json:"display_context,omitempty"`
	RedirectContext     string   `json:"redirect_context,omitempty"`
	MaxRedirects        *int     `json:"max_redirects,omitempty"`
	MaxInteractionDepth *int     `json:"max_interaction_depth,omitempty"`
}

// PaymentHandler is a payment method the seller accepts for a session (schema PaymentHandler).
type PaymentHandler struct {
	ID                      string         `json:"id"`
	Name                    string         `json:"name"`
	DisplayName             string         `json:"display_name,omitempty"`
	Version                 string         `json:"version"`
	Spec                    string         `json:"spec"`
	RequiresDelegatePayment bool           `json:"requires_delegate_payment"`
	RequiresPCICompliance   bool           `json:"requires_pci_compliance"`
	PSP                     string         `json:"psp"`
	ConfigSchema            string         `json:"config_schema"`
	InstrumentSchemas       []string       `json:"instrument_schemas"`
	Config                  map[string]any `json:"config"`
	DisplayOrder            *int           `json:"display_order,omitempty"`
}

// Payment lists the payment handlers of a session.
type Payment struct {
	Handlers []PaymentHandler `json:"handlers"`
}

// Capabilities is the negotiated feature set. The agent sends it on create; the seller
// answers with the intersection. Extensions is either an array of names (agent) or of
// declarations (seller), carried as the JSON it arrived as.
type Capabilities struct {
	Payment       *Payment        `json:"payment,omitempty"`
	Interventions *Interventions  `json:"interventions,omitempty"`
	Extensions    json.RawMessage `json:"extensions,omitempty"`
}

// Item is an item an agent asks to buy. Quantity is not part of the 2026-04-17 request item.
type Item struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	UnitAmount *int64 `json:"unit_amount,omitempty"`
}

// Disclosure is a legal disclosure shown with a line item.
type Disclosure struct {
	Type        string `json:"type"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

// CustomAttribute is a display attribute of a line item.
type CustomAttribute struct {
	DisplayName string `json:"display_name"`
	Value       string `json:"value"`
}

// MarketplaceSellerDetails names the seller of a marketplace item.
type MarketplaceSellerDetails struct {
	Name string `json:"name"`
}

// WeightInfo is an item's weight.
type WeightInfo struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// DimensionsInfo is an item's size.
type DimensionsInfo struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Unit   string  `json:"unit"`
}

// VariantOption is a selected variant (size, color).
type VariantOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// DiscountDetail is a discount applied to one line item.
type DiscountDetail struct {
	Code   string `json:"code,omitempty"`
	Type   string `json:"type"`
	Amount int64  `json:"amount"`
	Source string `json:"source,omitempty"`
}

// TaxBreakdownItem is one jurisdiction's share of a tax total.
type TaxBreakdownItem struct {
	Jurisdiction string  `json:"jurisdiction"`
	Rate         float64 `json:"rate"`
	Amount       int64   `json:"amount"`
}

// Total is one amount of a breakdown, in minor currency units.
type Total struct {
	Type              string             `json:"type"`
	DisplayText       string             `json:"display_text"`
	Amount            int64              `json:"amount"`
	PresentmentAmount *int64             `json:"presentment_amount,omitempty"`
	Description       string             `json:"description,omitempty"`
	Breakdown         []TaxBreakdownItem `json:"breakdown,omitzero"`
}

// LineItem is a line of a session.
type LineItem struct {
	ID                       string                    `json:"id"`
	Item                     Item                      `json:"item"`
	Quantity                 int                       `json:"quantity"`
	Name                     string                    `json:"name,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Images                   []string                  `json:"images,omitzero"`
	UnitAmount               *int64                    `json:"unit_amount,omitempty"`
	Disclosures              []Disclosure              `json:"disclosures,omitzero"`
	CustomAttributes         []CustomAttribute         `json:"custom_attributes,omitzero"`
	MarketplaceSellerDetails *MarketplaceSellerDetails `json:"marketplace_seller_details,omitempty"`
	ProductID                string                    `json:"product_id,omitempty"`
	SKU                      string                    `json:"sku,omitempty"`
	VariantID                string                    `json:"variant_id,omitempty"`
	Category                 string                    `json:"category,omitempty"`
	Tags                     []string                  `json:"tags,omitzero"`
	Weight                   *WeightInfo               `json:"weight,omitempty"`
	Dimensions               *DimensionsInfo           `json:"dimensions,omitempty"`
	AvailabilityStatus       string                    `json:"availability_status,omitempty"`
	AvailableQuantity        *int                      `json:"available_quantity,omitempty"`
	MaxQuantityPerOrder      *int                      `json:"max_quantity_per_order,omitempty"`
	FulfillableOn            string                    `json:"fulfillable_on,omitempty"`
	VariantOptions           []VariantOption           `json:"variant_options,omitzero"`
	DiscountDetails          []DiscountDetail          `json:"discount_details,omitzero"`
	TaxExempt                *bool                     `json:"tax_exempt,omitempty"`
	TaxExemptionReason       string                    `json:"tax_exemption_reason,omitempty"`
	ParentID                 string                    `json:"parent_id,omitempty"`
	Totals                   []Total                   `json:"totals"`
}

// PickupLocation is where a pickup option is collected.
type PickupLocation struct {
	Name         string  `json:"name"`
	Address      Address `json:"address"`
	Phone        string  `json:"phone,omitempty"`
	Instructions string  `json:"instructions,omitempty"`
}

// DeliveryWindow is the span a local delivery arrives in.
type DeliveryWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ServiceArea is where a local delivery goes.
type ServiceArea struct {
	RadiusMiles      *float64 `json:"radius_miles,omitempty"`
	CenterPostalCode string   `json:"center_postal_code,omitempty"`
}

// FulfillmentOption is a way the items can be fulfilled: schema FulfillmentOptionShipping,
// Digital, Pickup or LocalDelivery, told apart by Type.
type FulfillmentOption struct {
	Type                 string          `json:"type"`
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Description          string          `json:"description,omitempty"`
	Carrier              string          `json:"carrier,omitempty"`
	EarliestDeliveryTime string          `json:"earliest_delivery_time,omitempty"`
	LatestDeliveryTime   string          `json:"latest_delivery_time,omitempty"`
	Location             *PickupLocation `json:"location,omitempty"`
	PickupType           string          `json:"pickup_type,omitempty"`
	ReadyBy              string          `json:"ready_by,omitempty"`
	PickupBy             string          `json:"pickup_by,omitempty"`
	DeliveryWindow       *DeliveryWindow `json:"delivery_window,omitempty"`
	ServiceArea          *ServiceArea    `json:"service_area,omitempty"`
	Totals               []Total         `json:"totals"`
}

// SelectedFulfillmentOption is the option chosen for some items.
type SelectedFulfillmentOption struct {
	Type     string   `json:"type"`
	OptionID string   `json:"option_id"`
	ItemIDs  []string `json:"item_ids"`
}

// FulfillmentGroup groups items by how they are fulfilled.
type FulfillmentGroup struct {
	ID                 string              `json:"id"`
	ItemIDs            []string            `json:"item_ids"`
	DestinationType    string              `json:"destination_type"`
	FulfillmentDetails *FulfillmentDetails `json:"fulfillment_details,omitempty"`
	LocationID         string              `json:"location_id,omitempty"`
	Instructions       string              `json:"instructions,omitempty"`
}

// Message is a notice in a session: schema MessageInfo (Type info), MessageWarning (warning)
// or MessageError (error). Info carries no Code.
type Message struct {
	Type        string `json:"type"`
	Code        string `json:"code,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	Param       string `json:"param,omitempty"`
	ContentType string `json:"content_type"`
	Content     string `json:"content"`
}

// Link is a policy or support URL of the seller.
type Link struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

// AcquirerDetails are the acquirer's 3-D Secure identifiers.
type AcquirerDetails struct {
	AcquirerBIN        string `json:"acquirer_bin"`
	AcquirerCountry    string `json:"acquirer_country"`
	AcquirerMerchantID string `json:"acquirer_merchant_id"`
	MerchantName       string `json:"merchant_name"`
	RequestorID        string `json:"requestor_id,omitempty"`
}

// FlowPreference is the 3-D Secure flow the seller prefers.
type FlowPreference struct {
	Type         string            `json:"type"`
	Challenge    *FlowChallenge    `json:"challenge,omitempty"`
	Frictionless *FlowFrictionless `json:"frictionless,omitempty"`
}

// FlowChallenge qualifies a challenge flow.
type FlowChallenge struct {
	Type string `json:"type,omitempty"`
}

// FlowFrictionless qualifies a frictionless flow.
type FlowFrictionless struct {
	Type string `json:"type,omitempty"`
}

// AuthenticationMetadata is what an agent needs to run 3-D Secure for a session.
type AuthenticationMetadata struct {
	AcquirerDetails AcquirerDetails `json:"acquirer_details"`
	DirectoryServer string          `json:"directory_server"`
	FlowPreference  *FlowPreference `json:"flow_preference,omitempty"`
}

// OutcomeDetails are the cryptographic results of a 3-D Secure authentication.
type OutcomeDetails struct {
	ThreeDSCryptogram           string `json:"three_ds_cryptogram,omitempty"`
	ElectronicCommerceIndicator string `json:"electronic_commerce_indicator,omitempty"`
	TransactionID               string `json:"transaction_id"`
	Version                     string `json:"version"`
}

// AuthenticationResult is the outcome of the 3-D Secure flow an agent ran.
type AuthenticationResult struct {
	Outcome        string          `json:"outcome"`
	OutcomeDetails *OutcomeDetails `json:"outcome_details,omitempty"`
}

// MarketingConsentOption is a marketing channel the seller offers consent for.
type MarketingConsentOption struct {
	Channel          string `json:"channel"`
	DisplayText      string `json:"display_text"`
	PrivacyPolicyURL string `json:"privacy_policy_url"`
	IsSubscribed     *bool  `json:"is_subscribed,omitempty"`
}

// MarketingConsent is a buyer's decision on one channel.
type MarketingConsent struct {
	Channel string `json:"channel"`
	OptedIn bool   `json:"opted_in"`
}

// EstimatedDelivery is a delivery time range.
type EstimatedDelivery struct {
	Earliest string `json:"earliest"`
	Latest   string `json:"latest"`
}

// OrderConfirmation is the confirmation of an order.
type OrderConfirmation struct {
	ConfirmationNumber    string `json:"confirmation_number,omitempty"`
	ConfirmationEmailSent *bool  `json:"confirmation_email_sent,omitempty"`
	ReceiptURL            string `json:"receipt_url,omitempty"`
	InvoiceNumber         string `json:"invoice_number,omitempty"`
	OrderNotes            string `json:"order_notes,omitempty"`
}

// SupportInfo is how to reach the seller about an order.
type SupportInfo struct {
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Hours         string `json:"hours,omitempty"`
	HelpCenterURL string `json:"help_center_url,omitempty"`
}

// OrderLineItemQuantity tracks ordered, current and fulfilled quantity.
type OrderLineItemQuantity struct {
	Ordered   int  `json:"ordered"`
	Current   int  `json:"current"`
	Fulfilled *int `json:"fulfilled,omitempty"`
}

// OrderLineItem is a line of an order.
type OrderLineItem struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	ProductID   string                `json:"product_id,omitempty"`
	Description string                `json:"description,omitempty"`
	ImageURL    string                `json:"image_url,omitempty"`
	URL         string                `json:"url,omitempty"`
	Quantity    OrderLineItemQuantity `json:"quantity"`
	UnitPrice   *int64                `json:"unit_price,omitempty"`
	Subtotal    *int64                `json:"subtotal,omitempty"`
	Totals      []Total               `json:"totals,omitzero"`
	Status      string                `json:"status,omitempty"`
}

// LineItemReference names a line item and quantity in a fulfillment or adjustment.
type LineItemReference struct {
	ID       string `json:"id"`
	Quantity int    `json:"quantity"`
}

// DigitalDelivery is how digital content is reached.
type DigitalDelivery struct {
	AccessURL  string `json:"access_url,omitempty"`
	LicenseKey string `json:"license_key,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// FulfillmentEvent is a point-in-time event of a fulfillment.
type FulfillmentEvent struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	OccurredAt  string `json:"occurred_at"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
}

// Fulfillment is how items of an order are delivered.
type Fulfillment struct {
	ID                string              `json:"id"`
	Type              string              `json:"type"`
	Status            string              `json:"status,omitempty"`
	LineItems         []LineItemReference `json:"line_items,omitzero"`
	Carrier           string              `json:"carrier,omitempty"`
	TrackingNumber    string              `json:"tracking_number,omitempty"`
	TrackingURL       string              `json:"tracking_url,omitempty"`
	Destination       *Address            `json:"destination,omitempty"`
	EstimatedDelivery *EstimatedDelivery  `json:"estimated_delivery,omitempty"`
	DigitalDelivery   *DigitalDelivery    `json:"digital_delivery,omitempty"`
	Description       string              `json:"description,omitempty"`
	Events            []FulfillmentEvent  `json:"events,omitzero"`
}

// Adjustment is a change after the order: refund, credit, return, dispute.
type Adjustment struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	OccurredAt  string              `json:"occurred_at"`
	Status      string              `json:"status"`
	LineItems   []LineItemReference `json:"line_items,omitzero"`
	Amount      *int64              `json:"amount,omitempty"`
	Currency    string              `json:"currency,omitempty"`
	Description string              `json:"description,omitempty"`
	Reason      string              `json:"reason,omitempty"`
}

// Order is what a completed checkout created (schema Order). Type is "order" in webhook
// payloads and absent in a checkout response.
type Order struct {
	Type              string             `json:"type,omitempty"`
	ID                string             `json:"id"`
	CheckoutSessionID string             `json:"checkout_session_id"`
	OrderNumber       string             `json:"order_number,omitempty"`
	ClientReferenceID string             `json:"client_reference_id,omitempty"`
	PermalinkURL      string             `json:"permalink_url"`
	Status            string             `json:"status,omitempty"`
	EstimatedDelivery *EstimatedDelivery `json:"estimated_delivery,omitempty"`
	Confirmation      *OrderConfirmation `json:"confirmation,omitempty"`
	Support           *SupportInfo       `json:"support,omitempty"`
	LineItems         []OrderLineItem    `json:"line_items,omitzero"`
	Fulfillments      []Fulfillment      `json:"fulfillments,omitzero"`
	Adjustments       []Adjustment       `json:"adjustments,omitzero"`
	Totals            []Total            `json:"totals,omitzero"`
}

// ProtocolVersion names the protocol version of a session.
type ProtocolVersion struct {
	Version string `json:"version"`
}

// CheckoutSession is a checkout as the seller states it (schema CheckoutSession; after
// completion CheckoutSessionWithOrder, which adds Order). Every amount is in minor units.
type CheckoutSession struct {
	ID                         string                      `json:"id"`
	Protocol                   *ProtocolVersion            `json:"protocol,omitempty"`
	Capabilities               Capabilities                `json:"capabilities"`
	Buyer                      *Buyer                      `json:"buyer,omitempty"`
	Status                     string                      `json:"status"`
	Currency                   string                      `json:"currency"`
	PresentmentCurrency        string                      `json:"presentment_currency,omitempty"`
	ExchangeRate               *float64                    `json:"exchange_rate,omitempty"`
	ExchangeRateTimestamp      string                      `json:"exchange_rate_timestamp,omitempty"`
	Locale                     string                      `json:"locale,omitempty"`
	Timezone                   string                      `json:"timezone,omitempty"`
	LineItems                  []LineItem                  `json:"line_items"`
	FulfillmentDetails         *FulfillmentDetails         `json:"fulfillment_details,omitempty"`
	FulfillmentOptions         []FulfillmentOption         `json:"fulfillment_options"`
	SelectedFulfillmentOptions []SelectedFulfillmentOption `json:"selected_fulfillment_options,omitzero"`
	FulfillmentGroups          []FulfillmentGroup          `json:"fulfillment_groups,omitzero"`
	Totals                     []Total                     `json:"totals"`
	Messages                   []Message                   `json:"messages"`
	Links                      []Link                      `json:"links"`
	AuthenticationMetadata     *AuthenticationMetadata     `json:"authentication_metadata,omitempty"`
	CreatedAt                  string                      `json:"created_at,omitempty"`
	UpdatedAt                  string                      `json:"updated_at,omitempty"`
	ExpiresAt                  string                      `json:"expires_at,omitempty"`
	ContinueURL                string                      `json:"continue_url,omitempty"`
	Metadata                   map[string]any              `json:"metadata,omitempty"`
	QuoteID                    string                      `json:"quote_id,omitempty"`
	QuoteExpiresAt             string                      `json:"quote_expires_at,omitempty"`
	MarketingConsentOptions    []MarketingConsentOption    `json:"marketing_consent_options,omitzero"`
	Order                      *Order                      `json:"order,omitempty"`
}

// Session statuses (schema CheckoutSessionBase.status).
const (
	StatusReadyForPayment    = "ready_for_payment"
	StatusCompleteInProgress = "complete_in_progress"
	StatusCompleted          = "completed"
	StatusCanceled           = "canceled"
)

// DiscountsRequest carries the discount codes an agent submits (the discount extension).
type DiscountsRequest struct {
	Codes []string `json:"codes,omitzero"`
}

// AffiliateAttribution credits a third-party publisher. Servers ignore fields they do not
// know (rfc.affiliate_attribution §8.2), so it decodes without rejecting unknown fields.
type AffiliateAttribution struct {
	Provider    string                      `json:"provider"`
	Token       string                      `json:"token,omitempty"`
	PublisherID string                      `json:"publisher_id,omitempty"`
	CampaignID  string                      `json:"campaign_id,omitempty"`
	CreativeID  string                      `json:"creative_id,omitempty"`
	SubID       string                      `json:"sub_id,omitempty"`
	Source      *AffiliateAttributionSource `json:"source,omitempty"`
	IssuedAt    string                      `json:"issued_at,omitempty"`
	ExpiresAt   string                      `json:"expires_at,omitempty"`
	Metadata    map[string]any              `json:"metadata,omitempty"`
	Touchpoint  string                      `json:"touchpoint,omitempty"`
}

// AffiliateAttributionSource is where an attribution originated.
type AffiliateAttributionSource struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// UnmarshalJSON decodes an attribution, ignoring unknown fields.
func (a *AffiliateAttribution) UnmarshalJSON(b []byte) error {
	type plain AffiliateAttribution
	return json.Unmarshal(b, (*plain)(a))
}

// Credential is the secret a payment instrument carries.
type Credential struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Instrument is the payment instrument an agent submits. The schema does not close it: the
// base fields of rfc.payment_handlers §5.2 may accompany Type and Credential, and any other
// property is ignored.
type Instrument struct {
	ID         string     `json:"id,omitempty"`
	HandlerID  string     `json:"handler_id,omitempty"`
	Type       string     `json:"type"`
	Credential Credential `json:"credential"`
}

// UnmarshalJSON decodes an instrument, ignoring unknown fields.
func (i *Instrument) UnmarshalJSON(b []byte) error {
	type plain Instrument
	return json.Unmarshal(b, (*plain)(i))
}

// PaymentData is how the agent pays (schema PaymentData).
type PaymentData struct {
	HandlerID           string      `json:"handler_id,omitempty"`
	Instrument          *Instrument `json:"instrument,omitempty"`
	BillingAddress      *Address    `json:"billing_address,omitempty"`
	PurchaseOrderNumber string      `json:"purchase_order_number,omitempty"`
	PaymentTerms        string      `json:"payment_terms,omitempty"`
	DueDate             string      `json:"due_date,omitempty"`
	ApprovalRequired    *bool       `json:"approval_required,omitempty"`
}

// RiskSignals are fraud signals an agent passes on completion.
type RiskSignals struct {
	IPAddress         string `json:"ip_address,omitempty"`
	UserAgent         string `json:"user_agent,omitempty"`
	AcceptLanguage    string `json:"accept_language,omitempty"`
	SessionID         string `json:"session_id,omitempty"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
}

// IntentTrace is why a buyer abandoned a session.
type IntentTrace struct {
	ReasonCode   string         `json:"reason_code"`
	TraceSummary string         `json:"trace_summary,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// CreateRequest opens a session (schema CheckoutSessionCreateRequest).
type CreateRequest struct {
	Buyer                *Buyer                `json:"buyer,omitempty"`
	LineItems            []Item                `json:"line_items,omitzero"`
	Currency             string                `json:"currency,omitempty"`
	FulfillmentDetails   *FulfillmentDetails   `json:"fulfillment_details,omitempty"`
	Capabilities         *Capabilities         `json:"capabilities,omitempty"`
	FulfillmentGroups    []FulfillmentGroup    `json:"fulfillment_groups,omitzero"`
	AffiliateAttribution *AffiliateAttribution `json:"affiliate_attribution,omitempty"`
	Coupons              []string              `json:"coupons,omitzero"`
	Discounts            *DiscountsRequest     `json:"discounts,omitempty"`
	Locale               string                `json:"locale,omitempty"`
	Timezone             string                `json:"timezone,omitempty"`
	QuoteID              string                `json:"quote_id,omitempty"`
	Metadata             map[string]any        `json:"metadata,omitempty"`
	OrderNotes           *string               `json:"order_notes,omitempty"`
}

// UpdateRequest changes a session (schema CheckoutSessionUpdateRequest).
type UpdateRequest struct {
	Buyer                      *Buyer                      `json:"buyer,omitempty"`
	LineItems                  []Item                      `json:"line_items,omitzero"`
	FulfillmentDetails         *FulfillmentDetails         `json:"fulfillment_details,omitempty"`
	FulfillmentGroups          []FulfillmentGroup          `json:"fulfillment_groups,omitzero"`
	SelectedFulfillmentOptions []SelectedFulfillmentOption `json:"selected_fulfillment_options,omitzero"`
	Coupons                    []string                    `json:"coupons,omitzero"`
	Discounts                  *DiscountsRequest           `json:"discounts,omitempty"`
	OrderNotes                 *string                     `json:"order_notes,omitempty"`
}

// CompleteRequest finishes a session (schema CheckoutSessionCompleteRequest).
type CompleteRequest struct {
	Buyer                *Buyer                `json:"buyer,omitempty"`
	PaymentData          *PaymentData          `json:"payment_data,omitempty"`
	AuthenticationResult *AuthenticationResult `json:"authentication_result,omitempty"`
	AffiliateAttribution *AffiliateAttribution `json:"affiliate_attribution,omitempty"`
	RiskSignals          *RiskSignals          `json:"risk_signals,omitempty"`
	MarketingConsents    []MarketingConsent    `json:"marketing_consents,omitzero"`
	OrderNotes           *string               `json:"order_notes,omitempty"`
}

// CancelRequest abandons a session (schema CancelSessionRequest).
type CancelRequest struct {
	IntentTrace *IntentTrace `json:"intent_trace,omitempty"`
}

// Error is the protocol error returned in a 4xx or 5xx response (schema Error): Type is
// invalid_request, processing_error or service_unavailable, Param an RFC 9535 JSONPath.
type Error struct {
	Type              string   `json:"type"`
	Code              string   `json:"code"`
	Message           string   `json:"message"`
	Param             string   `json:"param,omitempty"`
	SupportedVersions []string `json:"supported_versions,omitempty"`
}

// Error types (schema Error.type).
const (
	TypeInvalidRequest     = "invalid_request"
	TypeProcessingError    = "processing_error"
	TypeServiceUnavailable = "service_unavailable"
)

// DiscoveryProtocol identifies the protocol and its versions.
type DiscoveryProtocol struct {
	Name              string   `json:"name"`
	Version           string   `json:"version"`
	SupportedVersions []string `json:"supported_versions"`
	DocumentationURL  string   `json:"documentation_url,omitempty"`
}

// DiscoveryExtension declares an extension the seller supports.
type DiscoveryExtension struct {
	Name   string `json:"name"`
	Spec   string `json:"spec,omitempty"`
	Schema string `json:"schema,omitempty"`
}

// DiscoveryCapabilities are the seller's stable capabilities.
type DiscoveryCapabilities struct {
	Services            []string             `json:"services"`
	Extensions          []DiscoveryExtension `json:"extensions,omitzero"`
	InterventionTypes   []string             `json:"intervention_types,omitzero"`
	SupportedCurrencies []string             `json:"supported_currencies,omitzero"`
	SupportedLocales    []string             `json:"supported_locales,omitzero"`
}

// Discovery is the document served at /.well-known/acp.json (schema DiscoveryResponse).
type Discovery struct {
	Protocol     DiscoveryProtocol     `json:"protocol"`
	APIBaseURL   string                `json:"api_base_url"`
	Transports   []string              `json:"transports"`
	Capabilities DiscoveryCapabilities `json:"capabilities"`
}

// Event is an order lifecycle webhook payload (openapi.agentic_checkout_webhook.yaml
// WebhookEvent): Type is order_create or order_update; Data is the full order.
type Event struct {
	Type string `json:"type"`
	Data Order  `json:"data"`
}

// Webhook event types.
const (
	EventOrderCreate = "order_create"
	EventOrderUpdate = "order_update"
)
