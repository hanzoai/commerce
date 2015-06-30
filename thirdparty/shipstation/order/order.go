package order

import (
	"encoding/xml"

	"github.com/gin-gonic/gin"

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
	Orders  []Order
}

func Get(c *gin.Context) {
	query := c.Request.URL.Query()
	action := query.Get("action")
	startDate := query.Get("start_date")
	endDate := query.Get("end_date")
	page := query.Get("page")

	log.Debug("action: %v, startDate: %v, endDate: %v, page: %v", action, startDate, endDate, page, c)

	// Example response
	ord := Order{}
	ord.OrderID = "123456"
	ord.OrderNumber = "ABC123"
	ord.OrderDate = "12/8/2011 21:56 PM"
	ord.OrderStatus = "AwaitingShipment"
	ord.LastModified = "12/8/2011 12:56 PM"
	ord.ShippingMethod = "USPSPriorityMail"
	ord.PaymentMethod = "Credit Card"
	ord.OrderTotal = "123.45"
	ord.TaxAmount = "0.00"
	ord.ShippingAmount = "4.50"
	ord.CustomerNotes = "Please make sure it gets here by Dec. 22nd!"
	ord.InternalNotes = "Ship by December 18th via Priority Mail."

	ord.Customer.CustomerCode = "dev@hanzo.ai"

	ord.Customer.BillTo.Name = "The President"
	ord.Customer.BillTo.Company = "US Govt"
	ord.Customer.BillTo.Phone = "512-555-5555"
	ord.Customer.BillTo.Email = "dev@hanzo.ai"

	ord.Customer.ShipTo.Name = "The President"
	ord.Customer.ShipTo.Company = "US Govt"
	ord.Customer.ShipTo.Address1 = "1600 Pennsylvania Ave"
	ord.Customer.ShipTo.Address2 = ""
	ord.Customer.ShipTo.City = "Washington"
	ord.Customer.ShipTo.State = "DC"
	ord.Customer.ShipTo.Country = "US"
	ord.Customer.ShipTo.Phone = "512-555-5555"

	ord.Items.Items = make([]Item, 1, 1)
	ord.Items.Items[0] = Item{
		SKU:         "FD88820",
		Name:        "My Product Name",
		ImageUrl:    "http://www.mystore.com/products/12345.jpg",
		Weight:      "8",
		WeightUnits: "Ounces",
		Quantity:    "2",
		UnitPrice:   "13.99",
		Location:    "A1-B2",
	}

	ord.Items.Items[0].Options.Options = []Option{
		Option{
			Name:   "Size",
			Value:  "Large",
			Weight: "10",
		},
		Option{
			Name:   "Color",
			Value:  "Green",
			Weight: "5",
		},
	}

	orders := []Order{ord}
	res, _ := xml.MarshalIndent(Response{Orders: orders}, "", "  ")
	res = append([]byte(xml.Header), res...)
	c.Data(200, "text/xml", res)
}
