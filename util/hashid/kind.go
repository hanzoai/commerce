package hashid

import "fmt"

// DO NOT ALPHABETIZE THESE OR ALTER IN ANYWAY
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
	"form":                15,
	"subscriber":          16,
	"referral":            17,
	"referrer":            18,
	"transaction":         19,
	"funnel":              20,
	"aggregate":           21,
	"site":                22,
	"deploy":              23,
	"submission":          24,
	"subscription":        25,
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
	"oauthtoken":          46,
	"app":                 47,
	"wallet":              48,
	"tokensale":           49,
	"adcampaign":          50,
	"adconfig":            51,
	"adset":               52,
	"ad":                  53,
	"copy":                54,
	"media":               55,
	"block":               56,
	"blockaddress":        57,
	"blocktransaction":    58,
	"paymentmethod":       60,

	// virtual kind used for making ancestor keys to force data synchronization
	"synckey":          59,
	"tokentransaction": 100,
	"disclosure":       101,
	"movie":            200,
	"watchlist":        201,
	"meter":            202,
	"credit-grant":     203,

	// Billing & commerce entities added for ORM compatibility
	"meter-event":              204,
	"billing-pricing-rule":     205,
	"billing-event":            206,
	"billing-invoice":          207,
	"billing-payout":           208,
	"balance-transaction":      209,
	"bank-transfer-instruction": 210,
	"credit-note":              211,
	"customer-balance":         212,
	"dispute":                  213,
	"payment-intent":           214,
	"refund":                   215,
	"spend-alert":              216,
	"webhook-endpoint":         217,
	"network-token":            218,
	"crypto-balance":           219,
	"crypto-payment-intent":    220,
	"usage-watermark":          221,
	"setup-intent":             222,
	"notification":             223,
	"apipermission":            224,
	"applicationmethod":        225,
	"campaignbudget":           226,
	"contributor":              227,
	"customergroup":            228,
	"customergroupmembership":  229,
	"fulfillment":              230,
	"fulfillmentprovider":      231,
	"fulfillmentset":           232,
	"geozone":                  233,
	"inventoryitem":            234,
	"inventorylevel":           235,
	"price":                    236,
	"pricelist":                237,
	"pricepreference":          238,
	"pricerule":                239,
	"priceset":                 240,
	"promotion":                241,
	"promotionrule":            242,
	"publishableapikey":        243,
	"redemption":               244,
	"region":                   245,
	"reservation":              246,
	"role":                     247,
	"saleschannel":             248,
	"sbom-entry":               249,
	"servicezone":              250,
	"shippingoption":           251,
	"shippingoptionrule":       252,
	"shippingprofile":          253,
	"stocklocation":            254,
	"taxprovider":              255,
	"taxrate":                  256,
	"taxraterule":              257,
	"taxregion":                258,
	"variantinventorylink":     259,
	"subscription-item":        260,
	"subscription-schedule":    261,
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
		return "", fmt.Errorf("Unknown encoded kind '%d', register in util/hashid/kind.go", encoded)
	}
}
