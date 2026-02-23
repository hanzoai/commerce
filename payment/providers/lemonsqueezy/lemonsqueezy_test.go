package lemonsqueezy

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

func newProvider() *Provider {
	return &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.LemonSqueezy, supportedCurrencies()),
	}
}

func TestType(t *testing.T) {
	p := newProvider()
	if got := p.Type(); got != processor.LemonSqueezy {
		t.Errorf("Type() = %q, want %q", got, processor.LemonSqueezy)
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
	p.Configure(Config{APIKey: "", StoreID: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with empty credentials, want false")
	}
}

func TestIsAvailable_PartialCredentials_APIKeyOnly(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "key", StoreID: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with only API key, want false")
	}
}

func TestIsAvailable_PartialCredentials_StoreIDOnly(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "", StoreID: "store"})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with only store ID, want false")
	}
}

func TestConfigure_Valid(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "test-key", StoreID: "store-123"})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after valid Configure(), want true")
	}
}

func TestConfigure_WithWebhookSecret(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		APIKey:        "test-key",
		StoreID:       "store-123",
		WebhookSecret: "whsec_test",
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after Configure() with webhook secret, want true")
	}
}

func TestConfigure_WithDefaultVariant(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		APIKey:           "test-key",
		StoreID:          "store-123",
		DefaultVariantID: "variant-456",
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after Configure() with default variant, want true")
	}
}

func TestConfigure_Reconfigure(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "key", StoreID: "store"})
	if !p.IsAvailable(context.Background()) {
		t.Error("expected available after first configure")
	}
	p.Configure(Config{APIKey: "", StoreID: ""})
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

	expected := []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.BRL}
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

func TestSupportedCurrencies_Count(t *testing.T) {
	p := newProvider()
	currencies := p.SupportedCurrencies()
	if len(currencies) != 7 {
		t.Errorf("SupportedCurrencies() returned %d currencies, want 7", len(currencies))
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ processor.PaymentProcessor = newProvider()
}
