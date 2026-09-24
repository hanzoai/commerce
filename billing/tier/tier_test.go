package tier_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hanzoai/commerce/billing/tier"
)

func TestTier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tier Suite")
}

var _ = Describe("Tier", func() {
	Describe("Get", func() {
		It("returns the correct config for known tiers", func() {
			// No free tier: Free grants NO daily credit, so a zero-balance
			// account is gated (effectiveAvailable collapses to prepaid).
			cfg := tier.Get(tier.Free)
			Expect(cfg.Name).To(Equal(tier.Free))
			Expect(cfg.DailyCreditsCents).To(Equal(int64(0)))

			cfg = tier.Get(tier.Pro)
			Expect(cfg.Name).To(Equal(tier.Pro))
			Expect(cfg.DailyCreditsCents).To(Equal(int64(0)))
		})

		It("falls back to Free for unknown tiers", func() {
			cfg := tier.Get(tier.Name("nonexistent"))
			Expect(cfg.Name).To(Equal(tier.Free))
		})
	})

	Describe("Parse", func() {
		It("parses known tier strings", func() {
			Expect(tier.Parse("free")).To(Equal(tier.Free))
			Expect(tier.Parse("starter")).To(Equal(tier.Starter))
			Expect(tier.Parse("pro")).To(Equal(tier.Pro))
			Expect(tier.Parse("enterprise")).To(Equal(tier.Enterprise))
		})

		It("defaults empty or unknown strings to Free", func() {
			Expect(tier.Parse("")).To(Equal(tier.Free))
			Expect(tier.Parse("gold")).To(Equal(tier.Free))
		})
	})

	Describe("IsModelAllowed", func() {
		It("allows wildcard tiers access to all models", func() {
			cfg := tier.Get(tier.Pro)
			Expect(cfg.IsModelAllowed("claude-opus-4")).To(BeTrue())
			Expect(cfg.IsModelAllowed("anything")).To(BeTrue())
		})

		It("holds free and trial callers to the free lane", func() {
			for _, name := range []tier.Name{tier.Free, tier.Starter} {
				cfg := tier.Get(name)
				for _, model := range []string{"enso-auto", "enso-free", "zen-free", "free"} {
					Expect(cfg.IsModelAllowed(model)).To(BeTrue(), "%s may call %s", name, model)
				}
				for _, model := range []string{"anthropic/claude-opus-5.5", "claude-sonnet-4-20250514", "openai/gpt-6-sol", "zen5-pro", "enso-pro"} {
					Expect(cfg.IsModelAllowed(model)).To(BeFalse(), "%s must not call %s", name, model)
				}
			}
		})
	})

	Describe("HasDailyCredits", func() {
		It("is false for every tier — there is no free tier", func() {
			// The daily-credit mechanism is retained but disabled by config
			// (DailyCreditsCents == 0 everywhere), so no tier grants a standing
			// daily balance. A zero-balance account is always gated.
			Expect(tier.Get(tier.Free).HasDailyCredits()).To(BeFalse())
			Expect(tier.Get(tier.Starter).HasDailyCredits()).To(BeFalse())
			Expect(tier.Get(tier.Pro).HasDailyCredits()).To(BeFalse())
			Expect(tier.Get(tier.Enterprise).HasDailyCredits()).To(BeFalse())
		})
	})

	Describe("All", func() {
		It("returns all four tiers", func() {
			all := tier.All()
			Expect(all).To(HaveLen(4))
			Expect(all).To(HaveKey(tier.Free))
			Expect(all).To(HaveKey(tier.Starter))
			Expect(all).To(HaveKey(tier.Pro))
			Expect(all).To(HaveKey(tier.Enterprise))
		})
	})
})
