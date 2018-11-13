package fixtures

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/demo/tokentransaction"
	"hanzo.io/models/user"
	"hanzo.io/util/fake"
)

var SECDemo = New("sec-demo", func(c *gin.Context) *user.User {
	db := datastore.New(c)

	for i := 0; i < 100; i++ {
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
	}

	for i := 0; i < 100; i++ {
		tr := tokentransaction.New(db)

		if rand.Float64() > 0.7 {
			tr.TransactionHash = fake.EthereumAddress()
			tr.SendingAddress = fake.EthereumAddress()
			tr.ReceivingAddress = fake.EthereumAddress()
			tr.Protocol = "ETH"
		} else {
			tr.TransactionHash = fake.EOSAddress()
			tr.SendingAddress = fake.EOSAddress()
			tr.ReceivingAddress = fake.EOSAddress()
			tr.Protocol = "EOS"
		}

		tr.Timestamp = time.Now()

		tr.SendingName = fake.FullName()
		tr.SendingUserId = fake.Id()
		tr.SendingState = fake.State()
		tr.SendingCountry = "US"

		tr.ReceivingName = fake.FullName()
		tr.ReceivingUserId = fake.Id()
		tr.ReceivingState = fake.State()
		tr.ReceivingCountry = "US"
		tr.MustPut()
	}

	return nil
})
