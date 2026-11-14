package billing

import (
	"context"
	"math"
	"strings"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// fxRecord is what a payment made in the buyer's currency cost and settled for, written
// on its transaction beside the credit: the local amount and currency, the USD credited,
// the rate and where it came from, who bore the conversion cost, Stripe's quote and its
// fee terms, and, once Stripe reports the balance transaction, the USD it settled for,
// its fees, the FX cost and the USD net. Costs are recorded and never added to a price.
// A field not yet known is absent, not zero.
func fxRecord(ctx context.Context, sp processor.PaymentProcessor, intentID string, charged int64, chargedCur currency.Type, credit int64) map[string]interface{} {
	rec, found, err := pendingintent.Get(intentID)
	if err != nil || !found || rec.PaidCurrency == "" {
		return nil
	}
	m := map[string]interface{}{
		"localAmount": charged, "localCurrency": strings.ToLower(string(chargedCur)), "usdAmount": credit,
		"rate": rec.Rate, "rateSource": rec.RateSource, "fxFee": rec.FXFee,
	}
	if rec.QuoteID != "" {
		m["quoteId"], m["baseRate"], m["fxFeeRate"], m["durationPremium"], m["exchangeRate"] = rec.QuoteID, rec.BaseRate, rec.FeeRate, rec.Premium, rec.ExchangeRate
		m["referenceRate"], m["referenceProvider"], m["lockExpiresAt"] = rec.ReferenceRate, rec.ReferenceProvider, rec.LockExpiresAt
	}
	sr, ok := sp.(processor.SettlementReader)
	if !ok {
		return m
	}
	s, err := sr.Settlement(ctx, intentID)
	if err != nil {
		log.Warn("settlement %s: the balance transaction is not readable yet: %v", intentID, err)
		return m
	}
	m["balanceTransaction"], m["usdSettled"], m["processorFee"], m["tax"], m["usdNet"] = s.BalanceTransaction, s.Gross, s.Fee-s.Tax, s.Tax, s.Net
	// The FX cost is what the conversion took: the local amount at the base rate, less
	// what Stripe settled. Only known when Stripe's own quote priced it.
	if rec.BaseRate > 0 {
		atBase := float64(charged) / math.Pow10(int(chargedCur.Decimals())) / rec.BaseRate * 100
		m["fxFee"] = max(0, int64(math.Round(atBase))-s.Gross)
	}
	return m
}
