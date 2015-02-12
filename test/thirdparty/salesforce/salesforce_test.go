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

	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/salesforce"
	"crowdstart.io/util/log"
)

func TestSalesforce(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	RegisterFailHandler(Fail)
	RunSpecs(t, "salesforce")
}

type MockSObjectSerializeable struct {
	Id        string
	FirstName string
}

func (s *MockSObjectSerializeable) SetExternalId(id string) {
	s.Id = id
}

func (s *MockSObjectSerializeable) ExternalId() string {
	return s.Id
}

// Only update the first name field
func (s *MockSObjectSerializeable) Write(u *models.User) {
	u.FirstName = s.FirstName
}

func (s *MockSObjectSerializeable) Read(u *models.User) {
	s.FirstName = u.FirstName
}

type ClientParams struct {
	Verb    string
	Path    string
	Data    string
	Headers map[string]string
	Bodies  [][]byte
}

type MockSalesforceClient struct {
	Params *ClientParams
}

func (a *MockSalesforceClient) Request(method, path, data string, headers *map[string]string, retry bool) error {
	a.Params.Verb = method
	a.Params.Path = path
	a.Params.Data = data
	if headers != nil {
		a.Params.Headers = *headers
	}
	return nil
}

func (a MockSalesforceClient) GetBody() []byte {
	bodies := a.Params.Bodies

	var body []byte
	body, a.Params.Bodies = bodies[0], bodies[1:]
	return body
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
			contact.Read(&user)

			account := salesforce.Account{}
			account.Read(&user)

			u := models.User{}
			contact.Write(&u)
			account.Write(&u)

			Expect(reflect.DeepEqual(user, u)).To(Equal(true))
		})

		It("Contact should treat CrowdstartIdC as ExternalId", func() {
			contact := salesforce.Contact{CrowdstartIdC: "1234"}
			Expect(contact.CrowdstartIdC).To(Equal(contact.ExternalId()))

			contact.SetExternalId("4321")
			Expect("4321").To(Equal(contact.CrowdstartIdC))
		})

		It("Account should treat CrowdstartIdC as ExternalId", func() {
			account := salesforce.Account{CrowdstartIdC: "1234"}
			Expect(account.CrowdstartIdC).To(Equal(account.ExternalId()))

			account.SetExternalId("4321")
			Expect("4321").To(Equal(account.CrowdstartIdC))
		})
	})

	Context("Push/Pull User", func() {
		It("Push Contact", func() {
			client := MockSalesforceClient{Params: params}
			contact := salesforce.Contact{}
			contact.Read(&user)
			contact.Push(&client)
			// blank out the CrowdstartIdC since it is never serialized
			contact.CrowdstartIdC = ""

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
			account.Read(&user)
			account.Push(&client)
			// blank out the CrowdstartIdC since it is never serialized
			account.CrowdstartIdC = ""

			// Verify that the client received the correct inputs
			Expect(params.Verb).To(Equal("PATCH"))

			path := fmt.Sprintf(salesforce.AccountExternalIdPath, strings.Replace(user.Id, ".", "_", -1))
			Expect(params.Path).To(Equal(path))

			data, _ := json.Marshal(account)
			Expect(params.Data).To(Equal(string(data[:])))

			Expect(params.Headers).To(Equal(map[string]string{"Content-Type": "application/json"}))
		})

		It("PullExternalId User", func() {
			client := MockSalesforceClient{Params: params}
			account := salesforce.Account{}
			contact := salesforce.Contact{}

			// Create reference objects for testing from user
			refAccount := salesforce.Account{}
			refAccount.Read(&user)

			refContact := salesforce.Contact{}
			refContact.Read(&user)

			// Set the bodies to be decoded
			body1, _ := json.Marshal(refAccount)
			body2, _ := json.Marshal(refContact)

			params.Bodies = append(params.Bodies, body1, body2)

			account.PullExternalId(&client, "Id")
			contact.PullExternalId(&client, "Id")

			// Referenced and Decoded values should be equal
			Expect(reflect.DeepEqual(account, refAccount)).To(Equal(true))
			Expect(reflect.DeepEqual(contact, refContact)).To(Equal(true))
		})

		It("PullId User", func() {
			client := MockSalesforceClient{Params: params}
			account := salesforce.Account{}
			contact := salesforce.Contact{}

			// Create reference objects for testing from user
			refAccount := salesforce.Account{}
			refAccount.CrowdstartIdC = "Id"
			refAccount.Read(&user)

			refContact := salesforce.Contact{}
			refContact.CrowdstartIdC = "Id"
			refContact.Read(&user)

			// Set the bodies to be decoded
			body1, _ := json.Marshal(refAccount)
			body2, _ := json.Marshal(refContact)

			params.Bodies = append(params.Bodies, body1, body2)

			account.PullId(&client, "Id")
			contact.PullId(&client, "Id")

			// Referenced and Decoded values should be equal
			Expect(reflect.DeepEqual(account, refAccount)).To(Equal(true))
			Expect(reflect.DeepEqual(contact, refContact)).To(Equal(true))
		})
	})

	Context("Salesforce API", func() {
		It("PullUpdated with nothing in the DB", func() {
			db := datastore.New(ctx)

			response := salesforce.UpdatedRecordsResponse{
				Ids: []string{"NOT IN THE DB"},
			}

			users := make(map[string]*models.User)
			salesforce.ProcessUpdatedSObjects(db,
				&response,
				users,
				func(id string) salesforce.SObjectSerializeable {
					s := new(MockSObjectSerializeable)
					s.Id = "NOT IN THE DB"
					s.FirstName = "SOME NAME"
					return s
				})

			u, ok := users["NOT IN THE DB"]
			Expect(ok).To(Equal(true))

			// Only the FirstName is updated for the MockSObjectSerializeable
			// FirstName should therefore be the only set field
			refUser := models.User{FirstName: "SOME NAME"}

			Expect(reflect.DeepEqual(&refUser, u)).To(Equal(true))
		})

		It("PullUpdated with something in the DB", func() {
			db := datastore.New(ctx)
			key := db.NewKey("user", "Id", 0, nil)
			id := key.Encode()

			// PullUpdated will update a record in db, so add a record to the db that is slightly different than the master user
			someUser := models.User{
				Id:              user.Id,
				FirstName:       "Bad First Name",
				LastName:        user.LastName,
				Phone:           user.Phone,
				Email:           user.Email,
				BillingAddress:  user.BillingAddress,
				ShippingAddress: user.ShippingAddress,
			}

			// Insert into DB
			db.Put(key, &someUser)

			response := salesforce.UpdatedRecordsResponse{
				Ids: []string{id},
			}

			users := make(map[string]*models.User)
			salesforce.ProcessUpdatedSObjects(db,
				&response,
				users,
				func(id string) salesforce.SObjectSerializeable {
					s := new(MockSObjectSerializeable)
					s.Id = id
					// Set the First Name to the one used by the master user
					s.FirstName = user.FirstName
					return s
				})

			// The updated user should look identical to the master user
			u, ok := users[id]
			Expect(ok).To(Equal(true))

			// The Datastore initializes these values differently so set them to what they should be
			u.Cart = user.Cart
			u.Campaigns = user.Campaigns
			u.PasswordHash = user.PasswordHash
			u.UpdatedAt = user.UpdatedAt
			u.CreatedAt = user.CreatedAt
			u.Metadata = user.Metadata

			Expect(reflect.DeepEqual(&user, u)).To(Equal(true))
		})
	})
})
