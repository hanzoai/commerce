package test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/zeekay/aetest"

	"crowdstart.io/models"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/log"
)

func TestSalesforce(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "salesforce")
}

type ClientParams struct {
	Verb    string
	Path    string
	Data    string
	Headers map[string]string
	Body    []byte
}

type MockSalesforceClient struct {
	Params *ClientParams
}

func (a MockSalesforceClient) Request(method, path, data string, headers *map[string]string, retry bool) error {
	a.Params.Verb = method
	a.Params.Path = path
	a.Params.Data = data
	if headers != nil {
		a.Params.Headers = *headers
	}
	return nil
}

func (a MockSalesforceClient) GetBody() []byte {
	return a.Params.Body
}

var (
	ctx    aetest.Context
	user   models.User
	params *ClientParams
)

var _ = BeforeSuite(func() {
	var err error
	ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	Expect(err).ToNot(HaveOccurred())

	user = models.User{
		Id:        "Id",
		FirstName: "First",
		LastName:  "Last",
		Phone:     "(123)-456-7890",
		Email:     "dev@hanzo.ai",
		BillingAddress: models.Address{
			Line1:      "BillMeAt",
			Line2:      "Line2",
			City:       "City",
			State:      "State",
			PostalCode: "PostalCode",
			Country:    "Country",
		},
		ShippingAddress: models.Address{
			Line1:      "ShipMeAt",
			Line2:      "Line2",
			City:       "City",
			State:      "State",
			PostalCode: "PostalCode",
			Country:    "Country",
		},
	}

	params = new(ClientParams)
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("User (de)serialization", func() {
	Context("Account and Contact To/From User", func() {
		It("Should work", func() {
			// Contact and Account should serialize and then deserialze to the original object
			contact := salesforce.Contact{}
			contact.FromUser(&user)

			// CrowdstartIdC should never be set as it causes SF to reject the request
			Expect(contact.CrowdstartIdC).To(Equal(""))
			contact.CrowdstartIdC = "Id"

			account := salesforce.Account{}
			account.FromUser(&user)

			// CrowdstartIdC should never be set as it causes SF to reject the request
			Expect(account.CrowdstartIdC).To(Equal(""))
			account.CrowdstartIdC = "Id"

			u := models.User{}
			contact.ToUser(&u)
			account.ToUser(&u)

			//log.Panic("%v ?= %v", u, user, ctx)
			Expect(reflect.DeepEqual(user, u)).To(Equal(true))
		})
	})

	Context("Push/Pull User", func() {
		It("Push Contact", func() {
			client := MockSalesforceClient{Params: params}
			contact := salesforce.Contact{}
			contact.Push(client, &user)

			// Verify that the client received the correct inputs
			Expect(params.Verb).To(Equal("PATCH"))

			path := fmt.Sprintf(salesforce.ContactExternalIdPath, strings.Replace(user.Id, ".", "_", -1))
			Expect(params.Path).To(Equal(path))

			data, _ := json.Marshal(contact)
			Expect(params.Data).To(Equal(string(data[:])))

			Expect(params.Headers).To(Equal(map[string]string{"Content-Type": "application/json"}))
		})

		It("Push Account", func() {
			client := MockSalesforceClient{Params: params}
			account := salesforce.Account{}
			account.Push(client, &user)

			// Verify that the client received the correct inputs
			Expect(params.Verb).To(Equal("PATCH"))

			path := fmt.Sprintf(salesforce.AccountExternalIdPath, strings.Replace(user.Id, ".", "_", -1))
			Expect(params.Path).To(Equal(path))

			data, _ := json.Marshal(account)
			Expect(params.Data).To(Equal(string(data[:])))

			Expect(params.Headers).To(Equal(map[string]string{"Content-Type": "application/json"}))
		})

		It("Pull User", func() {
			client := MockSalesforceClient{Params: params}
			account := salesforce.Account{}
			contact := salesforce.Contact{}

			// Create reference objects for testing from user
			refAccount := salesforce.Account{}
			refAccount.CrowdstartIdC = "Id"
			refAccount.FromUser(&user)

			refContact := salesforce.Contact{}
			refContact.CrowdstartIdC = "Id"
			refContact.FromUser(&user)

			u := models.User{}

			// Set the bodies to be decoded
			params.Body, _ = json.Marshal(refAccount)
			account.Pull(client, "Id", &u)
			params.Body, _ = json.Marshal(refContact)
			contact.Pull(client, "Id", &u)

			// Referenced and Decoded values should be equal
			Expect(reflect.DeepEqual(account, refAccount)).To(Equal(true))
			Expect(reflect.DeepEqual(contact, refContact)).To(Equal(true))
			Expect(reflect.DeepEqual(u, user)).To(Equal(true))
		})
	})
})
