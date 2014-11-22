package models

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"

	"crowdstart.io/datastore"
)

type LineItem struct {
	FieldMapMixin
	Product      Product
	Variant      ProductVariant
	Description  string `schema:"-"`
	DiscountAmnt int64  `schema:"-"`
	LineNo       int    `schema:"-"`
	Quantity     int
	UOM          string `schema:"-"`
	// Material     string
	// NetAmnt      string
	// TaxAmnt      string
	// UPC          string
}

func (li LineItem) Price() int64 {
	return li.Variant.Price * int64(li.Quantity)
}

func (li LineItem) DisplayPrice() string {
	return DisplayPrice(li.Price())
}

func (li LineItem) SKU() string {
	return li.Variant.SKU
}

func (li LineItem) Slug() string {
	return li.Product.Slug
}

func (li LineItem) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	if li.SKU() == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Variant.SKU"},
			Classification: "InputError",
			Message:        "SKU cannot be empty.",
		})
	}

	if li.Quantity < 1 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Quantity"},
			Classification: "InputError",
			Message:        "Quantity cannot be less than 1.",
		})
	}

	return errs
}

type Cart struct {
	FieldMapMixin
	Id        string `schema:"-"`
	Items     []LineItem
	CreatedAt time.Time `schema:"-"`
}

func (c Cart) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	// Cart cannot be empty.
	if len(c.Items) == 0 {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Items"},
			Classification: "InputError",
			Message:        "Cart is empty.",
		})
	} else {
		for _, v := range c.Items {
			errs = v.Validate(req, errs)
		}
	}
	return errs
}

type PaymentAccount struct {
	CVV2   int
	Month  int
	Year   int
	Expiry string
	Number string
	Type   string `schema:"-"`
}

type Order struct {
	FieldMapMixin
	Account         PaymentAccount
	BillingAddress  Address
	CreatedAt       time.Time `schema:"-"`
	Id              string    `schema:"-"`
	Shipping        int64     `schema:"-"`
	ShippingAddress Address
	Subtotal        int64 `schema:"-"`
	Tax             int64 `schema:"-"`
	Total           int64 `schema:"-"`

	ItemIds []string
	Items   []LineItem `datastore:"-"`

	StripeToken string `schema:"-"`
	Campaign    Campaign

	Cancelled bool // represents whether the order has been cancelled
	Shipped   bool
	// ShippingOption  ShippingOption
}

func (o *Order) Save(c *gin.Context) error {
	return o.saveItems(c)
}

// TODO Run this in a transaction
func (o *Order) saveItems(c *gin.Context) error {
	o.ItemIds = make([]string, len(o.Items))
	genItems := make([]interface{}, len(o.Items))
	for i, item := range o.Items {
		err := item.Product.Save(c)
		if err != nil {
			return err
		}
		genItems[i] = interface{}(item)
	}

	db := datastore.New(c)
	keys, err := db.PutMulti("variant", genItems)
	o.ItemIds = keys

	return err
}

func (o *Order) Load(c *gin.Context) error {
	return o.loadItems(c)
}

func (o *Order) loadItems(c *gin.Context) error {
	db := datastore.New(c)
	genItems := make([]interface{}, len(o.ItemIds))
	err := db.GetKeyMulti("line-item", o.ItemIds, genItems)

	if err != nil {
		return err
	}

	o.Items = make([]LineItem, len(genItems))
	for i, item := range genItems {
		o.Items[i] = item.(LineItem)
	}

	return err
}

func (o Order) DisplaySubtotal() string {
	return DisplayPrice(o.Subtotal)
}

func (o Order) Description() string {
	buffer := bytes.NewBufferString("")

	for _, i := range o.Items {
		buffer.WriteString(i.Description)
		buffer.WriteString(" ")
		buffer.WriteString(string(i.Quantity))
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func (o Order) DisplayTax() string {
	return DisplayPrice(o.Tax)
}

func (o Order) DisplayTotal() string {
	return DisplayPrice(o.Total)
}

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
