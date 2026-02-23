package recurly

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
		BaseProcessor: processor.NewBaseProcessor(processor.Recurly, supportedCurrencies()),
		client:        &http.Client{Timeout: 30 * time.Second},
	}
}

func TestType(t *testing.T) {
	p := newProvider()
	if got := p.Type(); got != processor.Recurly {
		t.Errorf("Type() = %q, want %q", got, processor.Recurly)
	}
}

func TestIsAvailable_NotConfigured(t *testing.T) {
	p := newProvider()
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true for unconfigured provider, want false")
	}
}

func TestIsAvailable_EmptyAPIKey(t *testing.T) {
	p := newProvider()
	p.Configure("")
	if p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = true with empty API key, want false")
	}
}

func TestConfigure_Valid(t *testing.T) {
	p := newProvider()
	p.Configure("test-api-key")
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after valid Configure(), want true")
	}
}

func TestConfigure_WithSubdomain(t *testing.T) {
	p := newProvider()
	p.Configure("test-api-key", "mycompany")
	if !p.IsAvailable(context.Background()) {
		t.Error("IsAvailable() = false after Configure() with subdomain, want true")
	}
}

func TestConfigure_Reconfigure(t *testing.T) {
	p := newProvider()
	p.Configure("key1")
	if !p.IsAvailable(context.Background()) {
		t.Error("expected available after first configure")
	}
	p.Configure("")
	if p.IsAvailable(context.Background()) {
		t.Error("expected not available after reconfigure with empty key")
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

	expected := []currency.Type{currency.USD, currency.EUR, currency.GBP, currency.CAD, currency.JPY}
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
