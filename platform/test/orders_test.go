package test

import (
	"appengine"
	"appengine/aetest"
	"crowdstart.io/datastore"
	"testing"
)

func TestOrders(t *testing.T) {
	c, err := aetest.NewContext(nil)
	defer c.Close()
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	orders, err := Orders(c, "AzureDiamond")

	if err != nil {
		t.Error(err)
		t.Fail()
	}

	if orders == nil {
		t.Error("orders are nil")
		t.Fail()
	}

	if len(orders) < 1 {
		t.Error("orders slice is empty")
	}
}
