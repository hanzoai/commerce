package order

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/country"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/pricing"
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

	// Associated Crowdstart user or buyer.
	UserId string `json:"userId,omitempty"`
	Email  string `json:"email,omitempty"`

	// Associated cart
	CartId string `json:"cartId,omitempty"`

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

	Adjustments []Adjustment `json:"-"`

	Coupons     []coupon.Coupon `json:"coupons,omitempty"`
	CouponCodes []string        `json:"couponCodes,omitempty"`
	ReferrerId  string          `json:"referrerId,omitempty"`

	PaymentIds []string `json:"payments"`

	// Date order was cancelled at
	CancelledAt time.Time `json:"cancelledAt,omitempty"`

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

	Mailchimp struct {
		Id           string `json:"id,omitempty"`
		CampaignId   string `json:"campaignId,omitempty"`
		TrackingCode string `json:"trackingCode,omitempty"`
	} `json:"mailchimp,omitempty"`
}

func (o Order) Document() mixin.Document {
	preorder := "true"
	if !o.Preorder {
		preorder = "false"
	}
	confirmed := "true"
	if o.Unconfirmed {
		confirmed = "false"
	}

	productIds := make([]string, 0)
	for _, item := range o.Items {
		productIds = append(productIds, item.ProductId)
		productIds = append(productIds, item.ProductSlug)
	}

	return &Document{
		o.Id(),
		o.UserId,

		strings.Join(productIds, " "),

		o.BillingAddress.Line1,
		o.BillingAddress.Line2,
		o.BillingAddress.City,
		o.BillingAddress.State,
		o.BillingAddress.Country,
		country.ByISOCodeISO3166_2[o.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.BillingAddress.PostalCode,

		o.ShippingAddress.Line1,
		o.ShippingAddress.Line2,
		o.ShippingAddress.City,
		o.ShippingAddress.State,
		o.BillingAddress.Country,
		country.ByISOCodeISO3166_2[o.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.ShippingAddress.PostalCode,

		o.Type,

		o.CreatedAt,
		o.UpdatedAt,

		string(o.Currency),
		float64(o.Total),
		strings.Join(o.CouponCodes, " "),
		o.ReferrerId,

		string(o.Status),
		string(o.PaymentStatus),
		string(o.FulfillmentStatus),
		string(preorder),
		string(confirmed),
	}
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

func (o *Order) AddAffiliateFee(pricing pricing.Fees, fees []*fee.Fee) ([]*fee.Fee, error) {
	if o.ReferrerId == "" {
		// No referrer, no need to check affiliate
		return fees, nil
	}

	ctx := o.Context()
	db := datastore.New(ctx)

	// Lookup referrer
	ref := referrer.New(db)
	if err := ref.GetById(o.ReferrerId); err != nil {
		return fees, err
	}

	if ref.AffiliateId == "" {
		// No affiliate, no fee
		return fees, nil
	}

	// Lookup affiliate
	aff := affiliate.New(db)
	if err := aff.GetById(ref.AffiliateId); err != nil {
		return fees, err
	}

	// Compute fees
	affFee := currency.Cents(math.Floor(float64(o.Total)*aff.Commission.Percent)) + aff.Commission.Flat
	platformFee := currency.Cents(math.Floor(float64(affFee)*pricing.Affiliate.Percent)) + pricing.Affiliate.Flat

	// Create affiliate fee
	fe := fee.New(db)
	fe.Parent = aff.Key()
	fe.Type = fee.Affiliate
	fe.Currency = o.Currency
	fe.Amount = affFee

	fees = append(fees, fe)

	// Create platform fee
	fe = fee.New(db)
	fe.Parent = pricing.Key(ctx)
	fe.Type = fee.Platform
	fe.Currency = o.Currency
	fe.Amount = platformFee

	return append(fees, fe), nil
}

func (o *Order) AddPlatformFee(pricing pricing.Fees, fees []*fee.Fee) []*fee.Fee {
	ctx := o.Context()
	db := datastore.New(ctx)

	// Add platform fee
	fe := fee.New(db)
	fe.Parent = pricing.Key(ctx)
	fe.Type = fee.Platform
	fe.Currency = o.Currency
	fe.Amount = pricing.Card.Flat + currency.Cents(math.Ceil(float64(o.Total)*pricing.Card.Percent)) // Round up for platform fee

	return append(fees, fe)
}

func (o *Order) AddPartnerFee(partners []pricing.Partner, fees []*fee.Fee) ([]*fee.Fee, error) {
	ctx := o.Context()
	db := datastore.New(ctx)

	// Add partner fees
	for _, partner := range partners {
		fe := fee.New(db)
		fe.Parent = partner.Key(ctx)
		fe.Type = fee.Platform
		fe.Currency = o.Currency
		fe.Amount = partner.Commission.Flat + currency.Cents(math.Floor(float64(o.Total)*partner.Commission.Percent))
		fees = append(fees, fe)
	}

	return fees, nil
}

func (o *Order) CalculateFees(pricing pricing.Fees, partners []pricing.Partner) (currency.Cents, []*fee.Fee, error) {
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

// Get line items from datastore
func (o *Order) GetCoupons() error {
	o.DedupeCouponCodes()
	db := o.Model.Db
	ctx := db.Context

	log.Debug("CouponCodes: %#v", o.CouponCodes)
	num := len(o.CouponCodes)
	o.Coupons = make([]coupon.Coupon, num, num)

	for i := 0; i < num; i++ {
		cpn := coupon.New(db)
		code := strings.TrimSpace(strings.ToUpper(o.CouponCodes[i]))

		log.Debug("CODE: %s", code)
		err := cpn.GetById(code)

		if err != nil {
			log.Warn("Could not find CouponCodes[%v] => %v, Error: %v", i, code, err, ctx)
			return errors.New("Invalid coupon code: " + code)
		}

		o.Coupons[i] = *cpn
	}

	return nil
}

func (o *Order) DedupeCouponCodes() {
	found := make(map[string]bool)
	j := 0
	for i, code := range o.CouponCodes {
		if !found[code] {
			found[code] = true
			o.CouponCodes[j] = o.CouponCodes[i]
			j++
		}
	}
	o.CouponCodes = o.CouponCodes[:j]
}

// Check if there is a discount
func (o Order) HasDiscount() bool {
	if o.Discount != currency.Cents(0) {
		return true
	}
	return false
}

// Update discount using coupon codes/order info.
func (o *Order) UpdateDiscount() {
	o.Discount = 0

	num := len(o.CouponCodes)

	ctx := o.Model.Db.Context

	log.Warn("TRYING TO APPLY COUPONS", ctx)
	for i := 0; i < num; i++ {
		c := &o.Coupons[i]
		if !c.ValidFor(o.CreatedAt) {
			continue
		}

		log.Warn("TRYING TO APPLY COUPON %v", c.Code(), ctx)

		if c.ItemId() == "" {
			log.Warn("Coupon Applies to All", ctx)

			// Not per product
			switch c.Type {
			case coupon.Flat:
				log.Warn("Flat", ctx)
				o.Discount += currency.Cents(c.Amount)
			case coupon.Percent:
				log.Warn("Percent", ctx)
				for _, item := range o.Items {
					o.Discount += currency.Cents(int(math.Floor(float64(item.TotalPrice()) * float64(c.Amount) * 0.01)))
				}
			case coupon.FreeShipping:
				log.Warn("FreeShipping", ctx)
				o.Discount += currency.Cents(int(o.Shipping))
			}
		} else {
			log.Warn("Coupon Applies to %v", c.ItemId(), ctx)
			// Coupons per product
			for _, item := range o.Items {
				log.Debug("Coupon.ProductId: %v, Item.ProductId: %v", c.ProductId, item.ProductId, ctx)
				if item.Id() == c.ItemId() {
					switch c.Type {
					case coupon.Flat:
						log.Warn("Flat", ctx)
						o.Discount += currency.Cents(item.Quantity * c.Amount)
					case coupon.Percent:
						log.Warn("Percent", ctx)
						o.Discount += currency.Cents(math.Floor(float64(item.TotalPrice()) * float64(c.Amount) * 0.01))
					case coupon.FreeItem:
						log.Warn("FreeShipping", ctx)
						o.Discount += currency.Cents(item.Price)
					}

					// Break out unless required to apply to each product
					if c.Once {
						break
					}
				}
			}
		}
	}
}

// Update discount using coupon codes/order info.
// Refactor later when we have more time to think about it
func (o *Order) UpdateCouponItems() error {
	nCodes := len(o.CouponCodes)

	items := make([]LineItem, 0)
	for _, item := range o.Items {
		if item.AddedBy != "coupon" {
			items = append(items, item)
		}
	}

	o.Items = items

	for i := 0; i < nCodes; i++ {
		c := &o.Coupons[i]
		if !c.ValidFor(o.CreatedAt) {
			continue
		}
		if c.ProductId == "" {
			switch c.Type {
			case coupon.FreeItem:
				o.Items = append(o.Items, LineItem{
					ProductId: c.FreeProductId,
					VariantId: c.FreeVariantId,
					Quantity:  c.FreeQuantity,
					Free:      true,
					AddedBy:   "coupon",
				})
			}
		} else {
			for _, item := range o.Items {
				if item.ProductId == c.ProductId {
					switch c.Type {
					case coupon.FreeItem:
						o.Items = append(o.Items, LineItem{
							ProductId: c.FreeProductId,
							VariantId: c.FreeVariantId,
							Quantity:  c.FreeQuantity,
							Free:      true,
							AddedBy:   "coupon",
						})
					}
				}
			}
		}
	}

	return nil
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

// Calculate total of an order
func (o *Order) Tally() {
	// Update total
	subtotal := 0
	nItems := len(o.Items)
	for i := 0; i < nItems; i++ {
		subtotal += o.Items[i].Quantity * int(o.Items[i].Price)
	}
	o.LineTotal = currency.Cents(subtotal)

	// TODO: Make this use shipping/tax information
	discount := int(o.Discount)
	shipping := int(o.Shipping)
	tax := int(o.Tax)
	subtotal = subtotal - discount
	total := subtotal + tax + shipping

	o.Subtotal = currency.Cents(subtotal)
	o.Total = currency.Cents(total)
}

// Update order with information from datastore and tally
func (o *Order) UpdateAndTally(stor *store.Store) error {
	ctx := o.Db.Context

	// Get coupons from datastore
	if err := o.GetCoupons(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get coupons")
	}

	for _, coup := range o.Coupons {
		if !coup.Redeemable() {
			return errors.New(fmt.Sprintf("Coupon %v limit reached", coup.Code()))
		}
	}

	// Update the list of free coupon items
	o.UpdateCouponItems()

	// Get underlying product/variant entities
	if err := o.GetItemEntities(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get underlying line items")
	}

	// Update against store listings
	if stor != nil {
		o.UpdateEntities(stor)
	}

	// Update line items using that information
	o.UpdateFromEntities()

	// Update discount amount
	o.UpdateDiscount()

	// Tally up order again
	o.Tally()

	return nil
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
