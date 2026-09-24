// Package couponredemption is the once-per-user redemption guard for coupons.
//
// Each CouponRedemption is one immutable fact: "user U redeemed coupon C". The
// storage id is DeterministicID(couponCode, userId) and the model registers
// WithStringKey, so a second redeem by the same user for the same coupon
// resolves to the SAME row via the backend ON CONFLICT(id, kind, namespace)
// upsert — the guard cannot be bypassed.
//
// This replaces a guard that queried the credit grant by Tags="coupon:CODE"
// while the grant was written Tags="promo,coupon:CODE": the datastore Tags=
// filter is exact-string, so it never matched and the same user re-minted a
// fresh spendable grant on every submit. A structured redemption record, keyed
// by (code, user), makes the check exact and forgery-proof.
package couponredemption

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[CouponRedemption]("coupon-redemption", orm.WithStringKey[CouponRedemption]())
}

// CouponRedemption records that a user redeemed a coupon. One row per
// (CouponCode, UserId); the deterministic id makes a duplicate submit a no-op
// replay rather than a second mint.
type CouponRedemption struct {
	mixin.Model[CouponRedemption]

	CouponCode string `json:"couponCode"`
	UserId     string `json:"userId"`
}

func (r *CouponRedemption) Load(ps []datastore.Property) error {
	return datastore.LoadStruct(r, ps)
}

func (r *CouponRedemption) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(r)
}

// DeterministicID derives the stable storage id for a (couponCode, userId)
// pair. Same inputs → same id → ON CONFLICT dedup at the storage layer.
// Prefixed so the id is human-recognizable in logs/audits.
func DeterministicID(couponCode, userId string) string {
	sum := sha256.Sum256([]byte(couponCode + "\x00" + userId))
	return "cpr_" + hex.EncodeToString(sum[:16])
}

func New(db *datastore.Datastore) *CouponRedemption {
	r := new(CouponRedemption)
	r.Init(db)
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("coupon-redemption")
}
