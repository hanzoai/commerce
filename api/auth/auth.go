package auth

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware/oauthmiddleware"
	"github.com/hanzoai/commerce/models/oauthtoken"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// {
// 	"access_token":"2YotnFZFEjr1zCsicMWpAA",
// 	"token_type":"example",
// 	"expires_in":3600,
// 	"refresh_token":"tGzv3JOkF0XG5Qx2TlKWIA",
// 	"example_parameter":"example_value"
// }

type OAuthRequest struct {
	GrantType    string `json:"grant_type,omitempty"`
	Password     string `json:"password,omitempty"`
	Username     string `json:"username,omitempty"`
	ClientId     string `json:"client_id,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type OAuthResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func credentials(c *zip.Ctx) error {
	req := OAuthRequest{}
	grantType := ""

	if c.Header("content-type") == "application/json" {
		if err := json.DecodeBytes(c.Body(), &req); err != nil {
			log.Warn("Failed to decode request body: %v", err, c)
			return http.Fail(c, 400, "Failed to decode request body", nil)
		}

		grantType = req.GrantType
	} else {
		grantType = c.Fiber().FormValue("grant_type")
	}

	switch grantType {
	case "password":
		return passwordCredentials(c, req)
	case "refresh_token":
		return refreshCredentials(c, req)
	default:
		return http.Fail(c, 400, "Grant Type is not supported", nil)
	}
}

func passwordCredentials(c *zip.Ctx, req OAuthRequest) error {
	var username, pw, orgId string

	if req.GrantType == "" {
		username = c.Fiber().FormValue("username")
		pw = c.Fiber().FormValue("password")
		orgId = c.Fiber().FormValue("client_id")
	} else {
		username = req.Username
		pw = req.Password
		orgId = req.ClientId
		// scope := c.Query("scope")
	}

	if username == "" {
		return http.Fail(c, 400, "Username required", nil)
	}

	if pw == "" {
		return http.Fail(c, 400, "Password required", nil)
	}

	db := datastore.New(c.Context())
	org := organization.New(db)
	if err := org.GetById(orgId); err != nil {
		return http.Fail(c, 400, "Invalid organization", err)
	}

	usr := user.New(db)
	if err := usr.GetByEmail(username); err != nil {
		return http.Fail(c, 401, "User does not exist", err)
	}

	// Check user's password
	if !password.HashAndCompare(usr.PasswordHash, pw) {
		return http.Fail(c, 401, "Email or password is incorrect", errors.New("email or password is incorrect"))
	}

	// If user is not enabled fail
	if !usr.Enabled {
		return http.Fail(c, 401, "Account needs to be enabled", errors.New("account needs to be enabled"))
	}

	tok, ok, err := org.GetReferenceToken(usr)
	if !ok {
		// log.Warn("Reference token may have been revoked: %v", err, c)
		return http.Fail(c, 403, "Reference token may have been revoked", err)
	}

	expiresIn := tok.AccessPeriod * 3600
	aTokString, err := tok.IssueAccessToken(usr.Id(), org.SecretKey)
	if err != nil {
		return http.Fail(c, 403, "Could not issue access token", err)
	}

	rTokString, err := tok.IssueRefreshToken(usr.Id(), org.SecretKey)
	if err != nil {
		return http.Fail(c, 403, "Could not issue refresh token", err)
	}

	resp := OAuthResponse{
		AccessToken:  aTokString,
		RefreshToken: rTokString,
		ExpiresIn:    int(expiresIn),
		TokenType:    "jwt",
	}

	return http.Render(c, 200, resp)
}

func refreshCredentials(c *zip.Ctx, req OAuthRequest) error {
	var refreshToken string

	if req.GrantType == "" {
		refreshToken = c.Fiber().FormValue("refresh_token")
	} else {
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		return http.Fail(c, 400, "Refresh token required", nil)
	}

	org, _, claims, ok := oauthmiddleware.DecodeToken(c, refreshToken, func(claims oauthtoken.Claims) error {
		if claims.Type != oauthtoken.Refresh {
			return errors.New("token must be type refresh")
		}
		return nil
	})

	if !ok {
		return nil
	}

	db := datastore.New(c.Context())
	usr := user.New(db)
	if err := usr.GetById(claims.UserId); err != nil {
		return http.Fail(c, 403, "User does not exist", err)
	}

	tok, ok, err := org.GetReferenceToken(usr)
	if !ok {
		return http.Fail(c, 403, "Reference token may have been revoked", err)
	}

	if tok.Revoked {
		return http.Fail(c, 401, "Issuer has been revoked", nil)
	}

	expiresIn := tok.AccessPeriod * 100 * 3600
	aTokString, err := tok.IssueAccessToken(usr.Id(), org.SecretKey)
	if err != nil {
		return http.Fail(c, 403, "Could not issue token", err)
	}

	resp := OAuthResponse{
		AccessToken:  aTokString,
		RefreshToken: refreshToken,
		ExpiresIn:    int(expiresIn),
		TokenType:    "jwt",
	}

	return http.Render(c, 200, resp)
}
