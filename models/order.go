package models

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	stripe "crowdstart.io/thirdparty/stripe/models"
)

type Order struct {
	FieldMapMixin
	// Account         PaymentAccount
	BillingAddress  Address
	ShippingAddress Address
	CreatedAt       time.Time `schema:"-"`
	UpdatedAt       time.Time `schema:"-"`
	Id              string
	UserId          string

	// TODO: Recalculate Shipping/Tax on server
	Shipping int64
	Tax      int64
	Subtotal int64 `schema:"-"`
	Total    int64 `schema:"-"`

	Items []LineItem

	// Slices in order to record failed tokens/charges
	StripeTokens []string `schema:"-"`
	Charges      []Charge `schema:"-"`

	// Need to save campaign id
	CampaignId string

	Preorder  bool
	Cancelled bool
	Shipped   bool
	// Refunded  bool
	// ShippingOption  ShippingOption

	Test bool
}

var variantsMap map[string]ProductVariant
var productsMap map[string]Product

func (o Order) Disputes(disputedCharges []Charge, disputed bool) {
	for _, charge := range o.Charges {
		if charge.Dispute.Status != "" {
			disputedCharges = append(disputedCharges, charge)
		}
	}
	disputed = len(disputedCharges) > 0
}

func (o *Order) LoadVariantsProducts(c *gin.Context) {
	if variantsMap == nil || productsMap == nil {
		db := datastore.New(c)

		variantsMap = make(map[string]ProductVariant)
		var variants []ProductVariant
		db.Query("variant").GetAll(db.Context, &variants)
		for _, variant := range variants {
			variantsMap[variant.SKU] = variant
		}

		productsMap = make(map[string]Product)
		var products []Product
		db.Query("product").GetAll(db.Context, &products)
		for _, product := range products {
			productsMap[product.Slug] = product
		}
	}

	for i, item := range o.Items {
		o.Items[i].Product = productsMap[item.Slug_]
		o.Items[i].Variant = variantsMap[item.SKU_]
	}
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

func (o Order) Description() string {
	buffer := bytes.NewBufferString("")

	for i, item := range o.Items {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(item.SKU())
		buffer.WriteString(" x")
		buffer.WriteString(strconv.Itoa(item.Quantity))
	}
	return buffer.String()
}

// Use binding to validate that there are no errors
func (o Order) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if len(o.Items) == 0 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Items"},
			Classification: "InputError",
			Message:        "Order has no items.",
		})
	} else {
		for _, v := range o.Items {
			errs = v.Validate(req, errs)
		}
	}

	return errs
}

// Repopulate order with data from database, variant options, etc., and
// recalculate totals.
func (o *Order) Populate(db *datastore.Datastore) error {
	// TODO: Optimize this, multiget, use caching.
	for i, item := range o.Items {
		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", item.SKU(), &item.Variant); err != nil {
			return err
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", item.Slug(), &item.Product); err != nil {
			return err
		}

		// Set SKU so we can deserialize later
		item.SKU_ = item.SKU()
		item.Slug_ = item.Slug()

		// Update item in order
		o.Items[i] = item

		// Update subtotal
		o.Subtotal += item.Price()
	}

	// Update grand total
	o.Total = o.Subtotal + o.Tax + o.Shipping
	return nil
}

type PaymentAccount struct {
	CVV2   int
	Month  int
	Year   int
	Expiry string
	Number string
	Type   string `schema:"-"`
}

type Charge struct {
	ID             string
	Captured       bool
	Created        int64
	Desc           string
	Email          string
	FailCode       string
	FailMsg        string
	FailType       string
	Live           bool
	Paid           bool
	Refunded       bool
	Statement      string
	Amount         int64
	AmountRefunded int64
	Dispute        stripe.Dispute
}

type ShippingOption struct {
	Name  string
	Price int64
}

func (so ShippingOption) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if so.Name == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Name"},
			Classification: "InputError",
			Message:        "Shipping option has no name.",
		})
	}
	return errs
}
