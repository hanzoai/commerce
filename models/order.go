package models

import (
	"bytes"
	"net/http"
	"time"

	"github.com/mholt/binding"

	"crowdstart.io/datastore"
	"crowdstart.io/util/log"
)

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
		log.Debug("Fetching variant for:")
		log.Dump(item.Variant)

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
	o.Total = o.Subtotal + o.Tax
	return nil
}
