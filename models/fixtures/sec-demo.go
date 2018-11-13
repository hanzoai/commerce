package fixtures

import (
	"fmt"
	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/models/user"
	"hanzo.io/util/fake"
)

// 'user.email':                   [ isRequired, isEmail ]
// 'user.firstName':               [ isRequired ]
// 'user.lastName':                [ isRequired ]
// 'user.kyc.phone':               [ isRequired ]
// 'user.kyc.birthdate':           [ isRequired ]
// 'user.kyc.gender':              [ isRequired ]

// 'user.kyc.address.name':        [ isRequired ]
// 'user.kyc.address.line1':       [ isRequired ]
// 'user.kyc.address.line2':       null
// 'user.kyc.address.city':        [ isRequired ]
// 'user.kyc.address.state':       [ isStateRequired ]
// 'user.kyc.address.postalCode':  [ isPostalRequired ]
// 'user.kyc.address.country':     [ isRequired ]

// 'user.kyc.taxId':               [ isRequired ]

var SECDemo = New("sec-demo", func(c *gin.Context) *user.User {
	db := datastore.New(c)

	for i := 0; i < 100; i++ {
		// Such tees owner & operator
		usr := user.New(db)
		usr.Email = fake.EmailAddress()
		usr.GetOrCreate("Email=", usr.Email)

		usr.FirstName = fake.FirstName()
		usr.LastName = fake.LastName()
		usr.PasswordHash, _ = password.Hash("secdemo")

		usr.KYC.Phone = fake.Phone()
		usr.KYC.Birthdate = fmt.Sprintf("%d-%d-%d", fake.MonthNum(), fake.Day(), fake.Year(1942, 2000))
		usr.KYC.Gender = fake.Gender()
		usr.KYC.Address.Name = usr.FirstName + " " + usr.LastName
		usr.KYC.Address.Line1 = fake.StreetAddress()
		usr.KYC.Address.City = fake.City()
		usr.KYC.Address.State = fake.State()
		usr.KYC.Address.PostalCode = fake.Zip()
		usr.KYC.Address.Country = "US"
		usr.KYC.TaxId = fake.TaxID()
		usr.KYC.WalletAddresses = []string{fake.EOSAddress(), fake.EthereumAddress()}
		usr.MustPut()

		return usr
	}

	return nil
})
