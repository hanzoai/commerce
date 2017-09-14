package test

import (
	"testing"
	"time"

	"hanzo.io/util/bit"
	token "hanzo.io/util/oldjwt"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/oldjwt", t)
}

var (
	secret = []byte("secret")
)

// Backported test from Hanzo1 to master
var _ = Describe("Token", func() {
	It("Should Get Constructor Values", func() {
		t := token.New("test", "test", 16, secret)
		Expect(t.Get("sub").(string)).To(Equal("test"))
		Expect(int(t.Get("bit").(int64))).To(Equal(16))
		Expect(int(t.Permissions)).To(Equal(16))
	})

	It("Should Set/Get Standard Claims", func() {
		t := token.New("test", "test2", 16, secret)
		t.Set("bit", bit.Field(17))
		t.Permissions = 17

		now := time.Now()
		t.Set("iat", now)
		t.Set("exp", now)
		t.Set("jti", "jti")
		t.Set("org", "org")
		t.Set("usr", "usr")

		Expect(t.Get("sub").(string)).To(Equal("test2"))
		Expect(t.EntityId).To(Equal("test2"))
		Expect(int(t.Get("bit").(bit.Field))).To(Equal(17))
		Expect(int(t.Permissions)).To(Equal(17))
		Expect(t.Get("iat").(time.Time).Unix()).To(Equal(now.Unix()))
		Expect(t.IssuedAt.Unix()).To(Equal(now.Unix()))

		Expect(t.Get("jti").(string)).To(Equal("jti"))
		Expect(t.Get("org").(string)).To(Equal("org"))
		Expect(t.Get("usr").(string)).To(Equal("usr"))
	})

	It("Should Set/Get Non-Standard Claims", func() {
		t := token.New("test", "name", 16, secret)
		t.Set("email", "test@test.com")
		thing := []string{"okay"}
		t.Set("thing", thing)

		Expect(t.Get("email").(string)).To(Equal("test@test.com"))
		Expect(t.Get("thing").([]string)).To(Equal(thing))
	})

	It("Should Create Verifiable Token", func() {
		t := token.New("test", "name", 16, secret)
		t.TokenString = t.String()
		Expect(t.Verify(secret)).To(BeTrue())
	})

	It("Should Have a Different TokenString After Set", func() {
		t := token.New("test", "name", 16, secret)
		t.TokenString = t.String()
		str := t.TokenString
		t.Set("email", "test@test.com")
		t.TokenString = t.String()
		str2 := t.TokenString
		Expect(str).ToNot(Equal(str2))
	})

	// N/A for not hanzo1 branch, we need to keep the current keys working

	// It("Should Parse Created Tokens Claims", func() {
	// 	now := time.Now()

	// 	t := token.New("test", "name", 16, secret)
	// 	t.Set("iat", now)
	// 	t.Set("exp", now)
	// 	t.Set("jti", "jti")
	// 	t.Set("org", "org")
	// 	t.Set("usr", "usr")

	// 	t2, err := token.Parse(t.TokenString)
	// 	Expect(err).To(BeNil())

	// 	Expect(t.Type).To(Equal(t2.Type))
	// 	Expect(t.Permissions).To(Equal(t2.Permissions))
	// 	Expect(t.IssuedAt.Unix()).To(Equal(t2.IssuedAt.Unix()))

	// 	Expect(t.Get("jti").(string)).To(Equal(t2.Get("jti").(string)))
	// 	Expect(t.Get("org").(string)).To(Equal(t2.Get("org").(string)))
	// 	Expect(t.Get("usr").(string)).To(Equal(t2.Get("usr").(string)))
	// })

	// It("Should Clone Tokens Claims", func() {
	// 	now := time.Now()

	// 	t := token.New("test", 16, secret)
	// 	t.Set("iat", now)
	// 	t.Set("exp", now)
	// 	t.Set("jti", "jti")
	// 	t.Set("org", "org")
	// 	t.Set("usr", "usr")

	// 	t2, err := t.Clone()
	// 	Expect(err).To(BeNil())

	// 	Expect(t.Type).To(Equal(t2.Type))
	// 	Expect(t.Permissions).To(Equal(t2.Permissions))
	// 	Expect(t.IssuedAt.Unix()).To(Equal(t2.IssuedAt.Unix()))

	// 	Expect(t.Get("jti").(string)).To(Equal(t2.Get("jti").(string)))
	// 	Expect(t.Get("org").(string)).To(Equal(t2.Get("org").(string)))
	// 	Expect(t.Get("usr").(string)).To(Equal(t2.Get("usr").(string)))
	// })
})
