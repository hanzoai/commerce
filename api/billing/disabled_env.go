package billing

import (
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// applyEnvDisabledProcessors clamps any processor named in COMMERCE_DISABLED_PROCESSORS
// off in the returned policy copy so routing decisions, fallbacks, local-currency offers,
// crypto rails, and accounts never use a hard-disabled processor even when an older
// stored policy still lists it as on.
func applyEnvDisabledProcessors(p routing.Policy) routing.Policy {
	anyDisabled := false
	if len(p.Processors) > 0 {
		procs := make([]routing.ProcessorRule, len(p.Processors))
		copy(procs, p.Processors)
		squareIdx := -1
		otherCardWeight := 0
		for i := range procs {
			t := processor.ProcessorType(procs[i].Processor)
			if processor.DisabledByEnv(t) {
				procs[i].On = false
				procs[i].Weight = 0
				anyDisabled = true
				continue
			}
			if t == processor.Square && procs[i].On {
				squareIdx = i
			} else if t == processor.Stripe && procs[i].On {
				otherCardWeight += procs[i].Weight
			}
		}
		if anyDisabled && squareIdx >= 0 && procs[squareIdx].Weight == 0 && otherCardWeight == 0 {
			procs[squareIdx].Weight = 100
		}
		p.Processors = procs
	}
	if len(p.Accounts) > 0 {
		accts := make([]routing.Account, len(p.Accounts))
		copy(accts, p.Accounts)
		for i := range accts {
			if processor.DisabledByEnv(processor.ProcessorType(accts[i].Processor)) {
				accts[i].On = false
			}
		}
		p.Accounts = accts
	}
	if len(p.Fallback) > 0 {
		fb := make([]string, 0, len(p.Fallback))
		for _, f := range p.Fallback {
			if !processor.DisabledByEnv(processor.ProcessorType(f)) {
				fb = append(fb, f)
			}
		}
		if len(fb) == 0 && p.On(processor.Square) && !processor.DisabledByEnv(processor.Square) {
			fb = []string{string(processor.Square)}
		}
		p.Fallback = fb
	}
	if len(p.CryptoRails) > 0 {
		cr := make([]routing.CryptoRoute, 0, len(p.CryptoRails))
		for _, r := range p.CryptoRails {
			if !processor.DisabledByEnv(processor.ProcessorType(r.Processor)) {
				cr = append(cr, r)
			}
		}
		p.CryptoRails = cr
	}
	if processor.DisabledByEnv(processor.Stripe) {
		p.LocalOn = false
		p.FXSource = routing.FXReference
	}
	return p
}
