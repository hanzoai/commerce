package braintree

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

func newProvider() *Provider {
	return &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.Braintree, supportedCurrencies()),
	}
}

func TestType(t *testing.T) {
	p := newProvider()
	if got := p.Type(); got != processor.Braintree {
		t.Errorf("Type() = %q, want %q", got, processor.Braintree)
	}
}

func TestIsAvailable_NotConfigured(t *testing.T) {
	p := newProvider()
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true for unconfigured provider, want false")
	}
}

func TestIsAvailable_EmptyCredentials(t *testing.T) {
	p := newProvider()
	p.Configure(Config{PublicKey: "", PrivateKey: "", MerchantID: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with empty credentials, want false")
	}
}

func TestIsAvailable_PartialCredentials_MissingMerchant(t *testing.T) {
	p := newProvider()
	p.Configure(Config{PublicKey: "pub", PrivateKey: "priv", MerchantID: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with missing merchant ID, want false")
	}
}

func TestIsAvailable_PartialCredentials_MissingPrivateKey(t *testing.T) {
	p := newProvider()
	p.Configure(Config{PublicKey: "pub", PrivateKey: "", MerchantID: "merch"})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with missing private key, want false")
	}
}

func TestConfigure_Valid(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		PublicKey:   "pub-key",
		PrivateKey:  "priv-key",
		MerchantID:  "merchant-id",
		Environment: "sandbox",
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after valid Configure(), want true")
	}
}

func TestConfigure_Production(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		PublicKey:   "pub-key",
		PrivateKey:  "priv-key",
		MerchantID:  "merchant-id",
		Environment: "production",
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after production Configure(), want true")
	}
}

func TestConfigure_Reconfigure(t *testing.T) {
	p := newProvider()
	p.Configure(Config{PublicKey: "p", PrivateKey: "p", MerchantID: "m"})
	if !p.IsAvailable(context.Background()) {
		t.Error("expected available after first configure")
	}
	p.Configure(Config{PublicKey: "", PrivateKey: "", MerchantID: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("expected not available after reconfigure with empty credentials")
	}
}

func TestSupportedCurrencies(t *testing.T) {
	p := newProvider()
	currencies := p.SupportedCurrencies()
	if len(currencies) == 0 {
		t.Fatal("SupportedCurrencies() returned empty slice")
	}

	found := false
	for _, c := range currencies {
		if c == currency.USD {
			found = true
			break
		}
	}
	if !found {
		t.Error("SupportedCurrencies() does not include USD")
	}
}

func TestSupportedCurrencies_ContainsExpected(t *testing.T) {
	p := newProvider()
	currencies := p.SupportedCurrencies()

	expected := []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.CAD, currency.AUD}
	for _, want := range expected {
		found := false
		for _, got := range currencies {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedCurrencies() missing %s", want)
		}
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ processor.PaymentProcessor = newProvider()
}
