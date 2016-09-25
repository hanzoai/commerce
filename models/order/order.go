package order

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/discount"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
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

type Order struct {
	mixin.Model
	mixin.Salesforce `json:"-"`

	Number int `json:"number,omitempty" datastore:"-"`

	// Store this was sold from (if any)
	StoreId string `json:"storeId,omitempty"`

	// Associated campaign
	CampaignId string `json:"campaignId,omitempty"`

	// Associated user or buyer.
	UserId string `json:"userId,omitempty"`
	Email  string `json:"email,omitempty"`

	// Associated cart
	CartId string `json:"cartId,omitempty"`

	// Associated referrer
	ReferrerId string `json:"referrerId,omitempty"`

	// Status
	Status            Status            `json:"status"`
	PaymentStatus     payment.Status    `json:"paymentStatus"`
	FulfillmentStatus FulfillmentStatus `json:"fulfillmentStatus"`

	// Whether this was a preorder or not
	Preorder bool `json:"preorder"`

	// Order is unconfirmed if user has not declared (either implicitly or
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

	Company         string  `json:"company,omitempty"`
	BillingAddress  Address `json:"billingAddress"`
	ShippingAddress Address `json:"shippingAddress"`

	// Individual line items
	Items  []LineItem `json:"items" datastore:"-"`
	Items_ string     `json:"-"` // need props

	Adjustments []Adjustment        `json:"-"`
	Discounts   []discount.Discount `json:"discounts,omitempty"`
	Coupons     []coupon.Coupon     `json:"coupons,omitempty"`
	CouponCodes []string            `json:"couponCodes,omitempty"`

	PaymentIds []string `json:"payments"`

	// Date order was cancelled at
	CancelledAt time.Time `json:"cancelledAt,omitempty"`

	// Fulfillment information
	Fulfillment Fulfillment `json:"fulfillment"`

	// gift options
	Gift        bool   `json:"gift"`        // Is this a gift?
	GiftMessage string `json:"giftMessage"` // Message to go on gift
	GiftEmail   string `json:"giftEmail"`   // Email for digital gifts

	// Mailchimp tracking information
	Mailchimp struct {
		Id           string `json:"id,omitempty"`
		CampaignId   string `json:"campaignId,omitempty"`
		TrackingCode string `json:"trackingCode,omitempty"`
	} `json:"mailchimp,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty"`

	Test bool `json:"-"` // Whether our internal test flag is active or not
}

func (o *Order) Validator() *val.Validator {
	return val.New()
}

func (o *Order) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	o.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(o, c)); err != nil {
		return err
	}

	// Set order number
	o.Number = o.NumberFromId()

	// Initalize coupons
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

func (o *Order) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	o.Metadata_ = string(json.EncodeBytes(&o.Metadata))
	o.Items_ = string(json.EncodeBytes(o.Items))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(o, c))
}

func (o Order) CalculateFee(percent float64) currency.Cents {
	// Default to config.Fee if percent is not provided
	if percent <= 0 {
		percent = config.Fee
	}

	return currency.Cents(math.Floor(float64(o.Total) * percent))
}

func (o Order) NumberFromId() int {
	if o.Id_ == "" {
		return -1
	}
	return hashid.Decode(o.Id_)[1]
}

func (o Order) OrderDay() string {
	return string(o.CreatedAt.Day())
}

func (o Order) OrderMonthName() string {
	return o.CreatedAt.Month().String()
}

func (o Order) OrderYear() string {
	return string(o.CreatedAt.Year())
}

// Check if there is a discount
func (o Order) HasDiscount() bool {
	if o.Discount != currency.Cents(0) {
		return true
	}
	return false
}

// Update order's payment status based on payments
func (o *Order) UpdatePaymentStatus() {
	keys := make([]*aeds.Key, len(o.PaymentIds))
	ctx := o.Context()

	// Convert payment ids into keys
	for i, id := range o.PaymentIds {
		if key, err := hashid.DecodeKey(ctx, id); err != nil {
			log.Error("Unable to decode payment id into Key %s", id, ctx)
		} else {
			keys[i] = key
		}
	}

	// Get payments associated with this order
	payments := make([]payment.Payment, len(o.PaymentIds))

	db := datastore.New(ctx)
	err := db.GetMulti(keys, payments)
	if err != nil {
		log.Error("Unable to fetch payments for order '%s': %v", o.Id(), err, ctx)
		return
	}

	log.Warn(o.PaymentIds)

	// Sum payments to figure out what we've been paid and check for bad status
	var badstatus payment.Status
	failed := false
	disputed := false
	refunded := false
	totalPaid := 0

	for _, pay := range payments {
		switch pay.Status {
		case payment.Paid:
			totalPaid += int(pay.Amount)
		case payment.Failed, payment.Fraudulent:
			badstatus = pay.Status
			failed = true
		case payment.Disputed:
			disputed = true
		case payment.Refunded:
			refunded = true
		}
	}

	// Update order paid amount and status
	o.Paid = currency.Cents(int(o.Paid) + totalPaid)
	// Paid or Partially Refunded
	if o.Paid >= o.Total {
		// TODO Notify user via email.
		o.PaymentStatus = payment.Paid
		if o.Status != Completed {
			o.Status = Open
		}
	}

	if failed {
		// If something bad happened, cancel the order
		log.Warn("Something Bad Happened %v", badstatus)
		o.Status = Cancelled
		o.PaymentStatus = badstatus
	} else if refunded {
		o.Status = Cancelled
		o.PaymentStatus = payment.Refunded
	} else if disputed {
		o.Status = Locked
		o.PaymentStatus = payment.Disputed
	}
}

// Get line items from datastore
func (o *Order) GetItemEntities() error {
	db := o.Model.Db
	ctx := o.Model.Db.Context

	nItems := len(o.Items)
	keys := make([]datastore.Key, nItems, nItems)
	vals := make([]interface{}, nItems, nItems)

	for i := 0; i < nItems; i++ {
		key, dst, err := o.Items[i].Entity(db)
		if err != nil {
			log.Error("Failed to get entity for %#v: %v", o.Items[i], err, ctx)
			return err
		}
		keys[i] = key
		vals[i] = dst
	}

	return db.GetMulti(keys, vals)
}

// Update underlying line item entities using store listings
func (o *Order) UpdateEntities(stor *store.Store) {
	nItems := len(o.Items)
	for i := 0; i < nItems; i++ {
		if o.Items[i].Product != nil {
			stor.UpdateFromListing(o.Items[i].Product)
			continue
		}
		if o.Items[i].Variant != nil {
			stor.UpdateFromListing(o.Items[i].Variant)
		}
	}

	// Update order to reflectw which store was used
	o.StoreId = stor.Id()
}

// Update line items from underlying entities
func (o *Order) UpdateFromEntities() {
	nItems := len(o.Items)
	for i := 0; i < nItems; i++ {
		(&o.Items[i]).Update()
	}
}

func (o Order) ItemsJSON() string {
	return json.Encode(o.Items)
}

func (o Order) IntId() int {
	return int(o.Key().IntID())
}

func (o Order) DisplayId() string {
	return strconv.Itoa(o.IntId())
}

func (o Order) DisplayCreatedAt() string {
	duration := time.Since(o.CreatedAt)

	if duration.Hours() > 24 {
		year, month, day := o.CreatedAt.Date()
		return fmt.Sprintf("%s %s, %s", month.String(), strconv.Itoa(day), strconv.Itoa(year))
	}

	return humanize.Time(o.CreatedAt)
}

func (o Order) DisplaySubtotal() string {
	return DisplayPrice(o.Currency, o.Subtotal)
}

func (o Order) DisplayDiscount() string {
	return DisplayPrice(o.Currency, o.Discount)
}

func (o Order) DisplayTax() string {
	return DisplayPrice(o.Currency, o.Tax)
}

func (o Order) DisplayShipping() string {
	return DisplayPrice(o.Currency, o.Shipping)
}

func (o Order) DisplayTotal() string {
	return DisplayPrice(o.Currency, o.Total)
}

func (o Order) DisplayRefunded() string {
	return DisplayPrice(o.Currency, o.Refunded)
}

func (o Order) Description() string {
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

func (o Order) DescriptionLong() string {
	if o.Items == nil {
		return ""
	}

	buffer := bytes.NewBufferString("")

	for _, li := range o.Items {
		buffer.WriteString(fmt.Sprintf("%v (%v) x %v\n", li.DisplayName(), li.DisplayId(), li.Quantity))
	}

	return buffer.String()
}

func (o Order) GetPayments() ([]*payment.Payment, error) {
	payments := make([]*payment.Payment, 0)

	if _, err := payment.Query(o.Db).Ancestor(o.Key()).GetAll(&payments); err != nil {
		return nil, err
	}

	return payments, nil
}
