package models

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

type LineItem struct {
	FieldMapMixin
	SKU_         string         `json:"SKU"`
	Slug_        string         `json:"Slug"`
	Product      Product        `datastore:"-"`
	Variant      ProductVariant `datastore:"-"`
	Description  string
	DiscountAmnt int64
	LineNo       int
	Quantity     int

	// TODO: Deprecated UOM but unable to remove yet
	UOM string `schema:"-"`
	// UPC          string
	// Material     string
	// NetAmnt      string
	// TaxAmnt      string
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

type Charge struct {
	ID             string
	Live           bool
	Paid           bool
	Desc           string
	Email          string
	Amount         uint64
	FailMsg        string
	Created        int64
	Refunded       bool
	Captured       bool
	FailCode       string
	Statement      string
	AmountRefunded uint64
}

type Order struct {
	FieldMapMixin
	Account         PaymentAccount
	BillingAddress  Address
	ShippingAddress Address
	CreatedAt       time.Time `schema:"-"`
	UpdatedAt       time.Time `schema:"-"`
	Id              string    `schema:"-"`
	Shipping        int64     `schema:"-"`
	Subtotal        int64     `schema:"-"`
	Tax             int64     `schema:"-"`
	Total           int64     `schema:"-"`

	Items []LineItem

	// Slices in order to record failed tokens/charges
	StripeTokens []string `schema:"-"`
	Charges      []Charge `schema:"-"`

	// Need to save campaign id
	CampaignId string

	Cancelled bool
	Shipped   bool
	// ShippingOption  ShippingOption

	// TODO: Deprecate and remove from existing data so we can update production
	Campaign    Campaign
	StripeToken string
}

func (order *Order) Process(c *gin.Context) error {
	db := datastore.New(c)
	for i, lineItem := range order.Items {
		// Fetch Variant for LineItem from datastore
		if err := db.GetKey("variant", lineItem.SKU(), &lineItem.Variant); err != nil {
			log.Error(err.Error())
			return err
		}

		// Fetch Product for LineItem from datastore
		if err := db.GetKey("product", lineItem.Slug(), &lineItem.Product); err != nil {
			log.Error(err.Error())
			return err
		}

		order.Items[i] = lineItem
		order.Subtotal += lineItem.Price()
	}

	order.Total = order.Subtotal + order.Tax
	return nil
}

// func (o *Order) Save(c *gin.Context) error {
// 	return o.saveItems(c)
// }

// // TODO Run this in a transaction
// func (o *Order) saveItems(c *gin.Context) error {
// 	o.ItemIds = make([]string, len(o.Items))
// 	genItems := make([]interface{}, len(o.Items))
// 	for i, item := range o.Items {
// 		err := item.Product.Save(c)
// 		if err != nil {
// 			return err
// 		}
// 		genItems[i] = interface{}(item)
// 	}

// 	db := datastore.New(c)
// 	keys, err := db.PutMulti("variant", genItems)
// 	o.ItemIds = keys

// 	return err
// }

// func (o *Order) Load(c *gin.Context) error {
// 	return o.loadItems(c)
// }

// func (o *Order) loadItems(c *gin.Context) error {
// 	db := datastore.New(c)
// 	genItems := make([]interface{}, len(o.ItemIds))
// 	err := db.GetKeyMulti("line-item", o.ItemIds, genItems)

// 	if err != nil {
// 		return err
// 	}

// 	o.Items = make([]LineItem, len(genItems))
// 	for i, item := range genItems {
// 		o.Items[i] = item.(LineItem)
// 		err = o.Items[i].Product.Load(c)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return err
// }

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
