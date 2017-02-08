package test

import (
	"time"

	"hanzo.io/util/bit"
	"hanzo.io/util/token"

	. "hanzo.io/util/test/ginkgo"
)

var (
	secret = []byte("secret")
)

var _ = Describe("Token", func() {
	It("Should Get Constructor Values", func() {
		t := token.New("test", 16, secret)
		Expect(t.Get("typ").(string)).To(Equal("test"))
		Expect(t.Type).To(Equal("test"))
		Expect(int(t.Get("bit").(bit.Field))).To(Equal(16))
		Expect(int(t.Permissions)).To(Equal(16))
	})

	It("Should Set/Get Standard Claims", func() {
		t := token.New("test", 16, secret)
		t.Set("typ", "test2")
		t.Set("bit", bit.Field(17))

		now := time.Now()
		t.Set("iat", now)
		t.Set("exp", now)
		t.Set("jti", "jti")
		t.Set("org", "org")
		t.Set("usr", "usr")

		Expect(t.Get("typ").(string)).To(Equal("test2"))
		Expect(t.Type).To(Equal("test2"))
		Expect(int(t.Get("bit").(bit.Field))).To(Equal(17))
		Expect(int(t.Permissions)).To(Equal(17))
		Expect(t.Get("iat").(time.Time).Unix()).To(Equal(now.Unix()))
		Expect(t.IssuedAt.Unix()).To(Equal(now.Unix()))

		Expect(t.Get("jti").(string)).To(Equal("jti"))
		Expect(t.Get("org").(string)).To(Equal("org"))
		Expect(t.Get("usr").(string)).To(Equal("usr"))
	})

	It("Should Set/Get Non-Standard Claims", func() {
		t := token.New("test", 16, secret)
		t.Set("email", "test@test.com")
		thing := []string{"okay"}
		t.Set("thing", thing)

		Expect(t.Get("email").(string)).To(Equal("test@test.com"))
		Expect(t.Get("thing").([]string)).To(Equal(thing))
	})

	It("Should Create Verifiable Token", func() {
		t := token.New("test", 16, secret)
		Expect(t.Verify()).To(Equal(true))
	})

	It("Should Have a Different TokenString After Set", func() {
		t := token.New("test", 16, secret)
		str := t.TokenString
		t.Set("email", "test@test.com")
		str2 := t.TokenString
		Expect(str).ToNot(Equal(str2))
	})

	It("Should Parse Created Tokens Claims", func() {
		now := time.Now()

		t := token.New("test", 16, secret)
		t.Set("iat", now)
		t.Set("exp", now)
		t.Set("jti", "jti")
		t.Set("org", "org")
		t.Set("usr", "usr")

		t2, err := token.Parse(t.TokenString, secret)
		Expect(err).To(BeNil())

		Expect(t.Type).To(Equal(t2.Type))
		Expect(t.Permissions).To(Equal(t2.Permissions))
		Expect(t.IssuedAt.Unix()).To(Equal(t2.IssuedAt.Unix()))

		Expect(t.Get("jti").(string)).To(Equal(t2.Get("jti").(string)))
		Expect(t.Get("org").(string)).To(Equal(t2.Get("org").(string)))
		Expect(t.Get("usr").(string)).To(Equal(t2.Get("usr").(string)))
	})

	It("Should Clone Tokens Claims", func() {
		now := time.Now()

		t := token.New("test", 16, secret)
		t.Set("iat", now)
		t.Set("exp", now)
		t.Set("jti", "jti")
		t.Set("org", "org")
		t.Set("usr", "usr")

		t2, err := t.Clone()
		Expect(err).To(BeNil())

		Expect(t.Type).To(Equal(t2.Type))
		Expect(t.Permissions).To(Equal(t2.Permissions))
		Expect(t.IssuedAt.Unix()).To(Equal(t2.IssuedAt.Unix()))

		Expect(t.Get("jti").(string)).To(Equal(t2.Get("jti").(string)))
		Expect(t.Get("org").(string)).To(Equal(t2.Get("org").(string)))
		Expect(t.Get("usr").(string)).To(Equal(t2.Get("usr").(string)))
	})
})
