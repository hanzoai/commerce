package library

import (
	"time"

	"github.com/gin-gonic/gin"

	// "hanzo.io/datastore"
	// "hanzo.io/middleware"
	"hanzo.io/models/types/country"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

// Countries Loading
type SubDivision struct {
	Name    string `json:"name"`
	IsoCode string `json:"code"`
}

type Country struct {
	Name         string        `json:"name"`
	IsoCode      string        `json:"code"`
	SubDivisions []SubDivision `json:"subdivision"`
}

var Countries []Country
var CountryLastUpdated time.Time

func init() {
	// Populate Countries list if it doesn't exist
	if CountryLastUpdated.IsZero() {
		CountryLastUpdated = time.Now()

		Countries = make([]Country, 0)

		for _, c := range country.Countries {
			sdvs := make([]SubDivision, 0)

			for _, sd := range c.SubDivisions() {
				sdvs = append(sdvs, SubDivision{
					sd.Name,
					sd.Code,
				})
			}

			co := Country{
				c.Name.Common,
				c.Codes.Alpha2,
				sdvs,
			}

			Countries = append(Countries, co)
		}
	}
}

// ShopJS request and response
type LoadShopJSReq struct {
	HasCountries     bool `json:"hasCountries"`
	HasTaxRates      bool `json:"hasTaxRates"`
	HasShippingRates bool `json:"hasShippingRates"`

	GetUpdatedAt time.Time `json:"date"`
	StoreId      string    `json:"storeId"`
}

type LoadShopJSRes struct {
	Countries []Country `json:"countries"`
	// TaxRates  *TaxRates `json:"taxRates"`
	// ShippingRates  *ShippingRates `json:"shippingRates"`
}

func LoadShopJS(c *gin.Context) {
	// org := middleware.GetOrganization(c)
	// db := datastore.New(org.Namespaced(c))

	req := &LoadShopJSReq{}

	// Decode response body to get ShopJS Params
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if !req.HasCountries {

	} else {

	}

	if !req.HasTaxRates {

	} else {

	}

	if !req.HasShippingRates {

	} else {

	}
}
