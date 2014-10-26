package cardconnect

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/jmcvetta/napping"
	"net/http"
)

type AuthorizationRequest struct {
	MerchId  string
	AcctType string
	OrderId  string
	Account  string
	Expiry   string
	Amount   string
	Currency string
	Name     string
	Address  string
	City     string
	Region   string
	Country  string
	Postal   string
	Ecomind  string
	CVV2     string // confirmation code
	Track    string
	Tokenize string
}

type AuthorizationResponse struct {
	RespStat string
	Account  string
	Token    string
	RetRef   string
	Amount   string
	MerchId  string
	RespCode string
	RespText string

	// If successful
	AVSResp  string
	CVVResp  string
	AuthCode string
	CommCard string
}

var baseUrl = "fts.prinpay.com:6443/cardconnect/rest" // 496160873888-CardConnect - USD - NORTH
var authCode = base64.StdEncoding.EncodeToString([]byte("testing:testing123"))

func Authorize(areq AuthorizationRequest) (ares AuthorizationResponse, err error) {
	s := napping.Session{
		Header: *http.Header{
			"Authorization": "Basic " + authCode,
		},
	}

	switch res, err := s.Post(baseUrl+"/auth", &areq, &ares); {
	case err != nil:
		return ares, err
	case res.Status() == 200:
		return ares, nil
	default:
		return ares, errors.New("CardConnect returned invalid response")
	}
}
