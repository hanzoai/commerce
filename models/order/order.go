package order

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/store"
	"crowdstart.com/models/types/country"
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
	Open      Status = "open"
	Locked           = "locked"
	Cancelled        = "cancelled"
	Completed        = "completed"
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

	// Type of order
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
	Adjustment currency.Cents `json:"adjustment"`

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
	Items []LineItem `json:"items"`

	Adjustments []Adjustment `json:"adjustments,omitempty"`

	Coupons     []coupon.Coupon `json:"coupons,omitempty"`
	CouponCodes []string        `json:"couponCodes,omitempty"`
	ReferrerId  string          `json:"referrerId,omitempty"`

	PaymentIds []string `json:"payments"`

	// Fulfillment information
	Fulfillment Fulfillment `json:"fulfillment"`

	// Series of events that have occured relevant to this order
	History []Event `json:"history,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ string   `json:"-" datastore:",noindex"`

	Test bool `json:"-"` // Whether our internal test flag is active or not
}

func (o *Order) Init() {
	o.Adjustments = make([]Adjustment, 0)
	o.History = make([]Event, 0)
	o.Items = make([]LineItem, 0)
	o.Metadata = make(Metadata)
	o.Coupons = make([]coupon.Coupon, 0)
}

func New(db *datastore.Datastore) *Order {
	o := new(Order)
	o.Init()
	o.Model = mixin.Model{Db: db, Entity: o}

	o.Status = Open
	o.PaymentStatus = payment.Unpaid
	o.FulfillmentStatus = FulfillmentUnfulfilled
	return o
}

func (o Order) Kind() string {
	return "order"
}

func (o Order) Document() mixin.Document {
	return &Document{
		o.Id(),
		o.UserId,

		o.BillingAddress.Line1,
		o.BillingAddress.Line2,
		o.BillingAddress.City,
		o.BillingAddress.State,
		country.ByISOCodeISO3166_2[o.BillingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.BillingAddress.PostalCode,

		o.ShippingAddress.Line1,
		o.ShippingAddress.Line2,
		o.ShippingAddress.City,
		o.ShippingAddress.State,
		country.ByISOCodeISO3166_2[o.ShippingAddress.Country].ISO3166OneEnglishShortNameReadingOrder,
		o.ShippingAddress.PostalCode,
	}
}

func (o *Order) Validator() *val.Validator {
	return val.New(o)
}

func (o *Order) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	o.Init()

	// Set order number
	o.Number = o.NumberFromId()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(o, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(o.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(o.Metadata_), &o.Metadata)
	}

	return err
}

func (o *Order) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	o.Metadata_ = string(json.EncodeBytes(&o.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(o, c))
}

func (o Order) Fee() currency.Cents {
	return currency.Cents(math.Floor(float64(o.Total) * 0.02))
}

func (o Order) NumberFromId() int {
	return hashid.Decode(o.Id_)[1]
}

func (o Order) Description() string {
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

// Get line items from datastore
func (o *Order) GetCoupons() error {
	o.DedupeCouponCodes()
	db := o.Model.Db

	num := len(o.CouponCodes)
	o.Coupons = make([]coupon.Coupon, num, num)
	keys := make([]datastore.Key, num, num)

	for i := 0; i < num; i++ {
		c := coupon.New(db)
		ok, err := c.Query().Filter("Code=", o.CouponCodes[i]).KeysOnly().First()
		if err != nil {
			return err
		}

		if !ok {
			return errors.New("Invalid coupon code")
		}

		keys[i] = c.Key()
	}

	return db.GetMulti(keys, o.Coupons)
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

	for i := 0; i < num; i++ {
		c := &o.Coupons[i]
		if !c.Enabled {
			continue
		}

		if c.ProductId == "" {
			// Coupons per product
			switch c.Type {
			case coupon.Flat:
				o.Discount = currency.Cents(int(o.Discount) + c.Amount)
			case coupon.Percent:
				for _, item := range o.Items {
					o.Discount = currency.Cents(int(o.Discount) + int(math.Floor(float64(item.TotalPrice())*float64(c.Amount)*0.01)))
				}
			case coupon.FreeShipping:
				o.Discount = currency.Cents(int(o.Discount) + int(o.Shipping))
			}
		} else {
			// Coupons per product
			for _, item := range o.Items {
				log.Warn("Coupon.ProductId: %v, Item.ProductId: %v", c.ProductId, item.ProductId, ctx)
				// log.Warn("%v, %v ==? %v", item.ProductName, item.ProductId, c.ProductId)
				if item.ProductId == c.ProductId {
					switch c.Type {
					case coupon.Flat:
						o.Discount = currency.Cents(int(o.Discount) + (item.Quantity * c.Amount))
					case coupon.Percent:
						o.Discount = currency.Cents(int(o.Discount) + int(math.Floor(float64(item.TotalPrice())*float64(c.Amount)*0.01)))
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

	// Get coupons from datastore
	if err := o.GetCoupons(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get coupons")
	}

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
	return DisplayPrice(o.Subtotal)
}

func (o Order) DisplayDiscount() string {
	return DisplayPrice(o.Discount)
}

func (o Order) DisplayTax() string {
	return DisplayPrice(o.Tax)
}

func (o Order) DisplayShipping() string {
	return DisplayPrice(o.Shipping)
}

func (o Order) DisplayTotal() string {
	return DisplayPrice(o.Total)
}

func (o Order) DecimalTotal() uint64 {
	return uint64(FloatPrice(o.Total) * 100)
}

func (o Order) DecimalFee() uint64 {
	return uint64(FloatPrice(o.Total) * 100 * 0.02)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
