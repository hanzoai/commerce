package billing

import (
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// applyEnvDisabledProcessors clamps any processor named in COMMERCE_DISABLED_PROCESSORS
// off in the returned policy copy so routing decisions, fallbacks, local-currency offers,
// crypto rails, and accounts never use a hard-disabled processor — symmetrically across
// all card, alternative, and custody providers without hardcoding any provider name.
func applyEnvDisabledProcessors(p routing.Policy) routing.Policy {
	anyDisabled := false
	var enabledCardNames []string
	if len(p.Processors) > 0 {
		procs := make([]routing.ProcessorRule, len(p.Processors))
		copy(procs, p.Processors)
		firstEnabledCardIdx := -1
		totalCardWeight := 0
		for i := range procs {
			t := processor.ProcessorType(procs[i].Processor)
			if processor.DisabledByEnv(t) {
				procs[i].On = false
				procs[i].Weight = 0
				anyDisabled = true
				continue
			}
			if procs[i].On && takesCardsAccount(procs[i].Processor) {
				if firstEnabledCardIdx < 0 {
					firstEnabledCardIdx = i
				}
				totalCardWeight += procs[i].Weight
				enabledCardNames = append(enabledCardNames, procs[i].Processor)
			}
		}
		if anyDisabled && firstEnabledCardIdx >= 0 && totalCardWeight == 0 {
			procs[firstEnabledCardIdx].Weight = 100
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
		if len(fb) == 0 && len(enabledCardNames) > 0 {
			fb = append(fb, enabledCardNames...)
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
	if anyDisabled {
		if !hasEnabledProcessorWithCapability[processor.LocalIntentMaker](p) {
			p.LocalOn = false
		}
		if !hasEnabledProcessorWithCapability[processor.RateQuoter](p) {
			p.FXSource = routing.FXReference
		}
	}
	return p
}

func hasEnabledProcessorWithCapability[T any](p routing.Policy) bool {
	for _, r := range p.Processors {
		if !r.On || processor.DisabledByEnv(processor.ProcessorType(r.Processor)) {
			continue
		}
		reg, err := processor.Global().Registered(processor.ProcessorType(r.Processor))
		if err != nil {
			continue
		}
		if _, ok := reg.(T); ok {
			return true
		}
	}
	return false
}
