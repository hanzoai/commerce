package test

import (
	"appengine/aetest"
	"crowdstart.io/datastore"
	"testing"
)

type TestStruct struct {
	Field string
}

func TestCRUD(t *testing.T) {
	ctx, _ := aetest.NewContext(nil)
	defer ctx.Close()
	db := datastore.New(ctx)

	// Put
	oPut := TestStruct{"eqhwikas"}
	key, err := db.Put("test", &oPut)
	if err != nil {
		t.Error(err)
	}

	// Get
	var oGet TestStruct
	err = db.Get(key, &oGet)
	if err != nil {
		t.Error(err)
	}
	if oGet != oPut {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oPut, oGet)
		t.Fail()
	}

	// Update
	oModified := TestStruct{"jaks"}
	key, err = db.PutKey("test", key, &oModified)
	if err != nil {
		t.Error(err)
	}
	err = db.Get(key, &oGet)
	if err != nil {
		t.Error(err)
	}
	if oModified != oGet {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oModified, oGet)
		t.Fail()
	}

	// Delete
	err = db.Delete(key)
	if err != nil {
		t.Error(err)
	}
	err = db.Get(key, &oGet)
	if err == nil {
		t.Logf("db.Get worked even though the entry was removed \n\t %#v", oGet)
		t.Fail()
	}
}

// Tests all the Key functions
func TestKeyCRUD(t *testing.T) {
	ctx, _ := aetest.NewContext(nil)
	defer ctx.Close()
	db := datastore.New(ctx)

	kind := "testkeystruct"
	key := "TestKeyCRUD"

	oPut := TestStruct{"hjaks"}
	_, err := db.PutKey(kind, key, &oPut)
	t.Logf("The key is %s", key)
	if err != nil {
		t.Error(err)
	}

	var oGet TestStruct
	err = db.GetKey(kind, key, &oGet)
	if err != nil {
		t.Error(err)
	}
	if oGet != oPut {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oPut, oGet)
		t.Fail()
	}

	oModified := TestStruct{"jaks"}
	_, err = db.PutKey(kind, key, &oModified)
	if err != nil {
		t.Error(err)
	}
	err = db.GetKey(kind, key, &oGet)
	if err != nil {
		t.Error(err)
	}
	if oGet != oModified {
		t.Logf("Object is not valid. \n\t Expected: %#v \n\t Actual: %#v", oModified, oGet)
		t.Fail()
	}
	// if key != returnedKey {
	// 	t.Logf("Returned key != key \n\t Expected %#v \n\t Actual: %#v", key, returnedKey)
	// }

	err = db.Delete(key)
	if err != nil {
		t.Error(err)
	}
	err = db.GetKey(kind, key, &oGet)
	if err == nil {
		t.Logf("Deletion of key did not work \n\t %#v", oGet)
		t.Fail()
	}
}

func TestMultiCRUD(t *testing.T) {
	ctx, _ := aetest.NewContext(nil)
	defer ctx.Close()
	db := datastore.New(ctx)

	kind := "TestMultiCRUD"

	oPut := make([]interface{}, 10)
	for i := 0; i < len(oPut); i++ {
		oPut[i] = interface{}(TestStruct{"i"})
	}
	t.Log(oPut)
	keys, err := db.PutMulti(kind, oPut)
	if err != nil {
		t.Error(err)
	}
	if len(keys) != len(oPut) {
		t.Logf("Wrong number of keys returned \n\t Expected: %d \n\t Actual: %d", len(oPut), len(keys))
		t.Fail()
	}

	oGet := make([]interface{}, 0)
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
	oModified := make([]interface{}, 10)
	var _keys []string
	for i := 0; i < 10; i++ {
		oModified[i] = TestStruct{"j"}
	}
	for i := range keys {
		key, err := db.Put(keys[i], oModified[i])
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

	if !identicalSlices(oGet, oModified) {
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

func identicalSlices(x, y []interface{}) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func TestEqualSlices(t *testing.T) {
	equalA := make([]interface{}, 10)
	equalB := make([]interface{}, 10)

	for i := range equalA {
		equalA[i] = string(i)
		equalB[i] = string(i)
	}

	if res := identicalSlices(equalA, equalB); !res {
		t.Logf("Comparison of identical slices \n\t Expected: %#v \n\t Actual: %#v", true, res)
		t.Fail()
	}

	diffLengthA := make([]interface{}, 2)
	diffLengthB := make([]interface{}, 1)

	if res := identicalSlices(diffLengthA, diffLengthB); res {
		t.Logf("Comparison of slices with different lengths \n\t Expected: %#v \n\t Actual: %#v", false, res)
		t.Fail()
	}

	diffA := make([]interface{}, 10)
	diffB := make([]interface{}, 10)

	for i := range diffA {
		diffA[i] = string(i + 1)
		diffB[i] = string(i)
	}

	if res := identicalSlices(diffA, diffB); !res {
		t.Logf("Comparison of slices with different elements \n\t Expected: %#v \n\t Actual: %#v", false, res)
		t.Fail()
	}
}
