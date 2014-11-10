package auth

import (
	"appengine/aetest"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewUser(t *testing.T) {
	ctx, err := aetest.NewContext(nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	c := &gin.Context{}
	c.Set("appengine", ctx)

	userId := "AzureDiamond"
	password := "hunter2"

	f := models.RegistrationForm{
		Email:    userId,
		Password: password,
	}

	err = NewUser(c, f)
	if err != nil {
		t.Error(err)
	}

	db := datastore.New(ctx)
	var user models.User
	err = db.GetKey("user", userId, user)

	if err != nil {
		t.Error(err)
	}

	if user == nil {
		t.Error(errors.New("User is nil"))
		t.FailNow()
	}

	if user.Id != userId {
		t.Logf("User id is not valid \n\tExpected: %s \n\tActual: %s", userId, user.Id)
		t.Fail()
	}

	if user.PasswordHash != f.PasswordHash() {
		t.Logf("User id is not valid \n\tExpected: %s \n\tActual: %s", user.PasswordHash, f.PasswordHash())
		t.Fail()
	}
}
