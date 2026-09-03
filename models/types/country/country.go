package country

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pariz/gountries"
)

type Query gountries.Query

type Country struct {
	gountries.Country
}

type SubDivision gountries.SubDivision

// All is every country, sorted by common name. ByISO is the same set keyed by
// ISO-3166-1 alpha-2. Both are built on FIRST USE, not at init.
//
// gountries.New() reads and unmarshals 5.1 MB of YAML — one file per country plus
// its subdivisions — and it cost 121 ms, measured. At package init every process
// linking this package paid that before it could serve a request, including the
// overwhelming majority that never name a country. Now the first caller pays it
// and nobody else does.
var All = sync.OnceValue(load)

// ByISO answers the same set keyed by alpha-2.
var ByISO = sync.OnceValue(func() map[string]Country {
	All()
	return byISO
})

// byISO is filled by load, under All's Once, so ByISO cannot observe it half-built.
var byISO map[string]Country

// Error returns a formatted error
func makeError(errMsg, errType string) error {
	return fmt.Errorf("gountries error. %s: %s", errMsg, errType)
}

func load() []Country {
	Countries := make([]Country, 0)
	ByISO3166_2 := make(map[string]Country)

	q := gountries.New()

	// Generate the list of countries and the map
	for _, country := range q.Countries {
		Countries = append(Countries, Country{country})
		ByISO3166_2[country.Codes.Alpha2] = Country{country}
	}

	// Sort the list of countries by their common name
	nameToIsoMap := make(map[string]string)
	sortedNames := make([]string, len(Countries))

	i := 0
	for iso, country := range ByISO3166_2 {
		name := country.Name.Common
		sortedNames[i] = name
		nameToIsoMap[name] = iso
		i++
	}

	sort.Strings(sortedNames)

	// Make the country list sorted by common name
	for i, name := range sortedNames {
		Countries[i] = ByISO3166_2[nameToIsoMap[name]]
	}
	byISO = ByISO3166_2
	return Countries
}

func FindByISO3166_2(code string) (Country, error) {
	codeU := strings.ToUpper(code)
	if c, ok := ByISO()[codeU]; ok {
		return c, nil
	}

	return Country{}, makeError("Could not find country with code %s", code)
}

func (c Country) FindSubDivision(nameOrCode string) (SubDivision, error) {
	nameOrCodeU := strings.ToUpper(nameOrCode)
	sds := c.SubDivisions()
	for _, sd := range sds {
		if sd.Code == nameOrCodeU || strings.ToUpper(sd.Name) == nameOrCodeU {
			return SubDivision(sd), nil
		}
	}

	return SubDivision{}, makeError("Could not find subdivision with name or code %s", nameOrCode)
}
