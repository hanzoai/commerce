package order

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/types/productcachedvalues"
)

// testOrder builds an Order with just enough datastore for the log context these
// calculations take; none of them read or write an entity.
func testOrder(items ...lineitem.LineItem) *Order {
	o := New(&datastore.Datastore{Context: context.Background()})
	o.Items = items
	return o
}

func item(price currency.Cents, qty int) lineitem.LineItem {
	return lineitem.LineItem{
		Quantity: qty,
		ProductCachedValues: productcachedvalues.ProductCachedValues{
			Price: price, Taxable: true,
		},
	}
}

func pctCoupon(amount int) coupon.Coupon {
	return coupon.Coupon{Type: coupon.Percent, Amount: amount, Enabled: true}
}

// TestCalcCouponDiscountRounds pins the rule on a percentage coupon over the whole order.
//
// `old` is what the truncating arithmetic returned before: floor(price * pct * 0.01),
// summed per line. Every case where old != want is a cent the merchant kept.
func TestCalcCouponDiscountRounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		price currency.Cents
		pct   int
		want  currency.Cents
		old   currency.Cents
	}{
		{"19.99 @ 10%", 1999, 10, 200, 199},   // 1.999
		{"19.99 @ 15%", 1999, 15, 300, 299},   // 2.9985
		{"19.99 @ 33%", 1999, 33, 660, 659},   // 6.5967
		{"9.95 @ 10%", 995, 10, 100, 99},      // 0.995 — exactly a half
		{"9.95 @ 15%", 995, 15, 149, 149},     // 1.4925 — already correct
		{"9.95 @ 33%", 995, 33, 328, 328},     // 3.2835
		{"33.33 @ 10%", 3333, 10, 333, 333},   // 3.333
		{"33.33 @ 15%", 3333, 15, 500, 499},   // 4.9995
		{"33.33 @ 33%", 3333, 33, 1100, 1099}, // 10.9989
		{"0.29 @ 10%", 29, 10, 3, 2},          // 0.029
		{"0.29 @ 15%", 29, 15, 4, 4},          // 0.0435
		{"0.29 @ 33%", 29, 33, 10, 9},         // 0.0957
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := testOrder(item(tc.price, 1))
			o.Coupons = []coupon.Coupon{pctCoupon(tc.pct)}
			o.CouponCodes = []string{""}
			if got := o.CalcCouponDiscount(); got != tc.want {
				t.Errorf("CalcCouponDiscount() = %d, want %d (the truncating arithmetic gave %d)",
					got, tc.want, tc.old)
			}
		})
	}
}

// TestCalcItemCouponDiscountRounds pins the same rule on a coupon scoped to one item, which
// is a different branch of CalcCouponDiscount.
func TestCalcItemCouponDiscountRounds(t *testing.T) {
	li := item(1999, 1)
	li.ProductId = "prod-1"
	o := testOrder(li)
	c := pctCoupon(10)
	c.ProductId = "prod-1"
	o.Coupons = []coupon.Coupon{c}
	o.CouponCodes = []string{""}

	if got, want := o.CalcCouponDiscount(), currency.Cents(200); got != want {
		t.Errorf("item coupon CalcCouponDiscount() = %d, want %d (truncating gave 199)", got, want)
	}
}

// TestCalcItemCouponTaxableDiscountMatchesDiscount — the taxable base reduction and the
// discount itself are the same money computed twice. If they round differently the tax is
// charged on an amount the customer was never billed.
func TestCalcItemCouponTaxableDiscountMatchesDiscount(t *testing.T) {
	for _, price := range []currency.Cents{1999, 995, 3333, 29, 1, 7, 12345} {
		for _, pct := range []int{10, 15, 33, 7, 50} {
			li := item(price, 1)
			li.ProductId = "prod-1"
			o := testOrder(li)
			c := pctCoupon(pct)
			c.ProductId = "prod-1"
			o.Coupons = []coupon.Coupon{c}
			o.CouponCodes = []string{""}

			discount := o.CalcCouponDiscount()
			taxable := o.CalcItemCouponTaxableDiscount()
			if discount != taxable {
				t.Errorf("price=%d pct=%d: discount %d != taxable discount %d",
					price, pct, discount, taxable)
			}
		}
	}
}

// TestOrderCouponDoesNotDriftFromOrderTotal is the drift the two code paths could have.
//
// CalcCouponDiscount walks the line items; OrderDB.ApplyCoupon takes the same percentage
// off LineTotal in one step. They are the same coupon on the same order and they must
// return the same number. Rounding each line independently and adding the results does NOT
// give that — floor or round, a per-line sum can differ from the whole-order figure by up
// to one minor unit per line — so an ORDER-LEVEL percentage coupon sums its base first and
// rounds once.
func TestOrderCouponDoesNotDriftFromOrderTotal(t *testing.T) {
	cart := []currency.Cents{1999, 995, 3333, 29}
	for _, pct := range []int{10, 15, 33, 7, 1, 50, 99} {
		items := make([]lineitem.LineItem, len(cart))
		var lineTotal currency.Cents
		for i, p := range cart {
			items[i] = item(p, 1)
			lineTotal += p
		}
		o := testOrder(items...)
		o.Coupons = []coupon.Coupon{pctCoupon(pct)}
		o.CouponCodes = []string{""}
		perLine := o.CalcCouponDiscount()

		odb := &OrderDB{LineTotal: lineTotal}
		if err := odb.ApplyCoupon(pctCoupon(pct)); err != nil {
			t.Fatalf("ApplyCoupon: %v", err)
		}

		if perLine != odb.Discount {
			t.Errorf("%d%% on %v: CalcCouponDiscount()=%d but ApplyCoupon gave %d (drift %+d)",
				pct, cart, perLine, odb.Discount, perLine-odb.Discount)
		}
	}
}

// TestItemCouponRoundsPerLine states the deliberate exception: a coupon scoped to an item
// is a discount on THAT item, so it rounds per line and CalcItemCouponTaxableDiscount can
// agree with it line for line. Two 19.99 items at 10% is 2.00 twice, not 4.00 off 39.98
// (which is the same here, but the rule is per-line and the test says so).
func TestItemCouponRoundsPerLine(t *testing.T) {
	a, b := item(1999, 1), item(995, 1)
	a.ProductId, b.ProductId = "prod-1", "prod-1"
	o := testOrder(a, b)
	c := pctCoupon(10)
	c.ProductId = "prod-1"
	o.Coupons = []coupon.Coupon{c}
	o.CouponCodes = []string{""}

	// 1999 @ 10% = 200 (from 199.9), 995 @ 10% = 100 (from 99.5). Per line: 300.
	if got, want := o.CalcCouponDiscount(), currency.Cents(300); got != want {
		t.Errorf("per-line item coupon = %d, want %d", got, want)
	}
}

// TestFlatCouponUnchanged — only the percentage arithmetic moved; flat and free-shipping
// coupons must be untouched.
func TestFlatCouponUnchanged(t *testing.T) {
	o := testOrder(item(1999, 3))
	o.Coupons = []coupon.Coupon{{Type: coupon.Flat, Amount: 500, Enabled: true}}
	o.CouponCodes = []string{""}
	if got, want := o.CalcCouponDiscount(), currency.Cents(500); got != want {
		t.Errorf("flat order coupon = %d, want %d", got, want)
	}

	li := item(1999, 3)
	li.ProductId = "prod-1"
	o2 := testOrder(li)
	o2.Coupons = []coupon.Coupon{{Type: coupon.Flat, Amount: 500, Enabled: true, ProductId: "prod-1"}}
	o2.CouponCodes = []string{""}
	// Flat item coupon is per unit: quantity 3 x 500.
	if got, want := o2.CalcCouponDiscount(), currency.Cents(1500); got != want {
		t.Errorf("flat item coupon = %d, want %d", got, want)
	}
}
