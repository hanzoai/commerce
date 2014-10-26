package cardconnect

import (
	"encoding/base64"
	"errors"
	"github.com/jmcvetta/napping"
	"net/http"
)

type AuthorizationRequest struct {		   // {
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
}										   // }

type AuthorizationResponse struct {        // {
	RespStat string `json:"respstat"`      //   "respstat": "A",
	Account  string `json:"account"`       //   "account": "41XXXXXXXXXX1111",
	Token    string `json:"token"`         //   "token": "9419786452781111",
	RetRef   string `json:"retref"`        //   "retref": "343005123105",
	Amount   string `json:"amount"`        //   "amount": "111",
	MerchId  string `json:"merchid"`       //   "merchid": "020594000000",
	RespCode string `json:"respcode"`      //   "respcode": "00",
	RespText string `json:"resptext"`      //   "resptext": "Approved",
	AvsResp  string `json:"avsresp"`       //   "avsresp": "9",
	CvvResp  string `json:"cvvresp"`       //   "cvvresp": "M",
	AuthCode string `json:"authcode"`      //   "authcode": "046221",
	RespProc string `json:"respproc"`      //   "respproc": "FNOR"
}                                          // }


func (ares *AuthorizationResponse) Success() bool {
	var successVars = [4]string{ares.AvsResp, ares.CvvResp, ares.AuthCode, ares.RespProc}

	for _,v := range successVars {
		if v == "" {
			return false
		}
	}
	return true
}

var baseUrl = "fts.prinpay.com:6443/cardconnect/rest" // 496160873888-CardConnect - USD - NORTH
var authCode = base64.StdEncoding.EncodeToString([]byte("testing:testing123"))

func Authorize(areq AuthorizationRequest) (ares AuthorizationResponse, err error) {
	header := http.Header{}
	header.Add("Authorization", "Basic " + authCode)
	s := napping.Session{Header: &header}

	switch res, err := s.Post(baseUrl+"/auth", &areq, &ares, nil); {
	case err != nil:
		return ares, err
	case res.Status() == 200:
		return ares, nil
	default:
		return ares, errors.New("CardConnect returned invalid response")
	}
}
