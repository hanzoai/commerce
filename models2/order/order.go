package order

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"

	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"

	. "crowdstart.io/models2"
)

type OrderStatus string

const (
	Open      OrderStatus = "open"
	Locked                = "locked"
	Cancelled             = "cancelled"
	Completed             = "completed"
)

type Order struct {
	mixin.Model
	mixin.Salesforce `json:"-"`

	// Associated campaign
	CampaignId string `json:"campaignId"`

	// Associated user, optional. Not necessary for when you use our RESTful
	// API.
	UserId string `json:"userId,omitempty"`

	OrderStatus OrderStatus `json:"orderStatus"`

	PaymentStatus PaymentStatus `json:"paymentStatus"`

	// unfullfilled, fulfilled, processing, failed
	FullfillmentStatus FullfillmentStatus `json:"fullfillmentStatus"`

	// Whether this was a preorder or not
	Preorder bool `json:"preorder"`

	// Order is unconfirmed if user has not declared (either implicitly or
	// explicitly) precise order variant options.
	Unconfirmed bool `json:"unconfirmed"`

	// 3-letter ISO currency code (lowercase).
	Currency CurrencyType `json:"currency"`

	// Seller notes
	Notes string `json:"notes"`

	// Type of order
	Type string `json:"type"`

	// Shipping method
	ShippingMethod string `json:"shippingMethod"`

	// Sum of the line item amounts. Amount in cents.
	LineTotal Cents `json:"lineTotal"`

	// Discount amount applied to the order. Amount in cents.
	Discount Cents `json:"discount"`

	// Sum of line totals less discount. Amount in cents.
	Subtotal Cents `json:"subtotal"`

	// Shipping cost applied. Amount in cents.
	Shipping Cents `json:"shipping"`

	// Sales tax applied. Amount in cents.
	Tax Cents `json:"tax"`

	// Price adjustments applied. Amount in cents.
	Adjustment Cents `json:"adjustment"`

	// Total = subtotal + shipping + taxes + adjustments. Amount in cents.
	Total Cents `json:"total"`

	// Amount owed to the seller. Amount in cents.
	Balance Cents `json:"balance"`

	// Gross amount paid to the seller. Amount in cents.
	Paid Cents `json:"paid"`

	// integer	Amount refunded by the seller. Amount in cents.
	Refunded Cents `json:"refunded"`

	// integer	Crowdstart application fee. Amount in cents.
	Fee Cents `json:"fee"`

	BillingAddress  Address `json:"billingAddress"`
	ShippingAddress Address `json:"shippingAddress"`

	// Individual line items
	Items []LineItem `json:"items"`

	Adjustments []Adjustment `json:"adjustments"`

	Discounts []Discount `json:"discounts"`

	Payments []Payment `json:"payments"`

	// Fullfillment information
	Fullfillment Fullfillment `json:"fullfillment"`

	// Series of events that have occured relevant to this order
	History []Event `json:"history"`

	Test    bool `json:"test"`    // Not a real transaction
	Version int  `json:"version"` // Versioning for struct
}

func New(db *datastore.Datastore) *Order {
	o := new(Order)
	o.Model = mixin.Model{Db: db, Entity: o}
	return o
}

func (o Order) Kind() string {
	return "order2"
}

func (o *Order) Tally() {
	subtotal := 0
	for _, item := range o.Items {
		subtotal += item.Quantity * int(item.Price)
	}
	o.LineTotal = Cents(subtotal)

	// TODO: Make this use shipping/tax information
	shipping := 0
	tax := 0
	total := tax + shipping + subtotal

	o.Shipping = Cents(shipping)
	o.Tax = Cents(tax)
	o.Subtotal = Cents(subtotal)
	o.Total = Cents(total)
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

// func (o Order) Description() string {
// 	buffer := bytes.NewBufferString("")

// 	for i, item := range o.Items {
// 		if i > 0 {
// 			buffer.WriteString(", ")
// 		}
// 		buffer.WriteString(item.SKU())
// 		buffer.WriteString(" x")
// 		buffer.WriteString(strconv.Itoa(item.Quantity))
// 	}
// 	return buffer.String()
// }

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
