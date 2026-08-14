package checkout

import "testing"

// The Hanzo brand is monochrome, and the checkout says who takes the money.
//
// `primaryColor` is not decoration: pay renders the primary CTA from it, so a
// hue here IS the "Continue to payment" button. It shipped as #808000 — CSS
// olive — which cleared pay's 3:1 legibility bar and so was accepted, painting
// the one control that matters mustard. Nothing failed; it was simply wrong,
// and it was wrong only for Hanzo (Lux and Zoo were already #ffffff).
//
// @hanzo/design states the rule this restores: "Hanzo is monochrome. One hue
// rendered through an opacity ladder," with the only permitted hues being
// error and success state. A brand accent is not a value this system has.
func TestHanzoBrandIsMonochromeAndNamesItsEntity(t *testing.T) {
	if got := brandHanzo.primaryColor; got != "#ffffff" {
		t.Fatalf("brandHanzo.primaryColor = %q, want #ffffff — the design system is monochrome", got)
	}
	// The footer is where a checkout says whose contract it is. A brand string
	// is not an entity: "Hanzo" sells nothing, "Hanzo Industries Inc." does.
	if brandHanzo.legalName == "" || brandHanzo.legalName == brandHanzo.displayName {
		t.Fatalf("brandHanzo.legalName = %q — must name the entity, not the brand", brandHanzo.legalName)
	}
	for name, url := range map[string]string{"termsURL": brandHanzo.termsURL, "privacyURL": brandHanzo.privacyURL} {
		if url == "" {
			t.Fatalf("brandHanzo.%s is empty — a checkout must link the policies it sells under", name)
		}
	}
}

// Every brand projects its entity, and one that has not named one falls back to
// its display name rather than to empty — a footer with no name at all is worse
// than a footer with the brand's.
func TestLegalNameFallsBackToDisplayName(t *testing.T) {
	org, err := NewOrgResolver(nil).Resolve("pay.zoo.ngo")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if org.Brand.LegalName == "" {
		t.Fatal("LegalName is empty — the fallback to displayName did not apply")
	}
}
