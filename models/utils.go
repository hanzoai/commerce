package models

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	humanize "github.com/dustin/go-humanize"

	"crowdstart.com/datastore"
	"crowdstart.com/models/types/currency"
)

func FloatPrice(price currency.Cents) float64 {
	return math.Floor(float64(price)*100+0.5) / 10000
}

// TODO: Make this work with non-decimal currencies
func DisplayPrice(price currency.Cents) string {
	f := strconv.FormatFloat(FloatPrice(price), 'f', 2, 64)
	bits := strings.Split(f, ".")
	decimal := bits[1]
	integer, _ := strconv.ParseInt(bits[0], 10, 64)
	return humanize.Comma(integer) + "." + decimal
}

// Non-breaking hyphens in title
func DisplayTitle(title string) string {
	return strings.Replace(title, "-", "&#8209;", -1)
}

func SplitParagraph(text string) []string {
	return regexp.MustCompile("\\n\\s*\\n").Split(text, -1)
}

func GetNamespaces(c interface{}) []string {
	namespaces := make([]string, 0)
	db := datastore.New(c)
	keys, err := db.Query("__namespace__").KeysOnly().GetAll(nil)
	if err != nil {
		panic(err)
	}

	for _, k := range keys {
		namespaces = append(namespaces, k.StringID())
	}

	return namespaces
}

func GetKinds(c interface{}) []string {
	kinds := make([]string, 0)
	db := datastore.New(c)
	keys, err := db.Query("__kind__").KeysOnly().GetAll(nil)
	if err != nil {
		panic(err)
	}

	for _, k := range keys {
		kinds = append(kinds, k.StringID())
	}

	return kinds
}
