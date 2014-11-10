package test

import (
	"appengine/aetest"
	"crowdstart.io/datastore"
	"github.com/gin-gonic/gin"
	"testing"
)

type TestStruct struct {
	Field string
}

func TestCRUD(t *testing.T) {
	ctx := aetest.NewContext(nil)
	db := datastore.New(ctx)

	oPut := TestStruct{"eqhwikas"}
	key, err := db.Put("test", oPut)
	if err != nil {
		t.Error(err)
	}

	var oGet TestStruct
	err = db.Get(key, oGet)

	if err != nil {
		t.Error(err)
	}

	if oGet != oPut {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oPut, oGet)
		t.Fail()
	}

	oModified := TestStruct{"jaks"}
	key, err = db.Update(key, oModified)
	if err != nil {
		t.Error(err)
	}

	err = db.Get(key, oGet)
	if err != nil {
		t.Error(err)
	}

	if oModified != oGet {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oModified, oGet)
		t.Fail()
	}

	err = db.Delete(key)

	if err != nil {
		t.Error(err)
	}

	err = db.Get(key, oGet)
	if err == nil {
		t.Logf("db.Get worked even though the entry was removed \n\t %#v", oGet)
		t.Fail()
	}
}

// Tests all the Key functions
func TestKeyCRUD(t *testing.T) {
	ctx := aetest.NewContext(nil)
	db := datastore.New(ctx)

	key := "testkey"
	oPut := TestStruct{"hjaks"}

	t.Logf("The key is %s", key)

	key, err := db.PutKey("test", key, oPut)
	if err != nil {
		t.Error(err)
	}

	var oGet TestStruct
	err = db.GetKey("test", key, oGet)
	if err != nil {
		t.Error(err)
	}
	if oGet != oPut {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oPut, oGet)
		t.Fail()
	}

	oModified := TestStruct{"jaks"}
	key, err = db.Update(key, oModified)
	if err != nil {
		t.Error(err)
	}

	err = db.GetKey("test", key, oGet)
	if err != nil {
		t.Error(err)
	}
	if oGet != oModified {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oModified, oGet)
		t.Fail()
	}

	err = db.Delete(key)

	if err != nil {
		t.Error(err)
	}

	err = db.GetKey("test", key, oGet)
	if err == nil {
		t.Logf("db.Get worked even though the entry was removed \n\t %#v", oGet)
		t.Fail()
	}
}
