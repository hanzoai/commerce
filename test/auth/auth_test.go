package test

import (
	"errors"
	"reflect"
	"testing"

	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"github.com/gin-gonic/gin"

	"appengine/aetest"
)

const kind = "user"

func TestNewUser(t *testing.T) {
	t.Skip()
	ctx, err := aetest.NewContext(nil)
	defer ctx.Close()
	if err != nil {
		t.Error(err)
	}

	c := &gin.Context{}
	c.Set("appengine", ctx)

	regForm := auth.RegistrationForm{
		User:     models.User{Email: "e@example.com"},
		Password: "hunter2",
	}
	regForm.User.Id = regForm.User.Email

	err = auth.NewUser(c, &regForm)
	if err != nil {
		t.Error(err)
	}

	db := datastore.New(ctx)
	user := new(models.User)
	err = db.GetKey(kind, regForm.User.Email, user)
	if err != nil {
		t.Error(err)
	}

	if reflect.DeepEqual(*user, models.User{}) {
		t.Error(errors.New("User is empty"))
	}

	if user.Id != regForm.User.Id {
		t.Logf("User id is not valid \n\tExpected: %s \n\tActual: %s", regForm.User.Id, user.Id)
		t.Fail()
	}
}
