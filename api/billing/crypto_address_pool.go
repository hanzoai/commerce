package billing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"
)

var poolMintMu sync.Mutex

func normalizeDepositAddr(addr string) string {
	a := strings.TrimSpace(addr)
	if strings.HasPrefix(a, "0x") || strings.HasPrefix(a, "0X") {
		return strings.ToLower(a)
	}
	return a
}

func queryAddressRows(db *datastore.Datastore, address string) ([]*cryptopaymentintent.CryptoPaymentIntent, error) {
	trimmed := strings.TrimSpace(address)
	rows := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
	if _, err := cryptopaymentintent.Query(db).Filter("DepositAddress=", trimmed).GetAll(&rows); err != nil {
		return nil, err
	}
	if norm := normalizeDepositAddr(trimmed); norm != trimmed {
		lowerRows := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
		if _, err := cryptopaymentintent.Query(db).Filter("DepositAddress=", norm).GetAll(&lowerRows); err == nil {
			seen := make(map[string]bool, len(rows))
			for _, r := range rows {
				seen[r.Id()] = true
			}
			for _, r := range lowerRows {
				if !seen[r.Id()] {
					rows = append(rows, r)
				}
			}
		}
	}
	return rows, nil
}

func allOrgDatastores(localDB *datastore.Datastore) ([]*datastore.Datastore, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(context.Background())).GetAll(&orgs); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(orgs)+1)
	dbs := make([]*datastore.Datastore, 0, len(orgs)+1)
	if localDB != nil {
		ns := nscontext.GetNamespace(localDB.Context)
		seen[ns] = true
		dbs = append(dbs, localDB)
	}
	for _, o := range orgs {
		name := strings.TrimSpace(o.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		dbs = append(dbs, datastore.New(nscontext.WithNamespace(context.Background(), name)))
	}
	return dbs, nil
}

// globalAddressSpokenFor reports whether any open (Pending or Confirming, non-expired)
// payment across ANY organization and ANY chain holds address, excluding exceptID.
// Cross-org and cross-chain scope prevents address collisions on shared platform custody
// accounts and unified multi-chain address pools.
func globalAddressSpokenFor(localDB *datastore.Datastore, address, exceptID string) bool {
	dbs, err := allOrgDatastores(localDB)
	if err != nil {
		return true
	}
	now := time.Now()
	var holder *cryptopaymentintent.CryptoPaymentIntent
	var mine *cryptopaymentintent.CryptoPaymentIntent
	for _, db := range dbs {
		rows, qerr := queryAddressRows(db, address)
		if qerr != nil {
			return true
		}
		for _, r := range rows {
			if r.Id() == exceptID {
				mine = r
				continue
			}
			if (r.Status != cryptopaymentintent.Pending && r.Status != cryptopaymentintent.Confirming) || now.After(r.ExpiresAt) {
				continue
			}
			if exceptID == "" {
				return true
			}
			if holder == nil || r.CreatedAt.Before(holder.CreatedAt) {
				holder = r
			}
		}
	}
	if exceptID == "" || holder == nil {
		return false
	}
	return mine != nil && !mine.CreatedAt.Before(holder.CreatedAt)
}

// globalAddressEverSettled reports whether address has EVER completed a deposit
// (Succeeded or Refunded) across any organization, ensuring settled pool wallets
// are retired permanently and never handed to a subsequent payer.
func globalAddressEverSettled(localDB *datastore.Datastore, address string) bool {
	dbs, err := allOrgDatastores(localDB)
	if err != nil {
		return true
	}
	for _, db := range dbs {
		rows, qerr := queryAddressRows(db, address)
		if qerr != nil {
			return true
		}
		for _, r := range rows {
			if r.Status == cryptopaymentintent.Succeeded || r.Status == cryptopaymentintent.Refunded {
				return true
			}
		}
	}
	return false
}

// addressSpokenFor preserves the single-org helper signature while enforcing global
// cross-org exclusivity.
func addressSpokenFor(db *datastore.Datastore, address, except string) bool {
	return globalAddressSpokenFor(db, address, except)
}

func openSameQuotePoolIntent(
	db *datastore.Datastore,
	poolProc processor.DepositPoolProcessor,
	payer, chain, token string,
	amountCents int64,
) (*cryptopaymentintent.CryptoPaymentIntent, bool) {
	if poolProc == nil || amountCents <= 0 {
		return nil, false
	}
	rows := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
	if _, err := cryptopaymentintent.Query(db).
		Filter("CustomerRef=", payer).
		Filter("Chain=", chain).
		Filter("Token=", token).
		Filter("Status=", string(cryptopaymentintent.Pending)).
		GetAll(&rows); err != nil {
		return nil, false
	}
	for _, r := range rows {
		if r == nil || r.IsExpired() || r.DepositAddress == "" || !poolProc.OwnsWallet(r.WalletID) {
			continue
		}
		if r.TxHash == "" && r.Amount == amountCents && !globalAddressEverSettled(db, r.DepositAddress) {
			return r, true
		}
	}
	return nil, false
}

// evictOldestIdlePoolIntent expires one un-sighted Pending intent belonging to poolProc
// that has sat idle past idleEvictAfter when all pool slots are occupied, so abandoned
// tabs cannot starve active depositors.
func evictOldestIdlePoolIntent(localDB *datastore.Datastore, poolProc processor.DepositPoolProcessor, idleEvictAfter time.Duration) bool {
	if poolProc == nil || idleEvictAfter <= 0 {
		return false
	}
	dbs, err := allOrgDatastores(localDB)
	if err != nil {
		return false
	}
	now := time.Now()
	var oldest *cryptopaymentintent.CryptoPaymentIntent
	var oldestDB *datastore.Datastore
	for _, db := range dbs {
		rows := make([]*cryptopaymentintent.CryptoPaymentIntent, 0)
		if _, qerr := cryptopaymentintent.Query(db).
			Filter("Status=", string(cryptopaymentintent.Pending)).
			GetAll(&rows); qerr != nil {
			continue
		}
		for _, r := range rows {
			if r == nil || now.After(r.ExpiresAt) || !poolProc.OwnsWallet(r.WalletID) {
				continue
			}
			if r.TxHash != "" || r.Confirmations > 0 {
				continue
			}
			if now.Sub(r.CreatedAt) < idleEvictAfter {
				continue
			}
			if oldest == nil || r.CreatedAt.Before(oldest.CreatedAt) {
				oldest = r
				oldestDB = db
			}
		}
	}
	if oldest == nil || oldestDB == nil {
		return false
	}
	in := cryptopaymentintent.New(oldestDB)
	if err := in.GetById(oldest.Id()); err != nil || in.Status != cryptopaymentintent.Pending || in.TxHash != "" {
		return false
	}
	in.Status = cryptopaymentintent.Expired
	return in.Update() == nil
}

func mintPerPaymentIntent(
	ctx context.Context,
	db *datastore.Datastore,
	cp processor.CryptoProcessor,
	payer, chain, token string,
	amountCents int64,
) (*cryptopaymentintent.CryptoPaymentIntent, error) {
	poolProc, isPool := cp.(processor.DepositPoolProcessor)
	var pendingTTL, idleEvictAfter time.Duration
	if isPool {
		pendingTTL, idleEvictAfter = poolProc.PoolLeaseTerms()
		poolMintMu.Lock()
		defer poolMintMu.Unlock()
		if existing, ok := openSameQuotePoolIntent(db, poolProc, payer, chain, token, amountCents); ok {
			return existing, nil
		}
	}

	mintCtx := processor.WithToken(ctx, token)
	mintCtx = processor.WithAddressFree(mintCtx, func(addr string) bool {
		return !globalAddressSpokenFor(db, addr, "")
	})
	if isPool {
		mintCtx = processor.WithAddressFresh(mintCtx, func(addr string) bool {
			return !globalAddressEverSettled(db, addr)
		})
	}

	wallet, err := cp.GenerateAddress(mintCtx, payer, chain)
	if err != nil && isPool && idleEvictAfter > 0 && isNoFreeAddressError(err) {
		if evictOldestIdlePoolIntent(db, poolProc, idleEvictAfter) {
			wallet, err = cp.GenerateAddress(mintCtx, payer, chain)
		}
	}
	if err != nil {
		return nil, depositRefusal{
			msg:   "Crypto deposits are temporarily unavailable — the custody service is not accepting new addresses. Try again shortly or choose another way to pay.",
			cause: fmt.Errorf("crypto deposit address generation failed for %q on %s: %w", payer, chain, err),
		}
	}

	ttl := 24 * time.Hour
	if pendingTTL > 0 {
		ttl = pendingTTL
	}
	intent, err := recordIntentWithTTL(db, payer, chain, token, amountCents, wallet, wallet.Tag, ttl)
	if err != nil {
		return nil, err
	}
	if globalAddressSpokenFor(db, wallet.Address, intent.Id()) {
		intent.Status = cryptopaymentintent.Expired
		_ = intent.Update()
		return nil, depositRefusal{msg: "Crypto deposits are temporarily unavailable — no deposit address is free right now. Try again shortly or choose another way to pay."}
	}
	return intent, nil
}

func isNoFreeAddressError(err error) bool {
	if err == nil {
		return false
	}
	if pe, ok := err.(*processor.PaymentError); ok && pe.Code == "NO_FREE_ADDRESS" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no free") ||
		strings.Contains(strings.ToLower(err.Error()), "no fresh deposit address")
}
