package test

import (
	"appengine/aetest"
	"crowdstart.io/auth"
	"crowdstart.io/models"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

var mockUser = struct {
	Id       string
	Password string
}{
	"AzureDiamond",
	"hunter2",
}

func TestNewUser(t *testing.T) {
	ctx, err := aetest.NewContext(nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	c := &gin.Context{}
	c.Set("appengine", ctx)

	f := models.RegistrationForm{
		Email:    mockUser.Id,
		Password: mockUser.Password,
	}

	err = auth.NewUser(c, f)
	if err != nil {
		t.Error(err)
	}

	db := datastore.New(ctx)
	var user models.User
	err = db.GetKey("user", mockUser.Id, user)

	if err != nil {
		t.Error(err)
	}

	if user == nil {
		t.Error(errors.New("User is nil"))
		t.FailNow()
	}

	if user.Id != mockUser.Id {
		t.Logf("User id is not valid \n\tExpected: %s \n\tActual: %s", mockUser.Id, user.Id)
		t.Fail()
	}

	if user.PasswordHash != f.PasswordHash() {
		t.Logf("User password hash is not valid \n\tExpected: %s \n\tActual: %s", user.PasswordHash, f.PasswordHash())
		t.Fail()
	}
}
