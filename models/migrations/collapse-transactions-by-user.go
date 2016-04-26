package migrations

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/models/transaction"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("collapse-transactions-by-user",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")
		return NoArgs
	},
	func(db *ds.Datastore, trans *transaction.Transaction) {
		ctx := db.Context
		userid := trans.UserId

		// Look up user for this order
		usr := user.New(db)
		if err := usr.GetById(userid); err != nil {
			log.Warn("Failed to query for user: %v", userid, ctx)
			return
		}

		// Look for 'deleted' emails
		if !strings.HasPrefix(usr.Email, "!______") {
			return
		}

		// If deleted email, then find the cannonical one
		usr2 := user.New(db)
		if err := usr2.GetByEmail(strings.Replace(usr.Email, "!______", "", 1)); err != nil {
			log.Error("Failed to query for cannonical user: %v", userid, ctx)
			return
		}

		log.Warn("Fixing Transaction: %v => %v", usr.Email, usr2.Email, ctx)
		trans.UserId = usr2.Id()
		trans.Put()
	},
)
