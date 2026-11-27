package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/payment/processor"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// GetCryptoDeposit reports an intent's state (pending -> confirming ->
// succeeded) — the READ, with no HTTP in it.
//
// It takes the payer as a value because the read is SCOPED to them and the scope
// is the point: the same lookup answered over the internal plane must refuse a
// foreign id there too, and a second copy of "is this yours" is a copy that can
// be written without the check. An id that names nothing and an id that names
// somebody else return the SAME error, so neither endpoint can be made to confirm
// that a foreign intent exists.
func GetCryptoDeposit(ctx context.Context, org *organization.Organization, payer, id string) (CryptoDeposit, error) {
	if org == nil {
		return CryptoDeposit{}, errors.New("crypto deposit: no organization")
	}
	if payer == "" {
		return CryptoDeposit{}, errors.New("crypto deposit: no payer")
	}
	if strings.HasPrefix(id, "pi_") {
		p, found, err := pendingintent.Get(id)
		if err != nil || !found || !strings.EqualFold(p.Subject, payer) || !strings.EqualFold(p.Org, org.Name) {
			return CryptoDeposit{}, errDepositNotFound
		}
		chain := strings.TrimSpace(p.Network)
		if chain == "" {
			chain = "base"
		}
		st := string(cryptopaymentintent.Pending)
		var planID, subID string
		sp, serr := processorsForOrg(org).Existing(ctx, processor.Stripe)
		if serr != nil {
			return CryptoDeposit{}, fmt.Errorf("crypto deposit: Stripe unavailable: %w", serr)
		}
		tx, terr := sp.GetTransaction(ctx, id)
		if terr != nil {
			return CryptoDeposit{}, fmt.Errorf("crypto deposit: Stripe transaction read: %w", terr)
		}
		if tx == nil {
			return CryptoDeposit{}, errors.New("crypto deposit: Stripe returned no transaction")
		}
		switch tx.Status {
		case "canceled":
			if !p.Done {
				if _, err := pendingintent.Settle(id); err != nil {
					return CryptoDeposit{}, err
				}
			}
			st = string(cryptopaymentintent.Expired)
		case "succeeded":
			if !p.Done {
				if err := reconcileIntent(ctx, p); err != nil {
					return CryptoDeposit{}, err
				}
				if updated, ok, err := pendingintent.Get(id); err != nil {
					return CryptoDeposit{}, err
				} else if ok && updated != nil {
					p = updated
				}
			}
			if p.Purpose == pendingintent.Plan && p.Ref != "" {
				if s, ok, _ := checkoutsession.Get(p.Ref); ok && s != nil {
					planID, subID = s.Plan, s.SubscriptionID
					if p.Done && subID != "" {
						st = string(cryptopaymentintent.Succeeded)
					}
				}
			} else if p.Purpose == pendingintent.Topup && p.Done && tx.ProcessorRef != "" {
				if _, credited, err := paymentorg.Get(tx.ProcessorRef); err != nil {
					return CryptoDeposit{}, err
				} else if credited {
					st = string(cryptopaymentintent.Succeeded)
				}
			}
		}
		if p.Purpose == pendingintent.Plan && p.Ref != "" && planID == "" {
			if s, ok, _ := checkoutsession.Get(p.Ref); ok && s != nil {
				planID = s.Plan
			}
		}
		return CryptoDeposit{
			ID:             p.Id(),
			Status:         st,
			Chain:          chain,
			Token:          "usdc",
			AmountCents:    p.Cents,
			PlanID:         planID,
			SubscriptionID: subID,
		}, nil
	}
	db := datastore.New(org.Namespaced(ctx))
	intent := cryptopaymentintent.New(db)
	if err := intent.GetById(id); err != nil {
		return CryptoDeposit{}, errDepositNotFound
	}
	if intent.CustomerRef != payer {
		return CryptoDeposit{}, errDepositNotFound
	}
	if (intent.Status == cryptopaymentintent.Pending || intent.Status == cryptopaymentintent.Confirming) &&
		strings.HasPrefix(intent.WalletID, "kraken:") {
		if svc := depositledger.Default(); svc != nil && svc.Running() {
			_, _ = svc.SyncOnce(ctx)
			_ = intent.GetById(id)
		}
	}
	return toCryptoDepositResponse(intent), nil
}

// GetBillingCryptoDeposit is the payer's endpoint for their own deposit.
//
//	GET /v1/billing/crypto/deposit/:id
func GetBillingCryptoDeposit(c *zip.Ctx) error {
	payer := userBillingKey(c)
	if payer == "" {
		return jsonhttp.Fail(c, 401, "Authentication required", nil)
	}
	out, err := GetCryptoDeposit(c.Context(), middleware.GetOrganization(c), payer, c.Param("id"))
	if err != nil {
		return jsonhttp.Fail(c, 404, "deposit not found", err)
	}
	return c.JSON(200, out)
}

func toCryptoDepositResponse(i *cryptopaymentintent.CryptoPaymentIntent) CryptoDeposit {
	out := CryptoDeposit{
		ID:                    i.Id(),
		Status:                string(i.Status),
		Chain:                 string(i.Chain),
		Token:                 i.Token,
		DepositAddress:        i.DepositAddress,
		AddressTag:            i.AddressTag,
		ExpiresAt:             i.ExpiresAt.UTC().Format(time.RFC3339),
		AmountCents:           i.Amount,
		RequiredConfirmations: cryptopaymentintent.RequiredConfirmationsForChain(i.Chain),
		Confirmations:         i.Confirmations,
	}
	if i.Status == cryptopaymentintent.Succeeded {
		out.CreditedCents = i.SettlementAmount
	}
	if i.Metadata != nil {
		if p, _ := i.Metadata["planId"].(string); p != "" {
			out.PlanID = p
		}
		if s, _ := i.Metadata["subscriptionId"].(string); s != "" {
			out.SubscriptionID = s
		}
	}
	if terms, ok := termsFor(watchedAssets(), string(i.Chain), i.Token); ok {
		out.FeeCents = terms.FeeCents
		out.MinimumCents = minimumCents(terms)
	}
	if strings.HasPrefix(i.WalletID, "kraken:") {
		if kMin := krakenCachedMinimumCents(string(i.Chain), i.Token); kMin > out.MinimumCents {
			out.MinimumCents = kMin
		}
	}
	return out
}

// termsFor answers what a deposit of (chain, token) has deducted before it is
// credited, from the watched asset — where the deduction is configured
// (CRYPTO_DEPOSIT_FEE_<CHAIN>). ok is false when nothing watches the asset.
func termsFor(assets []depositwatch.Asset, chain, token string) (depositwatch.Terms, bool) {
	for _, a := range assets {
		if strings.EqualFold(a.Chain, chain) && strings.EqualFold(a.Token, token) {
			return a.Terms, true
		}
	}
	return depositwatch.Terms{}, false
}

// minimumCents is the smallest deposit, in USD cents of what was sent, that
// depositwatch.AmountCents credits under t: the haircut leaves it at least one
// cent more than the fee. Anything less is dust or under the fee and credits
// nothing.
func minimumCents(t depositwatch.Terms) int64 {
	const bps = 10_000
	keep := int64(bps - t.SlippageBps)
	return ((t.FeeCents+1)*bps + keep - 1) / keep
}

func supportedChain(cp processor.CryptoProcessor, chain string) bool {
	for _, s := range cp.SupportedChains() {
		if s == chain {
			return true
		}
	}
	return false
}
