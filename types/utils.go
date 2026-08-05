package types

import (
	"regexp"
	"strings"

	"github.com/hanzoai/commerce/models/types/currency"
)

// DisplayPrice renders a price for a human: symbol, grouped thousands, the
// currency's own number of decimals ("$1,234.56", "¥10,000").
//
// This is the display funnel — every storefront, order and receipt string in
// commerce comes through here, so it holds no arithmetic of its own. The
// amount is rendered from the integer by money, which cannot round a cent away
// the way the float this used to compute could.
//
// A negative now renders "-$100.00" rather than the old "$-100.00": the sign
// belongs to the amount, not to the digits after the symbol, and a refund is
// the most common negative there is.
func DisplayPrice(t currency.Type, price currency.Cents) string {
	return t.Amount(price).Display()
}

// Non-breaking hyphens in title
func DisplayTitle(title string) string {
	return strings.Replace(title, "-", "&#8209;", -1)
}

func SplitParagraph(text string) []string {
	return regexp.MustCompile("\\n\\s*\\n").Split(text, -1)
}
