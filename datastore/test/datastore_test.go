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
		t.Logf("Deletion of key did not work \n\t %#v", oGet)
		t.Fail()
	}
}

func TestMultiCRUD(t *testing.T) {
	var oPut [10]TestStruct
	for i := 0; i < 10; i++ {
		oPut[i] = TestStruct{string(i * 2)}
	}

	ctx := aetest.NewContext(nil)
	db := datastore.New(ctx)

	keys, err := db.PutMulti("test", oPut)
	if err != nil {
		t.Error(err)
	}

	if len(keys) != len(oPut) {
		t.Logf("Wrong number of keys returned \n\t Expected: %d \n\t Actual: %d", len(oPut), len(keys))
		t.Fail()
	}

	var oGet [len(oPut)]TestStruct
	err = db.GetMulti(keys, oGet)
	if err != nil {
		t.Error(err)
	}

	empty := TestStruct{}
	for i := 0; i < len(oGet); i++ {
		if oGet[i] == empty {
			t.Logf("Some objects are empty \n\t Expected: %#v \n\t Actual: %#v", oPut, oGet)
			t.Fail()
			break
		}
	}

	// Update
	var oModified [len(oPut)]TestStruct
	var _keys []string
	for i := 0; i < 10; i++ {
		oModified[i] = TestStruct{string(i * 3)}
	}
	for i := range keys {
		key, err := db.Update(keys[i], oModified[i])
		_keys = append(_keys, key)
		if err != nil {
			t.Error(err)
		}
	}

	keys = _keys

	err = db.GetMulti(keys, oGet)
	if err != nil {
		t.Error(err)
	}

	if oGet != oModified {
		t.Logf("Update of multiple keys did not work \n\t Expected: %#v \n\t Actual: %#v", oModified, oGet)
		t.Fail()
	}

	// Delete
	for i := range keys {
		err = db.Delete(keys[i])
		if err != nil {
			t.Error(err)
		}
	}

	err = db.GetMulti(keys, oGet)
	if err == nil {
		t.Logf("Deletion of multiple keys did not work. \n\t %#v", oGet)
		t.Fail()
	}
}
