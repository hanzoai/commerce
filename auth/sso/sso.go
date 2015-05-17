package sso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"

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

var std = New("d836444a9e4084d5b224a60c208dce14")

func Parse(sso, sig string) (Nonce, error) {
	return std.Parse(sso, sig)
}

func Build(nonce string, user *models.User) (url.Values, error) {
	return std.Build(Nonce(nonce), user)
}
