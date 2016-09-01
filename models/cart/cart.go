package cart

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/dustin/go-humanize"

	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
	. "crowdstart.com/models/lineitem"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Status string

const (
	Active    Status = "active"
	Discarded        = "discarded"
	Ordered          = "ordered"
)

type Cart struct {
	mixin.Model

	// Store this was sold from (if any)
	StoreId string `json:"storeId,omitempty"`

	// Associated campaign
	CampaignId string `json:"campaignId,omitempty"`

	// Associated Crowdstart user or buyer.
	UserId string `json:"userId,omitempty"`

	// Email of the user or someone else if no user id exists
	Email string `json:"email,omitempty"`

	// Associated order ID, if any
	OrderId string `json:"orderId,omitempty"`

	// Status
	Status Status `json:"status"`

	// 3-letter ISO currency code (lowercase).
	Currency currency.Type `json:"currency"`

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

	// Total = subtotal + shipping + taxes + adjustments. Amount in cents.
	Total currency.Cents `json:"total"`

	Company string `json:"company,omitempty"`

	BillingAddress  Address `json:"billingAddress,omitempty"`
	ShippingAddress Address `json:"shippingAddress,omitempty"`

	// Individual line items
	Items  []LineItem `json:"items" datastore:"-"`
	Items_ string     `json:"-"` // need props

	Coupons     []coupon.Coupon `json:"coupons,omitempty"`
	CouponCodes []string        `json:"couponCodes,omitempty"`
	ReferrerId  string          `json:"referrerId,omitempty"`

	// Series of events that have occured relevant to this order
	History []Event `json:"-,omitempty"`

	// Arbitrary key/value pairs associated with this order
	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`

	Gift        bool   `json:"gift"`        // Is this a gift?
	GiftMessage string `json:"giftMessage"` // Message to go on gift
	GiftEmail   string `json:"giftEmail"`   // Email for digital gifts

	Mailchimp struct {
		Id          string `json:"id,omitempty"`
		CampaignId  string `json:"campaignId,omitempty"`
		CheckoutUrl string `json:"checkoutUrl,omitempty"`
	} `json:"mailchimp,omitempty"`
}

func (c *Cart) Validator() *val.Validator {
	return val.New()
}

func (c *Cart) Load(ch <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	c.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(c, ch)); err != nil {
		return err
	}

	for _, coup := range c.Coupons {
		coup.Init(c.Model.Db)
	}

	// Deserialize from datastore
	if len(c.Items_) > 0 {
		err = json.DecodeBytes([]byte(c.Items_), &c.Items)
	}

	if len(c.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(c.Metadata_), &c.Metadata)
	}

	return err
}

func (c *Cart) Save(ch chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	c.Metadata_ = string(json.EncodeBytes(&c.Metadata))
	c.Items_ = string(json.EncodeBytes(c.Items))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(c, ch))
}

func (c *Cart) SetItem(db *datastore.Datastore, id string, typ string, quantity int) (err error) {
	// Remove item from cart
	if quantity == 0 {
		c.RemoveItem(id)
		return nil
	}

	// Update quantity of existing item
	for i, li := range c.Items {
		if li.HasId(id) {
			c.Items[i].Quantity = quantity
			return nil
		}
	}

	// New item
	li := &LineItem{}
	switch typ {
	case "product":
		err = li.SetProduct(db, id, quantity)
	case "variant":
		err = li.SetVariant(db, id, quantity)
	}

	if err != nil {
		return err
	}

	c.Items = append(c.Items, *li)
	return nil

}

func (c *Cart) RemoveItem(id string) (err error) {
	items := make([]LineItem, 0)
	for _, li := range c.Items {
		if !li.HasId(id) {
			items = append(items, li)
		}
	}
	c.Items = items
	return nil
}

func (c *Cart) SetProduct(db *datastore.Datastore, id string, quantity int) (err error) {
	return c.SetItem(db, id, "product", quantity)
}

func (c *Cart) SetVariant(db *datastore.Datastore, id string, quantity int) (err error) {
	return c.SetItem(db, id, "variant", quantity)
}

func (c Cart) ItemsJSON() string {
	return json.Encode(c.Items)
}

func (c Cart) IntId() int {
	return int(c.Key().IntID())
}

func (c Cart) DisplayId() string {
	return strconv.Itoa(c.IntId())
}

func (c Cart) DisplayCreatedAt() string {
	duration := time.Since(c.CreatedAt)

	if duration.Hours() > 24 {
		year, month, day := c.CreatedAt.Date()
		return fmt.Sprintf("%s %s, %s", month.String(), strconv.Itoa(day), strconv.Itoa(year))
	}

	return humanize.Time(c.CreatedAt)
}

func (c Cart) DisplaySubtotal() string {
	return DisplayPrice(c.Currency, c.Subtotal)
}

func (c Cart) DisplayDiscount() string {
	return DisplayPrice(c.Currency, c.Discount)
}

func (c Cart) DisplayTax() string {
	return DisplayPrice(c.Currency, c.Tax)
}

func (c Cart) DisplayShipping() string {
	return DisplayPrice(c.Currency, c.Shipping)
}

func (c Cart) DisplayTotal() string {
	return DisplayPrice(c.Currency, c.Total)
}

func (c Cart) Description() string {
	if c.Items == nil {
		return ""
	}

	buffer := bytes.NewBufferString("")

	for i, item := range c.Items {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(item.String())
		buffer.WriteString(" x")
		buffer.WriteString(strconv.Itoa(item.Quantity))
	}
	return buffer.String()
}
