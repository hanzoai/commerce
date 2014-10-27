package cardconnect

import (
	"encoding/base64"
	"errors"
	"github.com/jmcvetta/napping"
	"github.com/mholt/binding"
	"net/http"
	"crowdstart.io/models"
)

type AuthorizationRequest struct { // {
	MerchId  string      `json:"merchid"`  //   "merchid": "000000927996",
	AcctType string      `json:"accttype"` //   "accttype": "VISA",
	OrderId  string      `json:"orderid"`  //   "orderid": "AB-11-9876",
	Account  string      `json:"account"`  //   "account": "4111111111111111",
	Expiry   string      `json:"expiry"`   //   "expiry": "1212",
	Amount   string      `json:"amount"`   //   "amount": "0",
	Currency string      `json:"currency"` //   "currency": "USD",
	Name     string      `json:"name"`     //   "name": "TOM JONES",
	Address  string      `json:"address"`  //   "address": "123 MAIN STREET",
	City     string      `json:"city"`     //   "city": "anytown",
	Region   string      `json:"region"`   //   "region": "NY",
	Country  string      `json:"country"`  //   "country": "US",
	Postal   string      `json:"postal"`   //   "postal": "55555",
	Ecomind  string      `json:"ecomind"`  //   "ecomind": "E",
	Cvv2     string      `json:"cvv2"`     //   "cvv2": "123",
	Track    interface{} `json:"track"`    //   "track": null,
	Tokenize string      `json:"tokenize"` //   "tokenize": "Y"
} // }

func New(order models.Order, cardType string, account string, expiry string, amount models.Currency) (areq AuthorizationRequest) {
	areq.MerchId = "496160873888"
	areq.OrderId = order.OrderId
	areq.AcctType = cardType
	areq.Name = order.User.Name
	areq.Address = order.BillingAddress.Unit + " " + order.BillingAddress.Street
	areq.City = order.BillingAddress.City
	areq.Region =  order.State
	areq.Postal = order.PostalCode
	areq.Country = order.Country
	areq.Account = account
	areq.Expiry = expiry
	areq.Amount = amount.String()
	areq.Currency = "USD" // not necessary for USD
	return areq
}


type AuthorizationResponse struct { // {
	RespStat string `json:"respstat"` //   "respstat": "A",
	Account  string `json:"account"`  //   "account": "41XXXXXXXXXX1111",
	Token    string `json:"token"`    //   "token": "9419786452781111",
	RetRef   string `json:"retref"`   //   "retref": "343005123105",
	Amount   string `json:"amount"`   //   "amount": "111",
	MerchId  string `json:"merchid"`  //   "merchid": "020594000000",
	RespCode string `json:"respcode"` //   "respcode": "00",
	RespText string `json:"resptext"` //   "resptext": "Approved",
	AvsResp  string `json:"avsresp"`  //   "avsresp": "9",
	CvvResp  string `json:"cvvresp"`  //   "cvvresp": "M",
	AuthCode string `json:"authcode"` //   "authcode": "046221",
	RespProc string `json:"respproc"` //   "respproc": "FNOR"
} // }

var baseUrl = "fts.prinpay.com:6443/cardconnect/rest" // 496160873888-CardConnect - USD - NORTH
var authCode = base64.StdEncoding.EncodeToString([]byte("testing:testing123"))

func Authorize(areq AuthorizationRequest) (ares AuthorizationResponse, err error) {
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

func TranslateResponseCode(respstat string, respcode string) (code, status string) {
	switch {
	case respstat == "00" && respcode == "A":
		code = "Approval"
		status = "Approved"

	case respstat == "11" && respcode == "C":
		code = "Invalid card"
		status = "Bad Card Data"

	case respstat == "12" && respcode == "C":
		code = "Invalid track"
		status = "Bad Track Data"

	case respstat == "13" && respcode == "C":
		code = "Bad card check digit"
		status = "Failed Luhn"

	case respstat == "14" && respcode == "C":
		code = "Non-numeric CVV"
		status = "CVV not numeric"

	case respstat == "15" && respcode == "C":
		code = "Non-numeric expiry"
		status = "Expiration not numeric"

	case respstat == "16" && respcode == "C":
		code = "Card expired"
		status = "Expiration in the past"

	case respstat == "17" && respcode == "C":
		code = "Invalid zip"
		status = "US zip code not 5 or 9 digits"

	case respstat == "21" && respcode == "C":
		code = "Invalid merchant"
		status = "Merchant Id Not Found"

	case respstat == "22" && respcode == "C":
		code = "No auth route"
		status = "CardConnect configuration error"

	case respstat == "23" && respcode == "B":
		code = "No auth queue"
		status = "Retry [CardConnect error]"

	case respstat == "24" && respcode == "C":
		code = "Reversal not supported"
		status = "Cannot Void"

	case respstat == "25" && respcode == "C":
		code = "No matching auth for reversal"
		status = "Cannot Void"

	case respstat == "26" && respcode == "A":
		code = "Txn Settled"
		status = "Already Captured"

	case respstat == "27" && respcode == "C":
		code = "Txn Batched"
		status = "Cannot Void"

	case respstat == "28" && respcode == "C":
		code = "Txn not settled"
		status = "Cannot Refund"

	case respstat == "29" && respcode == "C":
		code = "Txn not found"
		status = "Bad Retref"

	case respstat == "31" && respcode == "C":
		code = "Invalid currency"
		status = "Bad Currency"

	case respstat == "32" && respcode == "C":
		code = "Wrong currency for merch"
		status = "Bad Currency for Merchant configuration"

	case respstat == "33" && respcode == "C":
		code = "Unknown card type"
		status = "Bad card"

	case respstat == "34" && respcode == "C":
		code = "Invalid field"
		status = "Bad Data"

	case respstat == "35" && respcode == "C":
		code = "No postal code"
		status = "No Postal"

	case respstat == "36" && respcode == "C":
		code = "Duplicate sequence"
		status = "Duplicate Txn"

	case respstat == "37" && respcode == "C":
		code = "CVV mismatch"
		status = "Proc approved but CVV mismatch"

	case respstat == "41" && respcode == "C":
		code = "Below min amount"
		status = "Bad amount"

	case respstat == "42" && respcode == "C":
		code = "Above max amount"
		status = "Bad amount"

	case respstat == "43" && respcode == "C":
		code = "Invalid amount"
		status = "Bad amount"

	case respstat == "44" && respcode == "C":
		code = "Prepaid not supported"
		status = "Not configured for Prepaid BINs"

	case respstat == "61" && respcode == "B":
		code = "Line down"
		status = "Retry [connection to processor down]"

	case respstat == "62" && respcode == "B":
		code = "Timed out"
		status = "Retry [no issuer response]"

	case respstat == "63" && respcode == "C":
		code = "Bad resp format"
		status = "Error parsing issuer response"

	case respstat == "64" && respcode == "C":
		code = "Bad HTTP Header"
		status = "Error parsing issuer response"

	case respstat == "65" && respcode == "C":
		code = "Socket close error"
		status = "Network Error"

	case respstat == "66" && respcode == "C":
		code = "Response mismatch"
		status = "Network Error"

	case respstat == "91" && respcode == "B":
		code = "No TokenSecure"
		status = "Retry [CardConnect error]"

	case respstat == "92" && respcode == "C":
		code = "No Merchant table"
		status = "Bad Data"

	case respstat == "93" && respcode == "B":
		code = "No Database"
		status = "Retry [CardConnect error]"

	case respstat == "94" && respcode == "C":
		code = "No action"
		status = "Bad Data"

	case respstat == "95" && respcode == "C":
		code = "Missing config"
		status = "Missing Config"

	case respstat == "96" && respcode == "C":
		code = "No Profile"
		status = "Profile Not Found"
	}

	return code, status
}
