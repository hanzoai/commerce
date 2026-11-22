package billing

import (
	"reflect"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// Every response of the agentic and bank surface speaks camelCase JSON: an exported field with
// no json tag would go out under its Go name. The types an endpoint answers with are walked,
// fields of fields included, and each exported field must be tagged.
func TestEveryExportedFieldOfTheAgenticAndBankResponsesIsTagged(t *testing.T) {
	roots := []any{
		CatalogOut{}, CreditPack{}, PaymentOption{}, Discovery{}, CryptoTopupOut{}, CheckoutView{}, OrderView{},
		NextAction{}, AgentError{}, TokenRoom{}, PayAction{}, AgentPayment{}, WireCredit{}, ItemIn{}, PaymentIn{},
		WireInstructions{}, TopupIntentOut{}, TopupIntentState{}, InvoicePayment{}, AuthenticationStep{},
		processor.Grant{}, processor.StablecoinReady{}, processor.StablecoinDeposit{}, processor.DepositAddress{}, processor.DepositToken{},
		routing.CryptoOffer{}, routing.Quote{},
	}
	seen := map[reflect.Type]bool{}
	var walk func(ty reflect.Type, path string)
	walk = func(ty reflect.Type, path string) {
		for ty.Kind() == reflect.Pointer || ty.Kind() == reflect.Slice || ty.Kind() == reflect.Array || ty.Kind() == reflect.Map {
			ty = ty.Elem()
		}
		if ty.Kind() != reflect.Struct || seen[ty] || ty.PkgPath() == "time" {
			return
		}
		seen[ty] = true
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			if !f.IsExported() {
				continue
			}
			if f.Anonymous {
				walk(f.Type, path)
				continue
			}
			tag, ok := f.Tag.Lookup("json")
			if !ok {
				t.Errorf("%s.%s has no json tag", path, f.Name)
				continue
			}
			if tag == "-" {
				continue
			}
			walk(f.Type, path+"."+f.Name)
		}
	}
	for _, r := range roots {
		walk(reflect.TypeOf(r), reflect.TypeOf(r).String())
	}
}
