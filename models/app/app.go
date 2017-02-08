package app

import (
	"time"

	"hanzo.io/models/mixin"
	"hanzo.io/models/token2"
	"hanzo.io/util/bit"
	"hanzo.io/util/permission"
)

const (
	PublishedKey     = "pub-key"
	SecretKey        = "secret-key"
	TestPublishedKey = "test-pub-key"
	TestSecretKey    = "test-secret-key"
)

type App struct {
	mixin.Model

	Name string `json:"name"`

	ApiKeys []*token.Token `json:"apiKeys,omitempty" datastore:"-"`

	SecretKey []byte `json:"-"`
}

func (a *App) NewApiKey(name string, claims token.Claims) (*token.Token, error) {
	a.RevokeApiKeyByName(name)

	tok := token.New(a.Db)
	tok.Name = name

	claims.AppName = a.Name
	claims.OrganizationName = a.Key().Namespace()
	claims.Type = token.Api
	claims.JTI = tok.Id()
	claims.IssuedAt = time.Now().Unix()

	tok.Claims = claims
	tok.AccessPeriod = 24

	if _, err := tok.Encode(a.SecretKey); err != nil {
		return nil, err
	}

	tok.MustCreate()

	return tok, nil
}

func (a *App) GetApiKeyByName(name string) (*token.Token, bool, error) {
	tok := token.New(a.Db)

	if ok, err := tok.Query().Filter("Claims.AppName=", a.Name).Filter("Claims.Type=", token.Api).Filter("Revoked=", false).Filter("Name=", name).First(); !ok {
		return nil, false, err
	}

	return tok, true, nil
}

func (a *App) RevokeApiKeyByName(name string) (*token.Token, bool, error) {
	if tok, ok, err := a.GetApiKeyByName(name); !ok {
		return nil, false, err
	} else {
		tok.Revoke()
		return tok, true, nil
	}
}

func (a *App) LoadApiKeys() error {
	slice, err := token.Query(a.Db).
		Filter("Claims.AppName=", a.Name).
		Filter("Claims.Type=", token.Api).
		Filter("Revoked=", false).
		GetAll()

	a.ApiKeys = slice.([]*token.Token)

	return err
}

func (a *App) ResetDefaultKeys() {
	pubClaims := token.Claims{
		AppName:          a.Name,
		OrganizationName: a.Key().Namespace(),
		Type:             token.Api,
		Permissions:      bit.Field(permission.Published | permission.Live | permission.ReadCoupon | permission.ReadProduct | permission.WriteReferrer),
	}

	secretClaims := pubClaims.Clone().(token.Claims)
	secretClaims.Permissions = bit.Field(permission.Admin | permission.Live)

	testPubClaims := pubClaims.Clone().(token.Claims)
	testPubClaims.Test = true
	testPubClaims.Permissions = bit.Field(permission.Published | permission.Test | permission.ReadCoupon | permission.ReadProduct | permission.WriteReferrer)

	testSecretClaims := testPubClaims.Clone().(token.Claims)
	testSecretClaims.Permissions = bit.Field(permission.Admin | permission.Test)

	var err error

	pubKey, err := a.NewApiKey(PublishedKey, pubClaims)
	if err != nil {
		panic(err)
	}

	secretKey, err := a.NewApiKey(SecretKey, secretClaims)
	if err != nil {
		panic(err)
	}

	testPubKey, err := a.NewApiKey(TestPublishedKey, testPubClaims)
	if err != nil {
		panic(err)
	}

	testSecretKey, err := a.NewApiKey(TestSecretKey, testSecretClaims)
	if err != nil {
		panic(err)
	}

	a.ApiKeys = []*token.Token{
		pubKey,
		secretKey,
		testPubKey,
		testSecretKey,
	}
}
