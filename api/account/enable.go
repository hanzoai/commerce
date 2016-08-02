package account

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

func enable(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	usr := user.New(db)
	tok := token.New(db)

	// Get Token
	id := c.Params.ByName("tokenid")
	if err := tok.GetById(id); err != nil {
		panic(err)
	}

	// Get user associated with token
	if err := usr.GetById(tok.UserId); err != nil {
		panic(err)
	}

	if tok.Expired() || tok.Used {
		http.Fail(c, 403, "Token expired", errors.New("Token expired"))
		return
	}

	// Set user as enabled
	usr.Enabled = true
	if err := usr.Put(); err != nil {
		http.Fail(c, 500, "Failed to enable user", err)
		return
	}

	// Save token
	tok.Used = true
	if err := tok.Put(); err != nil {
		log.Warn("Unable to update token", err, c)
	}

	// Send account confirmed email
	ctx := middleware.GetAppEngine(c)
	emails.SendEmailConfirmedEmail(ctx, org, usr)

	http.Render(c, 200, gin.H{"status": "ok"})
}
