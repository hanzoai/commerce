package country

import "github.com/vincent-petithory/countries"

type Country countries.Country

var Countries []Country
var ByISOCodeISO3166_2 map[string]Country

var enabledCountries = []string{
	"US",
	"GB",
	"AU",
	"AT",
	"BH",
	"BE",
	"BR",
	"CA",
	"CL",
	"CN",
	"HR",
	"DK",
	"FI",
	"FR",
	"DE",
	"GR",
	"HK",
	"IS",
	"IN",
	"IE",
	"IL",
	"IT",
	"JP",
	"KZ",
	"KR",
	"KW",
	"LB",
	"LT",
	"LU",
	"MO",
	"MY",
	"MX",
	"NL",
	"NZ",
	"NO",
	"PE",
	"PH",
	"PL",
	"PT",
	"PR",
	"RO",
	"RU",
	"SG",
	"ZA",
	"ES",
	"CH",
	"SE",
	"TW",
	"TH",
	"TR",
	"UA",
	"AE",
	"VN",
	"VI",
}

func init() {
	Countries = make([]Country, len(enabledCountries))
	ByISOCodeISO3166_2 = make(map[string]Country)
	nameToIsoMap := make(map[string]string)

	i := 0
	for iso, country := range countries.Countries {
		name := country.ISO3166OneEnglishShortNameReadingOrder
		nameToIsoMap[name] = iso
		ByISOCodeISO3166_2[iso] = Country(country)
		i++
	}

	for i, code := range enabledCountries {
		Countries[i] = ByISOCodeISO3166_2[code]
	}
}
