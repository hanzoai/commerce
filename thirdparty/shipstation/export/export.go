package export

import (
	"encoding/xml"
	"math"
	"strconv"
	"time"

	aeds "appengine/datastore"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/lineitem"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"

	. "crowdstart.com/models"
)

// <?xml version="1.0" encoding="utf-8"?>
// <Orders>
// 	<Order>
// 		<OrderID><![CDATA[123456]]></OrderID>
// 		<OrderNumber><![CDATA[ABC123]]></OrderNumber>
// 		<OrderDate>12/8/2011 21:56 PM</OrderDate>
// 		<OrderStatus><![CDATA[AwaitingShipment]]></OrderStatus>
// 		<LastModified>12/8/2011 12:56 PM</LastModified>
// 		<ShippingMethod><![CDATA[USPSPriorityMail]]></ShippingMethod>
// 		<PaymentMethod><![CDATA[Credit Card]]></PaymentMethod>
// 		<OrderTotal>123.45</OrderTotal>
// 		<TaxAmount>0.00</TaxAmount>
// 		<ShippingAmount>4.50</ShippingAmount>
// 		<CustomerNotes><![CDATA[Please make sure it gets here by Dec. 22nd!]]></CustomerNotes>
// 		<InternalNotes><![CDATA[Ship by December 18th via Priority Mail.]]></InternalNotes>
// 		<Customer>
// 			<CustomerCode><![CDATA[dev@hanzo.ai]]></CustomerCode>
// 			<BillTo>
// 				<Name><![CDATA[The President]]></Name>
// 				<Company><![CDATA[US Govt]]></Company>
// 				<Phone><![CDATA[512-555-5555]]></Phone>
// 				<Email><![CDATA[dev@hanzo.ai]]></Email>
// 			</BillTo>
// 			<ShipTo>
// 				<Name><![CDATA[The President]]></Name>
// 				<Company><![CDATA[US Govt]]></Company>
// 				<Address1><![CDATA[1600 Pennsylvania Ave]]></Address1>
// 				<Address2></Address2>
// 				<City><![CDATA[Washington]]></City>
// 				<State><![CDATA[DC]]></State>
// 				<PostalCode><![CDATA[20500]]></PostalCode>
// 				<Country><![CDATA[US]]></Country>
// 				<Phone><![CDATA[512-555-5555]]></Phone>
// 			</ShipTo>
// 		</Customer>
// 		<Items>
// 			<Item>
// 				<SKU><![CDATA[FD88821]]></SKU>
// 				<Name><![CDATA[My Product Name]]></Name>
// 				<ImageUrl><![CDATA[http://www.mystore.com/products/12345.jpg]]></ImageUrl>
// 				<Weight>8</Weight>
// 				<WeightUnits>Ounces</WeightUnits>
// 				<Quantity>2</Quantity>
// 				<UnitPrice>13.99</UnitPrice>
// 				<Location><![CDATA[A1-B2]]></Location>
// 				<Options>
// 					<Option>
// 						<Name><![CDATA[Size]]></Name>
// 						<Value><![CDATA[Large]]></Value>
// 						<Weight>10</Weight>
// 					</Option>
// 					<Option>
// 						<Name><![CDATA[Color]]></Name>
// 						<Value><![CDATA[Green]]></Value>
// 						<Weight>5</Weight>
// 					</Option>
// 				</Options>
// 			</Item>
// 		</Items>
// 	</Order>
// </Orders>

func parseDate(s string) time.Time {
	date, err := time.Parse("01/02/2006 15:04", s)
	if err != nil {
		log.Panic("Unable to parse start date: %v", err)
	}
	return date
}

func renderDate(date Date) string {
	return time.Time(date).Format("01/02/2006 15:04")
}

type CDATA string

func (c CDATA) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(struct {
		S string `xml:",innerxml"`
	}{
		S: "<![CDATA[" + string(c) + "]]>",
	}, start)
}

type Date time.Time

func (d Date) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(struct {
		S string `xml:",innerxml"`
	}{
		S: renderDate(d),
	}, start)
}

type Option struct {
	XMLName xml.Name `xml:"Option"`

	Name   CDATA
	Value  CDATA
	Weight string
}

type Item struct {
	XMLName xml.Name `xml:"Item"`

	SKU         CDATA
	Name        CDATA
	ImageUrl    CDATA
	Weight      string
	WeightUnits string
	Quantity    int
	UnitPrice   string
	Location    CDATA

	Options struct {
		Options []Option
	}
}

func newItem(item lineitem.LineItem) Item {
	si := Item{}
	si.SKU = CDATA(item.ProductSlug)
	si.Name = CDATA(item.ProductName)

	if item.VariantSKU != "" {
		si.SKU = CDATA(item.VariantSKU)
	}

	if item.VariantName != "" {
		si.SKU = CDATA(item.VariantName)
	}

	si.UnitPrice = item.DisplayPrice()
	si.Quantity = item.Quantity
	si.Weight = item.Weight.String()
	si.WeightUnits = string(item.WeightUnit)

	return si
}

type Customer struct {
	CustomerCode CDATA
	BillTo       struct {
		Name    CDATA
		Company CDATA
		Phone   CDATA
		Email   CDATA
	}
	ShipTo struct {
		Name       CDATA
		Company    CDATA
		Address1   CDATA
		Address2   CDATA
		City       CDATA
		State      CDATA
		PostalCode CDATA
		Country    CDATA
		Phone      CDATA
	}
}

func newCustomer(ord *order.Order, usr *user.User) *Customer {
	sc := &Customer{}

	if usr == nil {
		return sc
	}

	sc.CustomerCode = CDATA(usr.Id())
	sc.BillTo.Name = CDATA(usr.Name())
	sc.BillTo.Email = CDATA(usr.Email)
	sc.BillTo.Phone = CDATA(usr.Phone)

	sc.ShipTo.Name = CDATA(usr.Name())
	sc.ShipTo.Phone = CDATA(usr.Phone)
	sc.ShipTo.Address1 = CDATA(ord.ShippingAddress.Line1)
	sc.ShipTo.Address2 = CDATA(ord.ShippingAddress.Line2)
	sc.ShipTo.City = CDATA(ord.ShippingAddress.City)
	sc.ShipTo.State = CDATA(ord.ShippingAddress.State)
	sc.ShipTo.PostalCode = CDATA(ord.ShippingAddress.PostalCode)
	sc.ShipTo.Country = CDATA(ord.ShippingAddress.Country)

	return sc
}

type Order struct {
	XMLName        xml.Name `xml:"Order"`
	OrderID        CDATA
	OrderNumber    int
	OrderDate      Date
	OrderStatus    CDATA
	LastModified   Date
	ShippingMethod CDATA
	PaymentMethod  CDATA
	OrderTotal     string
	TaxAmount      string
	ShippingAmount string
	CustomerNotes  CDATA
	InternalNotes  CDATA

	// Need to nest items slice so we can have a proper XML node here
	Items struct {
		Items []Item
	}

	Customer *Customer
}

func newOrder(ord *order.Order) *Order {
	so := &Order{}
	so.OrderID = CDATA(ord.Id())
	so.OrderNumber = ord.Number
	so.OrderDate = Date(ord.CreatedAt)
	so.LastModified = Date(ord.UpdatedAt)
	so.OrderTotal = ord.DisplayTotal()
	so.TaxAmount = ord.DisplayTax()
	so.ShippingAmount = ord.DisplayShipping()
	so.Items.Items = make([]Item, len(ord.Items))
	for i, item := range ord.Items {
		so.Items.Items[i] = newItem(item)
	}

	// Try to figure out order status
	if ord.Status.PaymentStatus == payment.Unpaid {
		so.OrderStatus = CDATA(payment.Unpaid)
	}

	if ord.Status.PaymentStatus == payment.Paid {
		so.OrderStatus = CDATA(payment.Paid)
	}

	if ord.Status.FulfillmentStatus = FulfillmentShipped {
		so.OrderStatus = CDATA(FulfillmentShipped)
	}

	if ord.Status == order.Cancelled {
		so.OrderStatus = CDATA(order.Cancelled)
	}

	if ord.Status == order.Locked {
		so.OrderStatus = CDATA(order.Locked)
	}

	// Default to FulfillmentStatus
	if so.OrderStatus == "" {
		so.OrderStatus = CDATA(ord.FulfillmentStatus)
	}
	return so
}

type Response struct {
	XMLName xml.Name `xml:"Orders"`
	Orders  []*Order
	Pages   int `xml:"pages,attr"`
}

func Export(c *gin.Context) {
	query := c.Request.URL.Query()

	limit := 100
	offset := 0

	// Only support export action
	action := query.Get("action")
	if action != "export" {
		log.Panic("Invalid action %s, only understand 'export'", action, c)
	}

	// Parse offset
	page, err := strconv.Atoi(query.Get("page"))
	if err != nil && page > 1 {
		offset = limit * (page - 1)
	}

	// Get start/end dates
	startDate := parseDate(query.Get("start_date"))
	endDate := parseDate(query.Get("end_date"))

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	// Query out relevant orders
	q := order.Query(db).Order("CreatedAt").
		Filter("CreatedAt >=", startDate).
		Filter("CreatedAt <", endDate)

	// Calculate total pages
	count, _ := q.Count()
	pages := int(math.Ceil(float64(count) / float64(100)))

	// Get current page of orders
	orders := make([]*order.Order, 0, 0)
	_, err = q.Limit(limit).Offset(offset).GetAll(&orders)
	if err != nil {
		log.Panic("Unable to fetch orders between %s and %s, page %s: %v", startDate, endDate, page, err, c)
	}

	numOrders := len(orders)

	// Build XML response
	res := &Response{}
	res.Pages = pages
	res.Orders = make([]*Order, numOrders)

	ctx := db.Context
	keys := make([]*aeds.Key, numOrders)

	// Fetch orders
	for i, ord := range orders {
		// Store order
		res.Orders[i] = newOrder(ord)
		// Save user key for later
		keys[i], _ = hashid.DecodeKey(ctx, ord.UserId)
	}

	// Fetch users
	users := make([]*user.User, numOrders)
	if err := aeds.GetMulti(ctx, keys, users); err != nil {
		log.Warn("Unable to fetch all users using keys %v: %v", keys, err, c)
		log.Warn("Found users: %v", users, c)
	}

	// Set customers
	for i, ord := range orders {
		res.Orders[i].Customer = newCustomer(ord, users[i])
	}

	buf, _ := xml.MarshalIndent(res, "", "  ")
	buf = append([]byte(xml.Header), buf...)
	c.Data(200, "text/xml", buf)
}
