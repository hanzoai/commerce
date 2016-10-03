package country

import (
	"sort"

	"github.com/vincent-petithory/countries"
)

type Country countries.Country

var Countries []Country
var ByISOCodeISO3166_2 map[string]Country
var numCountries int

func init() {
	numCountries = len(countries.Countries)
	Countries = make([]Country, numCountries)
	ByISOCodeISO3166_2 = make(map[string]Country)

	nameToIsoMap := make(map[string]string)
	sortedNames := make([]string, numCountries)
	i := 0
	for iso, country := range countries.Countries {
		name := country.ISO3166OneEnglishShortNameReadingOrder
		sortedNames[i] = name
		nameToIsoMap[name] = iso
		ByISOCodeISO3166_2[iso] = Country(country)
		i++
	}

	sort.Strings(sortedNames)

	for i, name := range sortedNames {
		Countries[i] = ByISOCodeISO3166_2[nameToIsoMap[name]]
	}
}
