// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"fmt"
)

// Checkout statuses (schemas/shopping/checkout.json, status; shopping/checkout,
// "Status Values").
const (
	Incomplete         = "incomplete"
	RequiresEscalation = "requires_escalation"
	ReadyForComplete   = "ready_for_complete"
	CompleteInProgress = "complete_in_progress"
	Completed          = "completed"
	Canceled           = "canceled"
)

// Message severities (schemas/common/types/message_error.json).
const (
	Recoverable         = "recoverable"
	RequiresBuyerInput  = "requires_buyer_input"
	RequiresBuyerReview = "requires_buyer_review"
	Unrecoverable       = "unrecoverable"
)

// Checkout is the checkout resource (schemas/shopping/checkout.json). A
// business returns it with the ucp envelope; the members of an extension that
// composes onto checkout (fulfillment, discounts and the like) are kept in
// Extra, as the schema is open.
type Checkout struct {
	UCP         *Meta                      `json:"ucp,omitempty"`
	ID          string                     `json:"id"`
	LineItems   []LineItem                 `json:"line_items"`
	Buyer       *Buyer                     `json:"buyer,omitempty"`
	Context     *Context                   `json:"context,omitempty"`
	Signals     map[string]json.RawMessage `json:"signals,omitempty"`
	Attribution map[string]string          `json:"attribution,omitempty"`
	Status      string                     `json:"status"`
	Currency    string                     `json:"currency"`
	Totals      []Total                    `json:"totals"`
	Actions     map[string][]Action        `json:"actions,omitempty"`
	Messages    []Message                  `json:"messages,omitempty"`
	Links       []Link                     `json:"links"`
	ExpiresAt   string                     `json:"expires_at,omitempty"`
	ContinueURL string                     `json:"continue_url,omitempty"`
	Payment     *Payment                   `json:"payment,omitempty"`
	Order       *OrderRef                  `json:"order,omitempty"`

	Extra map[string]json.RawMessage `json:"-"`
}

func (c *Checkout) UnmarshalJSON(b []byte) error {
	type plain Checkout
	return decodeOpen(b, (*plain)(c), &c.Extra)
}

func (c Checkout) MarshalJSON() ([]byte, error) {
	type plain Checkout
	p := plain(c)
	if p.LineItems == nil {
		p.LineItems = []LineItem{}
	}
	if p.Totals == nil {
		p.Totals = []Total{}
	}
	if p.Links == nil {
		p.Links = []Link{}
	}
	return merge(p, c.Extra)
}

// LineItem is a line of a checkout (schemas/shopping/types/line_item.json).
type LineItem struct {
	ID       string  `json:"id,omitempty"`
	Item     Item    `json:"item"`
	Quantity int64   `json:"quantity"`
	Totals   []Total `json:"totals,omitempty"`
	ParentID string  `json:"parent_id,omitempty"`
}

// Item is what a line sells (schemas/shopping/types/item.json). On a request
// only id and quantity_unit are sent.
type Item struct {
	ID           string        `json:"id"`
	Title        string        `json:"title,omitempty"`
	Price        *int64        `json:"price,omitempty"`
	ListPrice    *int64        `json:"list_price,omitempty"`
	QuantityUnit *QuantityUnit `json:"quantity_unit,omitempty"`
	UnitPrice    *UnitPrice    `json:"unit_price,omitempty"`
	ImageURL     string        `json:"image_url,omitempty"`
}

// Descriptor is the shared unit descriptor (schemas/common/types/unit.json).
type Descriptor struct {
	Unit        string `json:"unit"`
	Scale       *int   `json:"scale,omitempty"`
	DisplayText string `json:"display_text"`
}

// QuantityUnit is the sale basis a quantity is denominated in
// (schemas/common/types/quantity_unit.json).
type QuantityUnit struct {
	Descriptor
	Increment *int `json:"increment,omitempty"`
}

// Measure is a value in a unit (schemas/common/types/measure.json).
type Measure struct {
	Descriptor
	Value int64 `json:"value"`
}

// UnitPrice is the price per standard unit of measurement
// (schemas/shopping/types/unit_price.json).
type UnitPrice struct {
	Amount    int64   `json:"amount"`
	Currency  string  `json:"currency"`
	Measure   Measure `json:"measure"`
	Reference Measure `json:"reference"`
}

// Total is one entry of a cost breakdown (schemas/common/types/total.json and
// totals.json). Amounts are ISO 4217 minor units; the sign is the direction.
type Total struct {
	Type        string      `json:"type"`
	DisplayText string      `json:"display_text,omitempty"`
	Amount      int64       `json:"amount"`
	Lines       []TotalLine `json:"lines,omitempty"`
}

// TotalLine is a sub-line of a total.
type TotalLine struct {
	DisplayText string `json:"display_text"`
	Amount      int64  `json:"amount"`
}

// Link is a legal or informational link the platform displays
// (schemas/common/types/link.json).
type Link struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	ID    string `json:"id,omitempty"`
}

// Message is an error, warning or info message (schemas/common/types/
// message.json): a oneOf over message_error, message_warning and message_info,
// told apart by Type.
type Message struct {
	Type         string `json:"type"`
	Code         string `json:"code,omitempty"`
	Path         string `json:"path,omitempty"`
	Content      string `json:"content"`
	ContentType  string `json:"content_type,omitempty"`
	Severity     string `json:"severity,omitempty"`
	Presentation string `json:"presentation,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	URL          string `json:"url,omitempty"`
}

// Err builds an error message.
func Err(code, severity, path, format string, args ...any) Message {
	return Message{Type: "error", Code: code, Severity: severity, Path: path, Content: fmt.Sprintf(format, args...)}
}

// Check validates a message against its variant of the schema: an error has
// type, code, content and a severity of the enum; a warning has type, code and
// content; an info has type and content.
func (m Message) Check() error {
	switch m.Type {
	case "error":
		switch m.Severity {
		case Recoverable, RequiresBuyerInput, RequiresBuyerReview, Unrecoverable:
		default:
			return fmt.Errorf("error message severity %q is not one of the enum", m.Severity)
		}
		if m.Code == "" {
			return fmt.Errorf("error message needs a code")
		}
	case "warning":
		if m.Code == "" {
			return fmt.Errorf("warning message needs a code")
		}
	case "info":
	default:
		return fmt.Errorf("message type %q is not error, warning or info", m.Type)
	}
	if m.Content == "" {
		return fmt.Errorf("message needs content")
	}
	return nil
}

// Action is one outstanding extension-defined Action instance
// (schemas/common/types/actions.json); this package defines none.
type Action struct {
	ID     string          `json:"id"`
	Config json.RawMessage `json:"config,omitempty"`
}

// OrderRef is the order created for a checkout (schemas/shopping/types/order_confirmation.json).
type OrderRef struct {
	ID           string `json:"id"`
	Label        string `json:"label,omitempty"`
	PermalinkURL string `json:"permalink_url"`
}

// Buyer represents the buyer (schemas/shopping/types/buyer.json); the schema is open.
type Buyer struct {
	FirstName   string                     `json:"first_name,omitempty"`
	LastName    string                     `json:"last_name,omitempty"`
	Email       string                     `json:"email,omitempty"`
	PhoneNumber string                     `json:"phone_number,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

func (b *Buyer) UnmarshalJSON(data []byte) error {
	type plain Buyer
	return decodeOpen(data, (*plain)(b), &b.Extra)
}

func (b Buyer) MarshalJSON() ([]byte, error) {
	type plain Buyer
	return merge(plain(b), b.Extra)
}

// Context is provisional buyer context (schemas/common/types/context.json); open.
type Context struct {
	AddressCountry string                     `json:"address_country,omitempty"`
	AddressRegion  string                     `json:"address_region,omitempty"`
	PostalCode     string                     `json:"postal_code,omitempty"`
	Location       string                     `json:"location,omitempty"`
	Intent         string                     `json:"intent,omitempty"`
	Language       string                     `json:"language,omitempty"`
	Currency       string                     `json:"currency,omitempty"`
	Eligibility    []string                   `json:"eligibility,omitempty"`
	Payment        []ContextPayment           `json:"payment,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

// ContextPayment is a buyer-preferred payment handler.
type ContextPayment struct {
	Handler string   `json:"handler"`
	Types   []string `json:"types,omitempty"`
}

func (c *Context) UnmarshalJSON(data []byte) error {
	type plain Context
	return decodeOpen(data, (*plain)(c), &c.Extra)
}

func (c Context) MarshalJSON() ([]byte, error) {
	type plain Context
	return merge(plain(c), c.Extra)
}

// Address is a postal address (schemas/common/types/postal_address.json).
type Address struct {
	ExtendedAddress string `json:"extended_address,omitempty"`
	StreetAddress   string `json:"street_address,omitempty"`
	AddressLocality string `json:"address_locality,omitempty"`
	AddressRegion   string `json:"address_region,omitempty"`
	AddressCountry  string `json:"address_country,omitempty"`
	PostalCode      string `json:"postal_code,omitempty"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	PhoneNumber     string `json:"phone_number,omitempty"`
}

// Payment holds the payment instruments of a checkout (schemas/common/types/payment.json).
type Payment struct {
	Instruments []Instrument `json:"instruments,omitempty"`
}

// Instrument is a payment instrument linked to a handler by handler_id
// (schemas/common/types/payment_instrument.json, with selected). Handlers add
// members of their own, which the open schema keeps in Extra.
type Instrument struct {
	ID             string                     `json:"id"`
	HandlerID      string                     `json:"handler_id"`
	Type           string                     `json:"type"`
	Selected       *bool                      `json:"selected,omitempty"`
	BillingAddress *Address                   `json:"billing_address,omitempty"`
	Credential     *Credential                `json:"credential,omitempty"`
	Display        json.RawMessage            `json:"display,omitempty"`
	Extra          map[string]json.RawMessage `json:"-"`
}

func (i *Instrument) UnmarshalJSON(data []byte) error {
	type plain Instrument
	return decodeOpen(data, (*plain)(i), &i.Extra)
}

func (i Instrument) MarshalJSON() ([]byte, error) {
	type plain Instrument
	return merge(plain(i), i.Extra)
}

// Credential is a payment credential (schemas/common/types/payment_credential.json
// and token_credential.json): a type discriminator and, for a token credential,
// the token. Handlers define further members, kept in Extra.
type Credential struct {
	Type  string                     `json:"type"`
	Token string                     `json:"token,omitempty"`
	Extra map[string]json.RawMessage `json:"-"`
}

func (c *Credential) UnmarshalJSON(data []byte) error {
	type plain Credential
	return decodeOpen(data, (*plain)(c), &c.Extra)
}

func (c Credential) MarshalJSON() ([]byte, error) {
	type plain Credential
	return merge(plain(c), c.Extra)
}

// ErrorResponse is the response when no resource can be established or
// retrieved (schemas/common/types/error_response.json): ucp with status
// "error", at least one message, and an optional continue_url. The schema is
// closed.
type ErrorResponse struct {
	UCP         Meta      `json:"ucp"`
	Messages    []Message `json:"messages"`
	ContinueURL string    `json:"continue_url,omitempty"`
}

// CreateRequest is a Create Checkout request (shopping/checkout, "Create
// Checkout"): line_items or cart_id, at least one of them.
type CreateRequest struct {
	LineItems   []LineItem                 `json:"line_items,omitempty"`
	CartID      string                     `json:"cart_id,omitempty"`
	Buyer       *Buyer                     `json:"buyer,omitempty"`
	Context     *Context                   `json:"context,omitempty"`
	Signals     map[string]json.RawMessage `json:"signals,omitempty"`
	Attribution map[string]string          `json:"attribution,omitempty"`
	Payment     *Payment                   `json:"payment,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

// UpdateRequest is an Update Checkout request: the full checkout resource,
// whose line_items are required.
type UpdateRequest struct {
	LineItems   []LineItem                 `json:"line_items"`
	Buyer       *Buyer                     `json:"buyer,omitempty"`
	Context     *Context                   `json:"context,omitempty"`
	Signals     map[string]json.RawMessage `json:"signals,omitempty"`
	Attribution map[string]string          `json:"attribution,omitempty"`
	Payment     *Payment                   `json:"payment,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

// CompleteRequest is a Complete Checkout request: payment is required, and
// complete is a command that changes no purchase term.
type CompleteRequest struct {
	Payment     *Payment                   `json:"payment"`
	Signals     map[string]json.RawMessage `json:"signals,omitempty"`
	Attribution map[string]string          `json:"attribution,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}

func (r *CreateRequest) UnmarshalJSON(b []byte) error {
	type plain CreateRequest
	return decodeOpen(b, (*plain)(r), &r.Extra)
}

func (r CreateRequest) MarshalJSON() ([]byte, error) {
	type plain CreateRequest
	return merge(plain(r), r.Extra)
}

func (r *UpdateRequest) UnmarshalJSON(b []byte) error {
	type plain UpdateRequest
	return decodeOpen(b, (*plain)(r), &r.Extra)
}

func (r UpdateRequest) MarshalJSON() ([]byte, error) {
	type plain UpdateRequest
	return merge(plain(r), r.Extra)
}

func (r *CompleteRequest) UnmarshalJSON(b []byte) error {
	type plain CompleteRequest
	return decodeOpen(b, (*plain)(r), &r.Extra)
}

func (r CompleteRequest) MarshalJSON() ([]byte, error) {
	type plain CompleteRequest
	return merge(plain(r), r.Extra)
}
