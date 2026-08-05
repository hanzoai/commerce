package region

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	regionModel "github.com/hanzoai/commerce/models/region"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

func Route(router zip.Router, args ...zip.Handler) {
	namespaced := middleware.Namespace()

	api := rest.New(regionModel.Region{})

	api.GET("/:regionid/countries", namespaced, ListCountries)
	api.POST("/:regionid/countries", namespaced, AddCountry)
	api.DELETE("/:regionid/countries/:countryCode", namespaced, RemoveCountry)

	api.Route(router, args...)
}

// ListCountries returns all countries for a region.
func ListCountries(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("regionid")

	r := regionModel.New(db)
	if err := r.GetById(id); err != nil {
		return http.Fail(c, 404, "No region found with id: "+id, err)
	}

	return http.Render(c, 200, r.Countries)
}

// AddCountry adds a country to a region's country list.
func AddCountry(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("regionid")

	r := regionModel.New(db)
	if err := r.GetById(id); err != nil {
		return http.Fail(c, 404, "No region found with id: "+id, err)
	}

	country := regionModel.Country{}
	if err := json.DecodeBytes(c.Body(), &country); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	if country.ISO2 == "" {
		return http.Fail(c, 400, "Country iso2 code is required", errors.New("missing iso2"))
	}

	// Check for duplicate
	for _, existing := range r.Countries {
		if existing.ISO2 == country.ISO2 {
			return http.Fail(c, 409, "Country already exists in region: "+country.ISO2, errors.New("duplicate country"))
		}
	}

	country.RegionId = r.Id()
	r.Countries = append(r.Countries, country)

	if err := r.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update region", err)
	}

	return http.Render(c, 200, r)
}

// RemoveCountry removes a country from a region by ISO2 code.
func RemoveCountry(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("regionid")
	countryCode := c.Param("countryCode")

	r := regionModel.New(db)
	if err := r.GetById(id); err != nil {
		return http.Fail(c, 404, "No region found with id: "+id, err)
	}

	found := false
	countries := make([]regionModel.Country, 0, len(r.Countries))
	for _, country := range r.Countries {
		if country.ISO2 == countryCode {
			found = true
			continue
		}
		countries = append(countries, country)
	}

	if !found {
		return http.Fail(c, 404, "Country not found in region: "+countryCode, errors.New("country not found"))
	}

	r.Countries = countries

	if err := r.Update(); err != nil {
		return http.Fail(c, 500, "Failed to update region", err)
	}

	return http.Render(c, 200, r)
}
