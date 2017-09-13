package library

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/store"
	"hanzo.io/models/taxrates"
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

	LastChecked time.Time `json:"lastChecked"`
	StoreId     string    `json:"storeId"`
}

type LoadShopJSRes struct {
	Countries     []Country                    `json:"countries"`
	TaxRates      *taxrates.TaxRates           `json:"taxRates"`
	ShippingRates *shippingrates.ShippingRates `json:"shippingRates"`
}

func LoadShopJS(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	// Decode response body to get ShopJS Params
	req := &LoadShopJSReq{}

	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Default store if StoreId is left blank
	if req.StoreId == "" {
		req.StoreId = org.DefaultStore
	}

	stor := store.New(db)
	if err := stor.GetById(req.StoreId); err != nil {
		http.Fail(c, 404, "Store `"+req.StoreId+"` not found", err)
		return
	}

	// Build response
	res := LoadShopJSRes{}

	if !req.HasCountries ||
		req.LastChecked.Before(CountryLastUpdated) {
		res.Countries = Countries
	}

	if req.HasTaxRates {
		tr := taxrates.New(db)
		if ok, err := tr.Query().Filter("StoreId=", req.StoreId).Filter("UpdatedAt>=", req.LastChecked).Get(); ok {
			res.TaxRates = tr
		} else if err != nil {
			http.Fail(c, 500, err.Error(), err)
			return
		}
	} else {
		tr := taxrates.New(db)
		if ok, err := tr.Query().Filter("StoreId=", req.StoreId).Get(); ok {
			res.TaxRates = tr
		} else if err != nil {
			http.Fail(c, 500, err.Error(), err)
			return
		}
	}

	if req.HasShippingRates {
		sr := shippingrates.New(db)
		if ok, err := sr.Query().Filter("StoreId=", req.StoreId).Filter("UpdatedAt>=", req.LastChecked).Get(); ok {
			res.ShippingRates = sr
		} else if err != nil {
			http.Fail(c, 500, err.Error(), err)
			return
		}
	} else {
		sr := shippingrates.New(db)
		if ok, err := sr.Query().Filter("StoreId=", req.StoreId).Get(); ok {
			res.ShippingRates = sr
		} else if err != nil {
			http.Fail(c, 500, err.Error(), err)
			return
		}
	}

	http.Render(c, 200, res)
}
