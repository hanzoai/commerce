package order

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"hanzo.io/datastore"
	"hanzo.io/models/affiliate"
	"hanzo.io/models/coupon"
	"hanzo.io/models/discount"
	"hanzo.io/models/fee"
	"hanzo.io/models/mixin"
	"hanzo.io/models/payment"
	"hanzo.io/models/referrer"
	"hanzo.io/models/store"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/fulfillment"
	"hanzo.io/models/types/pricing"
	"hanzo.io/models/wallet"
	"hanzo.io/util/hashid"
	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/val"

	. "hanzo.io/models"
	"hanzo.io/models/lineitem"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Status string

const (
	Cancelled Status = "cancelled"
	Completed Status = "completed"
	Locked    Status = "locked"
	OnHold    Status = "on-hold"
	Open      Status = "open"
)

func init() {
	// This type must match exactly what youre going to be using,
	// down to whether or not its a pointer
	gob.Register(&Order{})
}

type Order struct {
	mixin.Model
	mixin.Salesforce `json:"-"`
	wallet.WalletHolder

	Number int `json:"number,omitempty"`

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
	Status        Status         `json:"status"`
	PaymentStatus payment.Status `json:"paymentStatus"`

	// Whether this was a preorder or not
	Preorder bool `json:"preorder"`

	// Order is unconfirmed if user has not declared (either implicitly or
	// explicitly) precise order variant options.
	Unconfirmed bool `json:"unconfirmed,omitempty"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type `json:"currency"`

	// Payment processor type - paypal, stripe, etc
	Type payment.Type `json:"type,omitempty"`

	// Shipping method
	ShippingMethod string `json:"shippingMethod,omitempty"`

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
	Balance currency.Cents `json:"balance,omitempty"`

	// Gross amount paid to the seller. Amount in cents.
	Paid currency.Cents `json:"paid,omitempty"`

	// integer	Amount refunded by the seller. Amount in cents.
	Refunded currency.Cents `json:"refunded"`

	Company         string  `json:"company,omitempty"`
	BillingAddress  Address `json:"billingAddress"`
	ShippingAddress Address `json:"shippingAddress"`

	// Individual line items
	Items  []lineitem.LineItem `json:"items" datastore:"-"`
	Items_ string              `json:"-" datastore:",noindex"`

	Adjustments []Adjustment `json:"-"`

	Discounts  []*discount.Discount `json:"discounts,omitempty" datastore:"-"`
	Discounts_ string               `json:"-" datastore:",noindex"` // need props

	Coupons     []coupon.Coupon `json:"coupons,omitempty" datastore:",noindex"`
	CouponCodes []string        `json:"couponCodes,omitempty" datastore:",noindex"`

	PaymentIds []string           `json:"payments" datastore:",noindex"`
	Payments   []*payment.Payment `json:"-" datastore:"-"`

	// Date order was cancelled at
	CancelledAt time.Time `json:"cancelledAt,omitempty"`

	// Fulfillment information
	Fulfillment fulfillment.Fulfillment `json:"fulfillment"`

	// Return ids
	ReturnIds []string `json:"returnIds" datastore:",noindex"`

	// Gift options
	Gift        bool   `json:"gift,omitempty"`                             // Is this a gift?
	GiftMessage string `json:"giftMessage,omitempty" datastore:",noindex"` // Message to go on gift
	GiftEmail   string `json:"giftEmail,omitempty"`                        // Email for digital gifts

	// Contribution are orders without items
	Contribution bool `json:"contribution"`

	// Token sales are processed differently, similar to contribution
	TokenSaleId string `json:"tokenSaleId,omitempty"`

	// Mailchimp tracking information
	Mailchimp struct {
		Id           string `json:"id,omitempty" datastore:",noindex"`
		CampaignId   string `json:"campaignId,omitempty"`
		TrackingCode string `json:"trackingCode,omitempty" datastore:",noindex"`
	} `json:"mailchimp,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty" datastore:",noindex"`

	Test bool `json:"-"` // Whether our internal test flag is active or not

	// Passphrase for the wallet accounts the order controls, never send to the client
	WalletPassphrase string `json:"-"`

	// At what point do we stop taking payments
	// PaymentStop time.Time `json:"paymentStop"`
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

	// Deserialize from datastore
	if len(o.Discounts_) > 0 {
		err = json.DecodeBytes([]byte(o.Discounts_), &o.Discounts)
	}

	if len(o.Items_) > 0 {
		err = json.DecodeBytes([]byte(o.Items_), &o.Items)
	}

	if len(o.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(o.Metadata_), &o.Metadata)
	}

	// Initalize coupons
	for _, coup := range o.Coupons {
		coup.Init(o.Model.Db)
	}

	// Initalize discounts
	for _, dis := range o.Discounts {
		dis.Init(o.Model.Db)
	}

	return err
}

func (o *Order) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	o.Discounts_ = string(json.EncodeBytes(o.Discounts))
	o.Items_ = string(json.EncodeBytes(o.Items))
	o.Metadata_ = string(json.EncodeBytes(&o.Metadata))
	o.Number = o.NumberFromId()

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(o, c))
}

func (o *Order) AddAffiliateFee(pricing *pricing.Fees, fees []*fee.Fee) ([]*fee.Fee, error) {
	log.Info("Add Affiliate Fee")

	if o.ReferrerId == "" {
		// No referrer, no need to check affiliate
		log.Info("No ReferrerId '%s'", o.ReferrerId)
		return fees, nil
	}

	ctx := o.Context()
	db := datastore.New(ctx)

	// Lookup referrer
	log.Info("Try to Get Referrer '%s'", o.ReferrerId)
	ref := referrer.New(db)
	if err := ref.GetById(o.ReferrerId); err != nil {
		log.Error("No Referrer '%s'", o.ReferrerId, o.Context)
		return fees, nil
	}

	if ref.AffiliateId == "" {
		// No affiliate, no fee
		log.Info("No Affiliate Id")
		return fees, nil
	}

	// Lookup affiliate
	log.Info("Try to Get Affiliate '%s'", o.ReferrerId)
	aff := affiliate.New(db)
	if err := aff.GetById(ref.AffiliateId); err != nil {
		log.Error("No Affiliate", o.Context)
		return fees, err
	}

	// Compute fees
	affFee := currency.Cents(math.Floor(float64(o.Total)*aff.Commission.Percent)) + aff.Commission.Flat
	platformFee := currency.Cents(math.Ceil(float64(affFee)*pricing.Affiliate.Percent)) + pricing.Affiliate.Flat

	// Create affiliate fee
	fe := fee.New(db)
	fe.Name = "Affiliate commission"
	fe.Parent = aff.Key()
	fe.Type = fee.Affiliate
	fe.Currency = o.Currency
	fe.AffiliateId = aff.Id()
	fe.Amount = affFee

	fees = append(fees, fe)

	// Create platform fee
	fe = fee.New(db)
	fe.Name = "Affiliate fee"
	fe.Type = fee.Platform
	fe.Currency = o.Currency
	fe.Amount = platformFee

	return append(fees, fe), nil
}

func (o *Order) AddPlatformFee(pricing *pricing.Fees, fees []*fee.Fee) []*fee.Fee {
	ctx := o.Context()
	db := datastore.New(ctx)

	// Add platform fee
	fe := fee.New(db)
	fe.Name = "Platform fee"
	fe.Parent = pricing.Key(ctx)
	fe.Type = fee.Platform
	fe.Currency = o.Currency

	switch o.Currency {
	case currency.ETH:
		fe.Amount = pricing.Ethereum.Flat + currency.Cents(math.Ceil(float64(o.Total)*pricing.Ethereum.Percent)) // Round up for platform fee
	case currency.BTC, currency.XBT:
		fe.Amount = pricing.Bitcoin.Flat + currency.Cents(math.Ceil(float64(o.Total)*pricing.Bitcoin.Percent)) // Round up for platform fee
	default:
		fe.Amount = pricing.Card.Flat + currency.Cents(math.Ceil(float64(o.Total)*pricing.Card.Percent)) // Round up for platform fee
	}

	return append(fees, fe)
}

func (o *Order) AddPartnerFee(partners []pricing.Partner, fees []*fee.Fee) ([]*fee.Fee, error) {
	ctx := o.Context()
	db := datastore.New(ctx)

	// Add partner fees
	for _, partner := range partners {
		fe := fee.New(db)
		fe.Name = "Partner fee"
		fe.Parent = partner.Key(ctx)
		fe.Type = fee.Platform
		fe.Currency = o.Currency

		switch o.Currency {
		case currency.ETH:
			fe.Amount = partner.Ethereum.Commission.Flat + currency.Cents(math.Ceil(float64(o.Total)*partner.Ethereum.Commission.Percent)) // Round up for platform fee
		case currency.BTC, currency.XBT:
			fe.Amount = partner.Bitcoin.Commission.Flat + currency.Cents(math.Ceil(float64(o.Total)*partner.Bitcoin.Commission.Percent)) // Round up for platform fee
		default:
			fe.Amount = partner.Card.Commission.Flat + currency.Cents(math.Ceil(float64(o.Total)*partner.Card.Commission.Percent)) // Round up for platform fee
		}

		fees = append(fees, fe)
	}

	return fees, nil
}

func (o *Order) CalculateFees(pricing *pricing.Fees, partners []pricing.Partner) (currency.Cents, []*fee.Fee, error) {
	fees := make([]*fee.Fee, 0)
	total := currency.Cents(0)

	// Add Affiliate fees
	fees, err := o.AddAffiliateFee(pricing, fees)
	if err != nil {
		return total, fees, err
	}

	// Add Platform fees
	fees = o.AddPlatformFee(pricing, fees)

	// Add Partner fees
	fees, err = o.AddPartnerFee(partners, fees)
	if err != nil {
		return total, fees, err
	}

	// Calculate total fee collected
	for _, fe := range fees {
		total += fe.Amount
	}

	return total, fees, nil
}

func (o Order) NumberFromId() int {
	ids, err := hashid.Decode(o.Id())
	if err != nil {
		panic(err)
	}
	return ids[1]
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
	db := o.Datastore()
	ctx := o.Context()

	log.Debug("Getting underlying entities for: %v", json.Encode(o.Items))

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
		log.Debug("key %v", key)
		vals[i] = dst
		log.Debug("dst %v", json.Encode(dst))
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

	// Update order to reflecte which store was used
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

func (o Order) DisplayRemaining() string {
	return DisplayPrice(o.Currency, o.Total-o.Refunded)
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
	if err := payment.Query(o.Db).Ancestor(o.Key()).GetModels(&payments); err != nil {
		return nil, err
	}
	return payments, nil
}
