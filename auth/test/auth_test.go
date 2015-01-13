package test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"

	"appengine/aetest"
)

var mockRegForm = struct {
	User     models.User
	Password string
}{
	models.User{Id: "AzureDiamond"},
	"hunter2",
}

func TestNewUser(t *testing.T) {
	t.Skip()
	println("ctx")
	ctx, err := aetest.NewContext(nil)
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	c := &gin.Context{}
	c.Set("appengine", ctx)

	f := auth.RegistrationForm{
		User:     mockRegForm.User,
		Password: mockRegForm.Password,
	}

	err = auth.NewUser(c, &f)
	if err != nil {
		t.Error(err)
	}
	println("New user")

	db := datastore.New(ctx)
	var user models.User
	err = db.Get(mockRegForm.User.Id, user)
	if err != nil {
		t.Error(err)
	}
	println("Get user")

	if reflect.DeepEqual(user, models.User{}) {
		t.Error(errors.New("User is empty"))
		t.Fail()
	}

	if user.Id != mockRegForm.User.Id {
		t.Logf("User id is not valid \n\tExpected: %s \n\tActual: %s", mockRegForm.User.Id, user.Id)
		t.Fail()
	}

	hash, _ := f.PasswordHash()
	if !bytes.Equal(user.PasswordHash, hash) {
		t.Logf("User password hash is not valid \n\tExpected: %s \n\tActual: %s", user.PasswordHash, hash)
		t.Fail()
	}
}
