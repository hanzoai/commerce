package models

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"

	humanize "github.com/dustin/go-humanize"

	"hanzo.io/datastore"
	"hanzo.io/models/types/currency"
)

func FloatPrice(price currency.Cents) float64 {
	return math.Floor(float64(price)*100+0.5) / 10000
}

func DisplayPrice(t currency.Type, price currency.Cents) string {
	f := ""
	if t.IsZeroDecimal() {
		f = strconv.FormatFloat(float64(price), 'f', 0, 64)
	} else {
		f = strconv.FormatFloat(FloatPrice(price), 'f', 2, 64)
	}
	bits := strings.Split(f, ".")
	decimal := ""
	if len(bits) > 1 {
		decimal = "." + bits[1]
	}
	integer, _ := strconv.ParseInt(bits[0], 10, 64)
	return t.Symbol() + humanize.Comma(integer) + decimal
}

// Non-breaking hyphens in title
func DisplayTitle(title string) string {
	return strings.Replace(title, "-", "&#8209;", -1)
}

func SplitParagraph(text string) []string {
	return regexp.MustCompile("\\n\\s*\\n").Split(text, -1)
}

func GetNamespaces(ctx context.Context) []string {
	namespaces := make([]string, 0)

	// Fetch namespaces from special __namespace__ table
	db := datastore.New(ctx)
	keys, err := db.Query("__namespace__").GetKeys()
	if err != nil {
		panic(err)
	}

	// Append stringID's
	for _, k := range keys {
		namespaces = append(namespaces, k.StringID())
	}

	return namespaces
}

func GetKinds(ctx context.Context) []string {
	kinds := make([]string, 0)
	db := datastore.New(ctx)
	keys, err := db.Query("__kind__").GetKeys()
	if err != nil {
		panic(err)
	}

	for _, k := range keys {
		kinds = append(kinds, k.StringID())
	}

	return kinds
}
