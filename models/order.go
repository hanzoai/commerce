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
	"crowdstart.io/util/log"
)

type Order struct {
	FieldMapMixin
	SalesforceSObject

	// Account         PaymentAccount

	BillingAddress  Address
	ShippingAddress Address
	CreatedAt       time.Time `schema:"-"`
	UpdatedAt       time.Time `schema:"-"`
	Id              string
	UserId          string
	Email           string

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

	// Basic status flags for order
	Cancelled   bool `schema:"-"`
	Locked      bool `schema:"-"'`
	Preorder    bool `schema:"-"`
	Refunded    bool `schema:"-"`
	Shipped     bool `schema:"-"`
	Unconfirmed bool `schema:"-"` // True only if preorder has not be confirmed by customer

	// Dispute details
	Disputed bool
	Dispute  stripe.Dispute // Refactor to use []stripe.Dispute for multiple charges.

	// ShippingOption  ShippingOption

	Test    bool // Not a real transaction
	Version int  // Versioning for struct
}

var variantsMap map[string]ProductVariant
var productsMap map[string]Product

func (o Order) DisputedCharges(c *gin.Context) (disputedCharges []Charge) {
	for _, charge := range o.Charges {
		if charge.Disputed {
			disputedCharges = append(disputedCharges, charge)
		}
	}
	return disputedCharges
}

func (o *Order) LoadVariantsProducts(c interface{}) {
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
		log.Warn("SKU %v, %v", item.SKU_, variantsMap[item.SKU_])
		o.Items[i].Product = productsMap[item.Slug_]
		o.Items[i].Variant = variantsMap[item.SKU_]
		o.Items[i].VariantId = variantsMap[item.SKU_].Id
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
		if err := db.GetKind("variant", item.SKU(), &item.Variant); err != nil {
			return err
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKind("product", item.Slug(), &item.Product); err != nil {
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
	Disputed       bool
}

type Dispute stripe.Dispute

func (charge Charge) Disputes(c *gin.Context) (disputes []Dispute, err error) {
	db := datastore.New(c)
	_, err = db.Query("dispute").
		Filter("Charge =", charge.ID).
		Order("Created").
		GetAll(db.Context, &disputes)
	return disputes, err
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
