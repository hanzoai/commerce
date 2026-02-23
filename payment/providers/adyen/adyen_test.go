package adyen

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

func newProvider() *Provider {
	return &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.Adyen, supportedCurrencies()),
	}
}

func TestType(t *testing.T) {
	p := newProvider()
	if got := p.Type(); got != processor.Adyen {
		t.Errorf("Type() = %q, want %q", got, processor.Adyen)
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
	p.Configure(Config{APIKey: "", MerchantAccount: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with empty credentials, want false")
	}
}

func TestIsAvailable_PartialCredentials_APIKeyOnly(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "key", MerchantAccount: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with only API key, want false")
	}
}

func TestIsAvailable_PartialCredentials_MerchantOnly(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "", MerchantAccount: "merchant"})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with only merchant account, want false")
	}
}

func TestConfigure_Valid(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "test-key", MerchantAccount: "TestMerchant"})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after valid Configure(), want true")
	}
}

func TestConfigure_LiveEnvironment(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		APIKey:          "live-key",
		MerchantAccount: "LiveMerchant",
		LiveURLPrefix:   "1797a841fbb37ca7-Demo",
		Environment:     Live,
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after live Configure(), want true")
	}
}

func TestConfigure_Reconfigure(t *testing.T) {
	p := newProvider()
	p.Configure(Config{APIKey: "key", MerchantAccount: "merchant"})
	if !p.IsAvailable(context.Background()) {
		t.Error("expected available after first configure")
	}
	p.Configure(Config{APIKey: "", MerchantAccount: ""})
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

	expected := []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.JPY, currency.KRW}
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
