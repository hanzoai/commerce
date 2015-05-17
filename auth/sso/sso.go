package sso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/models"
)

var (
	ErrInvalid = errors.New("invalid sso")
)

type Nonce string

type SSO struct {
	secret string
}

func New(secret string) *SSO {
	sso := new(SSO)
	sso.secret = secret
	return sso
}

func (s *SSO) Parse(sso, sig string) (Nonce, error) {
	h := hmac.New(sha256.New, []byte(s.secret))
	_, err := h.Write([]byte(sso))
	if err != nil {
		return "", err
	}
	sig2 := hex.EncodeToString(h.Sum(nil))
	if sig != sig2 {
		return "", ErrInvalid
	}

	qs, err := base64.StdEncoding.DecodeString(sso)
	if err != nil {
		return "", err
	}
	v, err := url.ParseQuery(string(qs))
	if err != nil {
		return "", err
	}

	return Nonce(v.Get("nonce")), nil
}

func (s *SSO) Build(nonce Nonce, user *models.User) (url.Values, error) {
	v := make(url.Values)
	v.Set("nonce", string(nonce))
	v.Set("email", user.Email)
	v.Set("external_id", user.Id)
	v.Set("username", user.Email)
	v.Set("name", user.Name())

	p := base64.StdEncoding.EncodeToString([]byte(v.Encode()))

	h := hmac.New(sha256.New, []byte(s.secret))

	_, err := h.Write([]byte(p))
	if err != nil {
		return nil, err
	}
	sig := hex.EncodeToString(h.Sum(nil))

	v = make(url.Values)
	v.Set("sso", p)
	v.Set("sig", sig)
	return v, nil
}

var std = New(config.Discourse.Secret)

func Parse(sso, sig string) (string, error) {
	nonce, err := std.Parse(sso, sig)

	return string(nonce), err
}

func Build(nonce string, user *models.User) (url.Values, error) {
	return std.Build(Nonce(nonce), user)
}

func Redirect(c *gin.Context, nonce string, user *models.User) {
	params, err := Build(nonce, user)
	if err != nil {
		c.Redirect(302, config.Discourse.URL)
	}

	url, _ := url.Parse(config.Discourse.URL + "/session/sso_login")
	url.RawQuery = params.Encode()
	c.Redirect(302, url.String())
}
