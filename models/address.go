package models

import (
	"net/http"
	"strings"

	"github.com/mholt/binding"

	"crowdstart.io/models/types/country"
)

type Address struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

func (a Address) Line() string {
	return a.Line1 + " " + a.Line2
}

func compact(str string) string {
	return strings.ToUpper(strings.Replace(strings.TrimSpace(str), " ", "", -1))
}

func (a Address) MatchCountry(country country.Country) bool {
	names := []string{
		country.ISO3166OneAlphaTwo,
		country.ISO3166OneAlphaThree,
		country.BGNEnglishLongName,
		country.PCGNEnglishLongName,
		country.UNGEGNEnglishFormalName,
		country.BGNEnglishShortNameReadingOrder,
		country.BGNEnglishShortNameGazetteerOrder,
		country.PCGNEnglishShortNameReadingOrder,
		country.PCGNEnglishShortNameGazetteerOrder,
		country.ISO3166OneEnglishShortNameReadingOrder,
		country.ISO3166OneEnglishShortNameGazetteerOrder,
	}

	ref := compact(a.Country)

	for _, name := range names {
		if ref == compact(name) {
			return true
		}
	}
	return false
}

func (a Address) DisplayCountry() string {
	return strings.TrimSpace(a.Country)
}

func (a Address) Validate(req *http.Request, errs binding.Errors) binding.Errors {

	if a.Line() == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Street"},
			Classification: "InputError",
			Message:        "Address Street is required.",
		})
	}

	if a.City == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"City"},
			Classification: "InputError",
			Message:        "Address City is required.",
		})
	}

	if a.State == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"State"},
			Classification: "InputError",
			Message:        "Address State is required.",
		})
	}

	if a.Country == "" {
		errs = append(errs, binding.Error{
			FieldNames:     []string{"Country"},
			Classification: "InputError",
			Message:        "Address Country is required.",
		})
	}
	return errs
}
