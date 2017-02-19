package dashv2

import (
	"errors"
	"sort"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type organizationRes struct {
	Id               string `json:"id"`
	Name             string `json:"name"`
	FullName         string `json:"fullName"`
	LiveSecretKey    string `json:"live-secret-key"`
	LivePublishKey   string `json:"live-published-key"`
	TestSecretKey    string `json:"test-secret-key"`
	TestPublishedKey string `json:"test-published-key"`
}

type loginRes struct {
	User          user.User         `json:"user"`
	Organizations []organizationRes `json:"organizations"`
}

func login(c *gin.Context) {
	req := &loginReq{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	db := datastore.New(c)

	// Get user by email
	usr := user.New(db)
	if err := usr.GetByEmail(req.Email); err != nil {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		return
	}

	// Check user's password
	if !password.HashAndCompare(usr.PasswordHash, req.Password) {
		http.Fail(c, 401, "Email or password is incorrect", errors.New("Email or password is incorrect"))
		log.Debug("Incorrect password", c)
		return
	}

	var orgs []*organization.Organization

	orgIds := usr.Organizations
	for _, orgId := range orgIds {
		org := organization.New(db)
		err := org.GetById(orgId)
		if err != nil {
			log.Error("Could not get Organization with Error %v", err, c)
			continue
		}
		orgs = append(orgs, org)
	}

	// Sort organizations by name
	sort.Sort(organization.ByName(orgs))

	res := loginRes{
		User:          *usr,
		Organizations: make([]organizationRes, len(orgs)),
	}

	for i, org := range orgs {
		res.Organizations[i] = organizationRes{
			Id:               org.Id(),
			Name:             org.Name,
			FullName:         org.FullName,
			LiveSecretKey:    org.MustGetTokenByName("live-secret-key").String(),
			LivePublishKey:   org.MustGetTokenByName("live-published-key").String(),
			TestSecretKey:    org.MustGetTokenByName("test-secret-key").String(),
			TestPublishedKey: org.MustGetTokenByName("test-published-key").String(),
		}
	}

	http.Render(c, 200, res)
	return
}
