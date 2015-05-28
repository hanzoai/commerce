package search

import (
	"fmt"

	"appengine/search"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json/http"
)

func searchUser(c *gin.Context) {
	q := c.Request.URL.Query().Get("q")

	u := user.User{}
	index, err := search.Open(u.Kind())
	if err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to find index 'user'"), err)
		return
	}

	db := datastore.New(c)

	users := make([]*user.User, 0)
	for t := index.Search(db.Context, q, nil); ; {
		var doc user.Document
		id, err := t.Next(&doc)
		if err == search.Done {
			break
		}
		if err != nil {
			http.Fail(c, 404, fmt.Sprintf("Failed to search index 'user' %v", err), err)
			break
		}

		u := user.New(db)
		err = u.GetById(id)
		if err != nil {
			continue
		}

		users = append(users, u)
	}

	http.Render(c, 200, users)
}
