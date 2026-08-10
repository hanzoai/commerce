package commerce

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/payment/processor"
)

// Sweep collects deposited money out of every custody address holding one asset
// and into the address `to`.
//
// It is driven by a PERSON, from `commerce sweep`, and by no schedule. Two
// things about the operation want a human and would be guesses without one:
// every EVM token sweep begins at an address holding a token and no gas
// (custody.ErrNoFee is its ordinary first state), so somebody has to fund it;
// and `to` is a treasury address, which nothing in this system should be
// inferring on its own.
//
// It configures nothing. The asset comes from the same CRYPTO_DEPOSIT_*
// environment the watcher reads, so an operator can only sweep an asset the rail
// was actually taking deposits for, and the two can never be pointed at
// different chains, contracts or endpoints.
func Sweep(ctx context.Context, chain, token, to string) error {
	if to == "" {
		return fmt.Errorf("sweep needs --to: the address the money is going to")
	}
	chain, token = strings.ToLower(chain), strings.ToLower(token)

	assets, err := depositwatch.AssetsFromEnv(os.Environ())
	if err != nil {
		return err
	}
	var asset depositwatch.Asset
	for _, a := range assets {
		if a.Chain == chain && a.Token == token {
			asset = a
			break
		}
	}
	if asset.Chain == "" {
		return fmt.Errorf("nothing is configured for %s/%s; CRYPTO_DEPOSIT_* names %d asset(s)", chain, token, len(assets))
	}

	// The processor is the fleet's one client, so it is also its signer.
	p, err := processor.Get(processor.MPC)
	if err != nil {
		return err
	}
	signer, ok := p.(custody.Signer)
	if !ok {
		return fmt.Errorf("processor %s cannot sign", processor.MPC)
	}

	c, err := depositledger.Chain(ctx, asset)
	if err != nil {
		return err
	}
	results, err := depositledger.Sweep(ctx, c, signer, asset, to)
	if err != nil {
		return err
	}

	var moved, failed int
	for _, r := range results {
		if r.Err != nil {
			failed++
			fmt.Printf("%-12s %s  FAILED: %v\n", r.Org, r.Address, r.Err)
			continue
		}
		moved++
		fmt.Printf("%-12s %s  -> %s  %s\n", r.Org, r.Address, to, r.TxID)
	}
	fmt.Printf("%s/%s: %d swept, %d failed (addresses holding nothing are not listed)\n", chain, token, moved, failed)
	if failed > 0 {
		return fmt.Errorf("%d address(es) could not be swept", failed)
	}
	return nil
}
