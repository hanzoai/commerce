package test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/test/ae"

	. "github.com/hanzoai/commerce/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("middleware/iammiddleware", t)
}

const (
	testKeyID    = "test-key-1"
	testClientID = "test-client-id"
	testOrgName  = "test-org"
)

var (
	ctx            ae.Context
	db             *datastore.Datastore
	testPrivateKey *rsa.PrivateKey
	testPublicKey  *rsa.PublicKey
	testIssuer     string
	jwksServer     *httptest.Server
)

// signToken creates a JWT signed with the test RSA key.
func signToken(claims *auth.IAMClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKeyID
	signed, err := token.SignedString(testPrivateKey)
	if err != nil {
		panic("failed to sign token: " + err.Error())
	}
	return signed
}

// makeAdminClaims creates IAM claims for an admin user.
func makeAdminClaims() *auth.IAMClaims {
	return &auth.IAMClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "user-admin-123",
			Issuer:    testIssuer,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
		Audience: auth.FlexAudience(testClientID),
		Owner:    testOrgName,
		Name:     "admin-user",
		Email:    "admin@test.com",
		Roles:    []string{"admin"},
		IsAdmin:  true,
	}
}

// makeMemberClaims creates IAM claims for a member user.
func makeMemberClaims() *auth.IAMClaims {
	return &auth.IAMClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "user-member-456",
			Issuer:    testIssuer,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
		Audience: auth.FlexAudience(testClientID),
		Owner:    testOrgName,
		Name:     "member-user",
		Email:    "member@test.com",
		Roles:    []string{"member"},
	}
}

// makeMinimalClaims creates IAM claims with no special roles.
func makeMinimalClaims() *auth.IAMClaims {
	return &auth.IAMClaims{
		StandardClaims: jwt.StandardClaims{
			Subject:   "user-basic-789",
			Issuer:    testIssuer,
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
		},
		Audience: auth.FlexAudience(testClientID),
		Owner:    testOrgName,
		Name:     "basic-user",
		Email:    "basic@test.com",
		Roles:    []string{"viewer"},
	}
}

var _ = BeforeSuite(func() {
	ctx = ae.NewContext()
	db = datastore.New(ctx)

	// Generate RSA key pair for JWT signing
	var err error
	testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	testPublicKey = &testPrivateKey.PublicKey

	// Create JWKS test server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discovery := map[string]interface{}{
			"issuer":                 testIssuer,
			"authorization_endpoint": testIssuer + "/login/oauth/authorize",
			"token_endpoint":         testIssuer + "/api/login/oauth/access_token",
			"userinfo_endpoint":      testIssuer + "/api/userinfo",
			"jwks_uri":               testIssuer + "/.well-known/jwks",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(discovery)
	})
	mux.HandleFunc("/.well-known/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"kid": testKeyID,
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(testPublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(intToBytes(testPublicKey.E)),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})
	jwksServer = httptest.NewServer(mux)
	testIssuer = jwksServer.URL

	// Initialize IAM middleware with test config
	err = iammiddleware.Init(&auth.IAMConfig{
		Issuer:     testIssuer,
		ClientID:   testClientID,
		HTTPClient: jwksServer.Client(),
	})
	Expect(err).NotTo(HaveOccurred())

	// Create test user and organization
	u := user.New(db)
	err = u.Put()
	Expect(err).NotTo(HaveOccurred())

	o := organization.New(db)
	o.Name = testOrgName
	o.SecretKey = []byte("test-secret-key")
	o.Owners = []string{u.Id()}
	err = o.Put()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if jwksServer != nil {
		jwksServer.Close()
	}
	ctx.Close()
})

// intToBytes converts an int to big-endian bytes (for JWK exponent encoding).
func intToBytes(n int) []byte {
	if n == 0 {
		return []byte{0}
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte(n & 0xff)}, b...)
		n >>= 8
	}
	return b
}
