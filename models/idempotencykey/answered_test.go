package idempotencykey

import (
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// TestAnswered_OnlyACompletedGuard — Answered reads a guard without recording one, and
// answers only a guard that completed: none, one in flight, and one of another status
// carrying a body are all unanswered.
func TestAnswered_OnlyACompletedGuard(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	if _, ok := Answered(db, "charge:u1", "k1"); ok {
		t.Fatal("a guard never begun was answered")
	}
	rec, _, err := Begin(db, "charge:u1", "k1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Answered(db, "charge:u1", "k1"); ok {
		t.Fatal("a guard in flight was answered")
	}
	rec.Response = `{"transactionId":"tx_1"}`
	if err := rec.Put(); err != nil {
		t.Fatal(err)
	}
	if _, ok := Answered(db, "charge:u1", "k1"); ok {
		t.Fatal("a started guard carrying a body was answered")
	}
	if err := Complete(rec, `{"transactionId":"tx_1"}`); err != nil {
		t.Fatal(err)
	}
	if body, ok := Answered(db, "charge:u1", "k1"); !ok || body != `{"transactionId":"tx_1"}` {
		t.Fatalf("a completed guard answered %q, %v", body, ok)
	}
	if _, ok := Answered(db, "charge:u2", "k1"); ok {
		t.Fatal("another scope's guard was answered")
	}
}
