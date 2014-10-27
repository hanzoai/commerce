package cardconnect

import (
	"crowdstart.io/models"
	"encoding/base64"
	"errors"
	"github.com/jmcvetta/napping"
	"net/http"
)

var baseUrl = "fts.prinpay.com:6443/cardconnect/rest" // 496160873888-CardConnect - USD - NORTH
var authCode = base64.StdEncoding.EncodeToString([]byte("testing:testing123"))

type LineItem struct {
	SKU          string // {
	Cost         int64  `json:"unitcost,string"` // "unitcost":    "450",
	Description  string `json:"description"`     // "description": "DESCRIPTION-2",
	DiscountAmnt int64  `json:"discamnt,string"` // "discamnt":    "0",
	LineNo       int    `json:"lineno,string"`   // "lineno":      "2",
	Quantity     int    `json:"quantity,string"` // "quantity":    "2000",
	UOM          string `json:"uom"`             // "uom":         "EA",
	// Material     string `json:"material"`		 // "material":    "MATERIAL-2"
	// NetAmnt      string `json:"netamnt"`         // "netamnt":     "300",
	// TaxAmnt      string `json:"taxamnt"`         // "taxamnt":     "117",
	// UPC          string `json:"upc"`             // "upc":	      "UPC-1",
}												 //  }

type AuthorizationRequest struct { // {
	Account  string      `json:"account"`  // "account":  "4111111111111111",
	AcctType string      `json:"accttype"` // "accttype": "VISA",
	Address  string      `json:"address"`  // "address":  "123 MAIN STREET",
	Amount   string      `json:"amount"`   // "amount":   "0",
	City     string      `json:"city"`     // "city":     "anytown",
	Country  string      `json:"country"`  // "country":  "US",
	Currency string      `json:"currency"` // "currency": "USD",
	CVV2     string      `json:"cvv2"`     // "cvv2":     "123",
	Ecomind  string      `json:"ecomind"`  // "ecomind":  "E",
	Expiry   string      `json:"expiry"`   // "expiry":   "1212",
	MerchId  string      `json:"merchid"`  // "merchid":  "000000927996",
	Name     string      `json:"name"`     // "name":     "TOM JONES",
	OrderId  string      `json:"orderid"`  // "orderid":  "AB-11-9876",
	Postal   string      `json:"postal"`   // "postal":   "55555",
	Region   string      `json:"region"`   // "region":   "NY",
	Tokenize string      `json:"tokenize"` // "tokenize": "Y",
	Track    interface{} `json:"track"`    // "track":    null,

	// Capture Request (embed to autocapture)
	Capture string     `json:"capture"` // "capture":  "Y",
	Items   []LineItem `json:"items"`   // "items": [],

	// 3D Secure (optional)
	SecureFlag  string `json:"secureflag"`
	SecureValue string `json:"securevalue"`
	SecureXid   string `json:"securexid"`
} // }

type AuthorizationResponse struct { // {
	Account  string `json:"account"`  // "account":  "41XXXXXXXXXX1111",
	Amount   string `json:"amount"`   // "amount":   "111",
	AuthCode string `json:"authcode"` // "authcode": "046221",
	AvsResp  string `json:"avsresp"`  // "avsresp":  "9",
	CvvResp  string `json:"cvvresp"`  // "cvvresp":  "M",
	MerchId  string `json:"merchid"`  // "merchid":  "020594000000",
	Code     string `json:"respcode"` // "respcode": "00",
	RespProc string `json:"respproc"` // "respproc": "FNOR"
	Status   string `json:"respstat"` // "respstat": "A",
	Text     string `json:"resptext"` // "resptext": "Approved",
	RetRef   string `json:"retref"`   // "retref":   "343005123105",
	Token    string `json:"token"`    // "token":    "9419786452781111",
} // }

func Authorize(order models.Order) (AuthorizationRespons, error) {
	areq := AuthorizationRequest{
		Account:  order.Account.Number,
		AcctType: order.Account.Type,
		Address:  order.BillingAddress.Unit + " " + order.BillingAddress.Street,
		Amount:   order.Total,
		CVV2:     order.Account.CVV2,
		City:     order.BillingAddress.City,
		Country:  order.BillingAddress.Country,
		Currency: "USD",
		Ecomind:  "E",
		Email:    order.User.Email,
		Expiry:   order.Account.Expiry,
		MerchId:  "496160873888",
		Name:     order.User.Name,
		OrderId:  order.Id,
		Phone:    order.User.Phone,
		Postal:   order.BillingAddress.PostalCode,
		Region:   order.BillingAddress.State,
		Tokenize: "Y",
		Track:    null,
		Capture:  "Y",
		Items:    order.Items,
	}

	header := http.Header{}
	header.Add("Authorization", "Basic "+authCode)
	s := napping.Session{Header: &header}

	switch res, err := s.Post(baseUrl+"/auth", &areq, &ares, nil); {
	case err != nil:
		return ares, err

	case res.Status() == 200:
		return ares, nil

	default:
		return ares, errors.New("Invalid response")
	}
}
