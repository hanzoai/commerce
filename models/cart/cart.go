package cart

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
	. "crowdstart.com/models/lineitem"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Status string

const (
	Cancelled Status = "cancelled"
	Completed        = "completed"
	Locked           = "locked"
	OnHold           = "on-hold"
	Open             = "open"
)

type Cart struct {
	mixin.Model

	// Store this was sold from (if any)
	StoreId string `json:"storeId,omitempty"`

	// Associated campaign
	CampaignId string `json:"campaignId,omitempty"`

	// Associated Crowdstart user or buyer.
	UserId string `json:"userId,omitempty"`

	// Whether this was a preorder or not
	Preorder bool `json:"preorder"`

	// Cart is unconfirmed if user has not declared (either implicitly or
	// explicitly) precise order variant options.
	Unconfirmed bool `json:"unconfirmed"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type `json:"currency"`

	// Payment processor type - paypal, stripe, etc
	Type string `json:"type"`

	// Shipping method
	ShippingMethod string `json:"shippingMethod"`

	// Sum of the line item amounts. Amount in cents.
	LineTotal currency.Cents `json:"lineTotal"`

	// Discount amount applied to the order. Amount in cents.
	Discount currency.Cents `json:"discount"`

	// Sum of line totals less discount. Amount in cents.
	Subtotal currency.Cents `json:"subtotal"`

	// Shipping cost applied. Amount in cents.
	Shipping currency.Cents `json:"shipping"`

	// Sales tax applied. Amount in cents.
	Tax currency.Cents `json:"tax"`

	// Price adjustments applied. Amount in cents.
	Adjustment currency.Cents `json:"-"`

	// Total = subtotal + shipping + taxes + adjustments. Amount in cents.
	Total currency.Cents `json:"total"`

	// Amount owed to the seller. Amount in cents.
	Balance currency.Cents `json:"balance"`

	// Gross amount paid to the seller. Amount in cents.
	Paid currency.Cents `json:"paid"`

	// integer	Amount refunded by the seller. Amount in cents.
	Refunded currency.Cents `json:"refunded"`

	BillingAddress  Address `json:"billingAddress"`
	ShippingAddress Address `json:"shippingAddress"`

	// Individual line items
	Items  []LineItem `json:"items" datastore:"-"`
	Items_ string     `json:"-"` // need props

	Adjustments []Adjustment `json:"-"`

	Coupons     []coupon.Coupon `json:"coupons,omitempty"`
	CouponCodes []string        `json:"couponCodes,omitempty"`
	ReferrerId  string          `json:"referrerId,omitempty"`

	PaymentIds []string `json:"payments"`

	// Fulfillment information
	Fulfillment Fulfillment `json:"fulfillment"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	Test bool `json:"-"` // Whether our internal test flag is active or not

	Gift        bool   `json:"gift"`        // Is this a gift?
	GiftMessage string `json:"giftMessage"` // Message to go on gift
	GiftEmail   string `json:"giftEmail"`   // Email for digital gifts
}

func (o *Cart) Validator() *val.Validator {
	return val.New()
}

func (o *Cart) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	o.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(o, c)); err != nil {
		return err
	}

	for _, coup := range o.Coupons {
		coup.Init(o.Model.Db)
	}

	// Deserialize from datastore
	if len(o.Items_) > 0 {
		err = json.DecodeBytes([]byte(o.Items_), &o.Items)
	}

	if len(o.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(o.Metadata_), &o.Metadata)
	}

	return err
}

func (o *Cart) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	o.Metadata_ = string(json.EncodeBytes(&o.Metadata))
	o.Items_ = string(json.EncodeBytes(o.Items))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(o, c))
}

func (o Cart) ItemsJSON() string {
	return json.Encode(o.Items)
}

func (o Cart) IntId() int {
	return int(o.Key().IntID())
}

func (o Cart) DisplayId() string {
	return strconv.Itoa(o.IntId())
}

func (o Cart) DisplayCreatedAt() string {
	duration := time.Since(o.CreatedAt)

	if duration.Hours() > 24 {
		year, month, day := o.CreatedAt.Date()
		return fmt.Sprintf("%s %s, %s", month.String(), strconv.Itoa(day), strconv.Itoa(year))
	}

	return humanize.Time(o.CreatedAt)
}

func (o Cart) DisplaySubtotal() string {
	return DisplayPrice(o.Currency, o.Subtotal)
}

func (o Cart) DisplayDiscount() string {
	return DisplayPrice(o.Currency, o.Discount)
}

func (o Cart) DisplayTax() string {
	return DisplayPrice(o.Currency, o.Tax)
}

func (o Cart) DisplayShipping() string {
	return DisplayPrice(o.Currency, o.Shipping)
}

func (o Cart) DisplayTotal() string {
	return DisplayPrice(o.Currency, o.Total)
}

func (o Cart) Description() string {
	if o.Items == nil {
		return ""
	}

	buffer := bytes.NewBufferString("")

	for i, item := range o.Items {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(item.String())
		buffer.WriteString(" x")
		buffer.WriteString(strconv.Itoa(item.Quantity))
	}
	return buffer.String()
}
