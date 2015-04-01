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

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/coupon"
	"crowdstart.io/models2/payment"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/util/gob"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/val"

	. "crowdstart.io/models2"
	. "crowdstart.io/models2/lineitem"
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

	PaymentIds []string `json:"payments"`

	// Fulfillment information
	Fulfillment Fulfillment `json:"fulfillment"`

	// Series of events that have occured relevant to this order
	History []Event `json:"history,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Metadata `json:"metadata" datastore:"-"`
	Metadata_ []byte   `json:"-"`

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
	return o
}

func (o Order) Kind() string {
	return "order"
}

func (o *Order) Validator() *val.Validator {
	return val.New(o)
}

func (o *Order) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	o.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(o, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(o.Metadata_) > 0 {
		err = gob.Decode(o.Metadata_, &o.Metadata)
	}

	return err
}

func (o *Order) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	o.Metadata_, err = gob.Encode(&o.Metadata)

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(o, c))
}

func (o Order) Fee() currency.Cents {
	return currency.Cents(math.Floor(float64(o.Total) * 0.02))
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

		keys = append(keys, c.Key())
	}

	return db.GetMulti(keys, &o.Coupons)
}

// Update discount using coupon codes/order info.
func (o *Order) UpdateDiscount() {
	o.Discount = 0
	num := len(o.CouponCodes)
	for i := 0; i < num; i++ {
		c := &o.Coupons[i]
		switch c.Type {
		case coupon.Flat:
			o.Discount = currency.Cents(int(o.Discount) + c.Amount)
		case coupon.Percent:
			o.Discount = currency.Cents(int(o.Discount) + (int(o.Total) * c.Amount))
		case coupon.FreeShipping:
			o.Discount = currency.Cents(int(o.Discount) + int(o.Shipping))
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
func (o *Order) UpdateAndTally() error {
	ctx := o.Db.Context

	// Get coupons from datastore
	if err := o.GetCoupons(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get coupons")
	}

	// Update discount amount
	o.UpdateDiscount()

	// Get underlying product/variant entities
	if err := o.GetItemEntities(); err != nil {
		log.Error(err, ctx)
		return errors.New("Failed to get underlying line items")
	}

	// Update line items using that information
	o.UpdateFromEntities()

	// Tally up order again
	o.Tally()

	return nil
}

func (o Order) ItemsJSON() string {
	return json.Encode(o.Items)
}

// var variantsMap map[string]Variant
// var salesforceVariantsMap map[string]Variant
// var productsMap map[string]Product

// func (o Order) EstimatedDeliveryHTML() string {
// 	return "<div>" + strings.Replace(o.EstimatedDelivery, ",", "</div><div>", -1) + "</div>"
// }

// func (o Order) DisputedCharges(c *gin.Context) (disputedCharges []Charge) {
// 	for _, charge := range o.Charges {
// 		if charge.Disputed {
// 			disputedCharges = append(disputedCharges, charge)
// 		}
// 	}
// 	return disputedCharges
// }

// func (o *Order) LoadVariantsProducts(c interface{}) {
// 	if variantsMap == nil || productsMap == nil || salesforceVariantsMap == nil {
// 		db := datastore.New(c)

// 		variantsMap = make(map[string]ProductVariant)
// 		salesforceVariantsMap = make(map[string]ProductVariant)
// 		var variants []ProductVariant
// 		db.Query("variant").GetAll(db.Context, &variants)
// 		for _, variant := range variants {
// 			variantsMap[variant.SKU] = variant
// 			salesforceVariantsMap[variant.SecondarySalesforceId_] = variant
// 		}

// 		productsMap = make(map[string]Product)
// 		var products []Product
// 		db.Query("product").GetAll(db.Context, &products)
// 		for _, product := range products {
// 			productsMap[product.Slug] = product
// 		}
// 	}

// 	for i, item := range o.Items {
// 		// We might need to derive Slug_ from Sku_
// 		if item.Slug_ == "" && item.SKU_ != "" {
// 			for slug, _ := range productsMap {
// 				upperSKU := strings.ToUpper(item.SKU_)
// 				upperSlug := strings.ToUpper(slug)
// 				if strings.HasPrefix(upperSKU, upperSlug) {
// 					// Remember that item is a copy and not the actual object
// 					o.Items[i].Slug_ = slug
// 					break
// 				}
// 			}
// 			log.Warn("Slug was missing on line item, guessed slug is '%v' based on SKU '%v'", o.Items[i].Slug_, item.SKU_, c)
// 		}
// 		o.Items[i].Product = productsMap[item.Slug_]

// 		// We might need to look up using sf id
// 		var ok bool
// 		if o.Items[i].Variant, ok = variantsMap[item.SKU_]; !ok {
// 			if o.Items[i].Variant, ok = salesforceVariantsMap[item.PrimarySalesforceId_]; !ok {
// 				o.Items[i].Variant, ok = salesforceVariantsMap[item.SecondarySalesforceId_]
// 			}
// 		}

// 		o.Items[i].VariantId = o.Items[i].VariantId
// 	}
// }

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

// Use binding to validate that there are no errors
// func (o Order) Validate(req *http.Request, errs binding.Errors) binding.Errors {
// 	if len(o.Items) == 0 {
// 		errs = append(errs, binding.Error{
// 			FieldNames:     []string{"Items"},
// 			Classification: "InputError",
// 			Message:        "Order has no items.",
// 		})
// 	} else {
// 		for _, v := range o.Items {
// 			errs = v.Validate(req, errs)
// 		}
// 	}

// 	return errs
// }

// // Repopulate order with data from database, variant options, etc., and
// // recalculate totals.
// func (o *Order) Populate(db *datastore.Datastore) error {
// 	// TODO: Optimize this, multiget, use caching.
// 	for i, item := range o.Items {
// 		// Fetch Variant for LineItem from datastore
// 		if err := db.GetKind("variant", item.SKU(), &item.Variant); err != nil {
// 			return err
// 		}

// 		// Fetch Product for LineItem from datastore
// 		if err := db.GetKind("product", item.Slug(), &item.Product); err != nil {
// 			return err
// 		}

// 		// Set SKU so we can deserialize later
// 		item.SKU_ = item.SKU()
// 		item.Slug_ = item.Slug()

// 		// Update item in order
// 		o.Items[i] = item

// 		// Update subtotal
// 		o.Subtotal += item.Price()
// 	}

// 	// Update grand total
// 	o.Total = o.Subtotal + o.Tax + o.Shipping
// 	return nil
// }

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
