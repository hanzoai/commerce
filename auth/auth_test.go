package auth

import (
	"appengine/aetest"
	"testing"
	"github.com/gin-gonic/gin"
)

func TestNewUser(t *testing.T) {
	c, err := aetest.NewContext(nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	
	NewUser(, f)
}
