package order

import (
	"encoding/xml"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/util/log"
)

type CDATA string

func (n CDATA) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return e.EncodeElement(struct {
		S string `xml:",innerxml"`
	}{
		S: "<![CDATA[" + string(n) + "]]>",
	}, start)
}

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
	Quantity    string
	UnitPrice   string
	Location    CDATA

	Options struct {
		Options []Option
	}
}

type Order struct {
	XMLName        xml.Name `xml:"Order"`
	OrderID        CDATA
	OrderNumber    CDATA
	OrderDate      string
	OrderStatus    CDATA
	LastModified   string
	ShippingMethod CDATA
	PaymentMethod  CDATA
	OrderTotal     string
	TaxAmount      string
	ShippingAmount string
	CustomerNotes  CDATA
	InternalNotes  CDATA

	Customer struct {
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

	// Need to nest items slice so we can have a proper XML node here
	Items struct {
		Items []Item
	}
}

type Response struct {
	XMLName xml.Name `xml:"Orders"`
	Orders  []*Order
}

func Get(c *gin.Context) {
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
	startDate, err := time.Parse("01/02/2006 15:04", query.Get("start_date"))
	if err != nil {
		log.Panic("Unable to parse start date: %v", err, c)
	}

	endDate, err := time.Parse("01/02/2006 15:04", query.Get("end_date"))
	if err != nil {
		log.Panic("Unable to parse end date: %v", err, c)
	}

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	// Query out relevant orders
	orders := make([]*order.Order, 0)
	q := order.Query(db).Order("CreatedAt").
		Filter("CreatedAt >=", startDate).
		Filter("CreatedAt <", endDate).
		Limit(limit).
		Offset(offset)

	count, _ := q.Count()
	log.Debug("Number of filtered orders: %v", count)

	_, err = q.GetAll(&orders)

	if err != nil {
		log.Panic("Unable to fetch orders between %s and %s, page %s: %v", startDate, endDate, page, err, c)
	}

	log.Debug("Orders: %v", orders, c)

	// Build XML response
	res := &Response{}
	res.Orders = make([]*Order, 0, 0)
	for _, ord := range orders {
		o := Order{}
		// Convert order -> shipstation order
		o.OrderID = CDATA(ord.Id())
		res.Orders = append(res.Orders, &o)
	}

	buf, _ := xml.MarshalIndent(res, "", "  ")
	buf = append([]byte(xml.Header), buf...)
	c.Data(200, "text/xml", buf)
}
