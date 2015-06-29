package notify

import (
	"encoding/xml"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"crowdstart.com/util/log"
)

// <?xml version="1.0" encoding="utf-8"?>
// <ShipNotice>
//        <OrderNumber>ABC123</OrderNumber>
//        <CustomerCode>dev@hanzo.ai</CustomerCode>
//        <LabelCreateDate>12/8/2011 12:56 PM</LabelCreateDate>
//        <ShipDate>12/8/2011</ShipDate>
//        <Carrier>USPS</Carrier>
//        <Service>Priority Mail</Service>
//        <TrackingNumber>1Z909084330298430820</TrackingNumber>
//        <ShippingCost>4.95</ShippingCost>
// <Recipient>
//               <Name>The President</Name>
//               <Company>US Govt</Company>
//               <Address1>1600 Pennsylvania Ave</Address1>
//               <Address2></Address2>
//               <City>Washington</City>
//               <State>DC</State>
//               <PostalCode>20500</PostalCode>
//               <Country>US</Country>
//        </Recipient>
//        <Items>
//               <Item>
//                      <SKU>FD88821</SKU>
//                      <Name>My Product Name</Name>
//                      <Quantity>2</Quantity>
//               </Item>
//        </Items>
// </ShipNotice>

type Request struct {
	ShipNotice struct {
		OrderNumber     string
		CustomerCode    string
		LabelCreateDate string
		ShipDate        string
		Carrier         string
		Service         string
		TrackingNumber  string
		ShippingCost    string

		Recipient struct {
			Name       string
			Company    string
			Address1   string
			Address2   string
			City       string
			State      string
			PostalCode string
			Country    string
		}

		Items []struct {
			SKU      string
			Name     string
			Quantity string
		}
	}
}

func Post(c *gin.Context) {
	query := c.Request.URL.Query()
	action := query.Get("action")
	orderId := query.Get("order_number")
	carrier := query.Get("carrier")
	trackingNumber := query.Get("tracking_number")

	b, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Panic("Failed to read request body: %v", err, c)
	}

	req := Request{}
	xml.Unmarshal(b, &req)

	log.Debug("action: %v, orderId: %v, carrier: %v, trackingNumber: %v, xml: %v", action, orderId, carrier, trackingNumber, req, c)
}
