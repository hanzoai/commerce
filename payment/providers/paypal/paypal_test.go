package paypal

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

func newProvider() *Provider {
	return &Provider{
		BaseProcessor: processor.NewBaseProcessor(processor.PayPal, supportedCurrencies()),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func TestType(t *testing.T) {
	p := newProvider()
	if got := p.Type(); got != processor.PayPal {
		t.Errorf("Type() = %q, want %q", got, processor.PayPal)
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
	p.Configure(Config{ClientID: "", ClientSecret: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with empty credentials, want false")
	}
}

func TestIsAvailable_PartialCredentials(t *testing.T) {
	p := newProvider()
	p.Configure(Config{ClientID: "id", ClientSecret: ""})
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with partial credentials, want false")
	}
}

func TestConfigure_Valid(t *testing.T) {
	p := newProvider()
	p.Configure(Config{ClientID: "client-id", ClientSecret: "client-secret"})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after valid Configure(), want true")
	}
}

func TestConfigure_WithSandbox(t *testing.T) {
	p := newProvider()
	p.Configure(Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Sandbox:      true,
	})
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after Configure() with sandbox, want true")
	}
}

func TestConfigure_Reconfigure(t *testing.T) {
	p := newProvider()
	p.Configure(Config{ClientID: "id1", ClientSecret: "secret1"})
	if !p.IsAvailable(context.Background()) {
		t.Error("expected available after first configure")
	}
	p.Configure(Config{ClientID: "", ClientSecret: ""})
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

	// USD must be supported.
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

	expected := []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.JPY}
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
