package square

import (
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
)

func TestAPayerIsSplitIntoGivenAndFamilyNameWithItsAddress(t *testing.T) {
	g, f := payerName("Ada Lovelace King")
	if *g != "Ada" || *f != "Lovelace King" {
		t.Fatalf("name = %q %q", *g, *f)
	}
	if g, f := payerName("  "); g != nil || f != nil {
		t.Fatal("an empty name must send nothing")
	}
	a := payerAddress(&processor.PayerAddress{Line1: "1 Market St", City: "San Francisco", State: "CA", Postal: "94115", Country: "us"})
	if *a.AddressLine1 != "1 Market St" || *a.Locality != "San Francisco" || *a.PostalCode != "94115" || string(*a.Country) != "US" {
		t.Fatalf("address = %+v", a)
	}
	if payerAddress(nil) != nil || payerAddress(&processor.PayerAddress{}) != nil {
		t.Fatal("an empty address must send nothing")
	}
}
