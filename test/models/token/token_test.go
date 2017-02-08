package test

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/models/token2"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("models/tokens", t)
}

var (
	ctx       context.Context
	inst      ae.Instance
	db        *datastore.Datastore
	secretKey []byte
	refClaims token.Claims
	apiClaims token.Claims
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	ctx, inst, _ = ae.NewContext()
	db = datastore.New(ctx)

	secretKey = []byte("SECRET")

	refClaims = token.Claims{}
	refClaims.OrganizationName = "ORG"
	refClaims.UserId = "USER"
	refClaims.Type = token.Reference
	refClaims.JTI = "JTI"
	refClaims.IssuedAt = time.Now().Unix()

	apiClaims = token.Claims{}
	apiClaims.AppName = "APP"
	apiClaims.OrganizationName = "ORG"
	apiClaims.Type = token.Api
	apiClaims.JTI = "JTI"
	apiClaims.IssuedAt = time.Now().Unix()
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})

var _ = Describe("models/token", func() {
	Context("Should Create and Validate Tokens", func() {
		It("Should validate a Ref Token", func() {
			Expect(token.IsReference(refClaims)).To(Equal(true))
			Expect(token.IsRefresh(refClaims)).To(Equal(false))
			Expect(token.IsAccess(refClaims)).To(Equal(false))
			Expect(token.IsApi(refClaims)).To(Equal(false))
			Expect(token.IsCustomer(refClaims)).To(Equal(false))
		})
		It("Should validate a Refresh Token", func() {
			tok := token.New(db)
			tok.Claims = refClaims
			str, err := tok.IssueRefreshToken("USER", secretKey)
			Expect(err).ToNot(HaveOccurred())

			tok2 := token.New(db)
			tok2.Decode(str, secretKey)
			claims := tok2.Claims

			Expect(token.IsReference(claims)).To(Equal(false))
			Expect(token.IsRefresh(claims)).To(Equal(true))
			Expect(token.IsAccess(claims)).To(Equal(false))
			Expect(token.IsApi(claims)).To(Equal(false))
			Expect(token.IsCustomer(claims)).To(Equal(false))
		})
		It("Should validate an Access Token", func() {
			tok := token.New(db)
			tok.Claims = refClaims
			str, err := tok.IssueAccessToken("USER", secretKey)
			Expect(err).ToNot(HaveOccurred())

			tok2 := token.New(db)
			tok2.Claims = token.Claims{}
			tok2.Decode(str, secretKey)
			claims := tok2.Claims

			Expect(token.IsReference(claims)).To(Equal(false))
			Expect(token.IsRefresh(claims)).To(Equal(false))
			Expect(token.IsAccess(claims)).To(Equal(true))
			Expect(token.IsApi(claims)).To(Equal(false))
			Expect(token.IsCustomer(claims)).To(Equal(false))
		})
		It("Should validate a API Key", func() {
			Expect(token.IsReference(apiClaims)).To(Equal(false))
			Expect(token.IsRefresh(apiClaims)).To(Equal(false))
			Expect(token.IsAccess(apiClaims)).To(Equal(false))
			Expect(token.IsApi(apiClaims)).To(Equal(true))
			Expect(token.IsAccess(apiClaims)).To(Equal(false))
		})
		It("Should validate an Customer Token", func() {
			tok := token.New(db)
			tok.Claims = apiClaims
			str, err := tok.IssueAccessToken("USER", secretKey)
			Expect(err).ToNot(HaveOccurred())

			tok2 := token.New(db)
			tok2.Claims = token.Claims{}
			tok2.Decode(str, secretKey)
			claims := tok2.Claims

			Expect(token.IsReference(claims)).To(Equal(false))
			Expect(token.IsRefresh(claims)).To(Equal(false))
			Expect(token.IsAccess(claims)).To(Equal(false))
			Expect(token.IsApi(claims)).To(Equal(false))
			Expect(token.IsCustomer(claims)).To(Equal(true))
		})
	})
})
