package hashid

import "fmt"

// DO NOT ALPHABETIZE THESE
var kinds = map[string]int{
	"bundle":              0,
	"campaign":            1,
	"collection":          2,
	"coupon":              3,
	"namespace":           4,
	"order":               5,
	"organization":        6,
	"payment":             7,
	"plan":                8,
	"analyticsevent":      9,
	"product":             10,
	"store":               11,
	"token":               12,
	"user":                13,
	"variant":             14,
	"mailinglist":         15,
	"subscriber":          16,
	"referral":            17,
	"referrer":            18,
	"transaction":         19,
	"funnel":              20,
	"aggregate":           21,
	"site":                22,
	"deploy":              23,
	"submission":          24,
	"cart":                31,
	"affiliate":           32,
	"fee":                 33,
	"transfer":            34,
	"reversal":            35,
	"partner":             36,
	"discount":            37,
	"webhook":             38,
	"referralprogram":     39,
	"review":              40,
	"return":              41,
	"note":                42,
	"analyticsidentifier": 43,
	"taxrates":            44,
	"shippingrates":       45,
}

var kindsReversed = make(map[int]string)

func init() {
	for k, v := range kinds {
		kindsReversed[v] = k
	}
}

func encodeKind(kind string) int {
	if encoded, ok := kinds[kind]; ok {
		return encoded
	} else {
		panic(fmt.Sprintf("Unknown kind '%s', register in util/hashid/kind.go", kind))
	}
}

func decodeKind(encoded int) (string, error) {
	if kind, ok := kindsReversed[encoded]; ok {
		return kind, nil
	} else {
		return "", fmt.Errorf("Unknown encoded kind '%s', register in util/hashid/kind.go", encoded)
	}
}
