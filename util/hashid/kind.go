package hashid

import (
	"errors"
	"fmt"
)

var kinds = map[string]int{
	"bundle":       0,
	"campaign":     1,
	"collection":   2,
	"coupon":       3,
	"discounts":    4,
	"order":        5,
	"organization": 6,
	"payment":      7,
	"plan":         8,
	"price":        9, // No longer used, kept for historical purposes
	"product":      10,
	"store":        11,
	"token":        12,
	"user":         13,
	"variant":      14,
	"mailinglist":  15,
	"subscriber":   16,
}

var kindsReversed = make(map[int]string)

func init() {
	for k, v := range kinds {
		kindsReversed[v] = k
	}
}

func encodeKind(kind string) int {
	v, ok := kinds[kind]
	if ok {
		return v
	}
	err := errors.New(fmt.Sprintf("Unknown kind %v. Please register in util/hashid/kind.go.", kind))
	panic(err)

}

func decodeKind(encoded int) string {
	return kindsReversed[encoded]
}
