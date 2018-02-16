package test

import (
	"google.golang.org/appengine"

	"github.com/zeekay/aetest"

	"hanzo.io/datastore"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/salesforce"

	. "hanzo.io/util/test/ginkgo"
)

// func Test(t *testing.T) {
// 	log.SetVerbose(testing.Verbose())
// 	RegisterFailHandler(Fail)
// 	RunSpecs(t, "salesforce")
// }

type MockSObjectTypes struct {
	A string              `json:"String__C"`
	B salesforce.Currency `json:"Currency__C"`
	C bool                `json:"Bool__C"`
	D string              `json:"D"`
	E string
}

type MockSObjectSerializeable struct {
	salesforce.ModelReference

	Id                string `json:"Id__C"`
	FirstName         string `json:"FirstName__C"`
	ExpectedId        string `json:"-",datastore:"-",schema:"-"`
	ExpectedFirstName string `json:"-",datastore:"-",schema:"-"`
}

func (s *MockSObjectSerializeable) SetExternalId(id string) {
	s.Id = id
}

func (s *MockSObjectSerializeable) ExternalId() string {
	return s.Id
}

// Only update the first name field
func (s *MockSObjectSerializeable) Write(so salesforce.SObjectCompatible) error {
	// u := so.(*models.User)
	// u.FirstName = s.FirstName

	return nil
}

func (s *MockSObjectSerializeable) Read(so salesforce.SObjectCompatible) error {
	// u := so.(*models.User)
	// s.FirstName = u.FirstName

	return nil
}

func (s *MockSObjectSerializeable) Load(db *datastore.Datastore) salesforce.SObjectCompatible {
	s.Ref = user.New(db)
	db.GetById(s.ExternalId(), s.Ref)
	return s.Ref
}

func (s *MockSObjectSerializeable) Push(api salesforce.SalesforceClient) error {
	return nil
}
func (s *MockSObjectSerializeable) PullExternalId(api salesforce.SalesforceClient, id string) error {
	return nil
}

func (s *MockSObjectSerializeable) PullId(api salesforce.SalesforceClient, id string) error {
	s.Id = s.ExpectedId
	s.FirstName = s.ExpectedFirstName
	return nil
}

func (s *MockSObjectSerializeable) LoadSalesforceId(db *datastore.Datastore, id string) salesforce.SObjectCompatible {
	objects := make([]*user.User, 0)
	db.Query("user").Filter("PrimarySalesforceId_=", id).Limit(1).GetAll(&objects)
	if len(objects) == 0 {
		return nil
	}
	return objects[0]
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

func (a *MockSalesforceClient) GetBody() []byte {
	bodies := a.Params.Bodies

	var body []byte
	body, a.Params.Bodies = bodies[0], bodies[1:]
	return body
}

func (a *MockSalesforceClient) GetStatusCode() int {
	return 204
}

func (a *MockSalesforceClient) GetContext() context.Context {
	return ctx
}

var (
	ctx aetest.Context
	// user   models.User
	params *ClientParams
)

var _ = BeforeSuite(func() {
	// var err error
	// ctx, err = aetest.NewContext(&aetest.Options{StronglyConsistentDatastore: true})
	// Expect(err).ToNot(HaveOccurred())

	// user = models.User{
	// 	Id:        "Id",
	// 	FirstName: "First",
	// 	LastName:  "Last",
	// 	Phone:     "(123)-456-7890",
	// 	Email:     "dev@hanzo.ai",
	// 	BillingAddress: models.Address{
	// 		Line1:      "BillMeAt",
	// 		Line2:      "Line2",
	// 		City:       "City",
	// 		State:      "State",
	// 		PostalCode: "PostalCode",
	// 		Country:    "Country",
	// 	},
	// 	ShippingAddress: models.Address{
	// 		Line1:      "ShipMeAt",
	// 		Line2:      "Line2",
	// 		City:       "City",
	// 		State:      "State",
	// 		PostalCode: "PostalCode",
	// 		Country:    "Country",
	// 	},
	// 	SalesforceSObject: models.SalesforceSObject{
	// 		PrimarySalesforceId_: "PrimarySalesforceId",
	// 	},
	// }

	// params = new(ClientParams)
})

var _ = AfterSuite(func() {
	err := ctx.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("User (de)serialization", func() {
	// Context("Metadata", func() {
	// 	It("Should find all sobject custom fields", func() {
	// 		metadata := salesforce.GetCustomFieldMetadata(MockSObjectTypes{})

	// 		Expect(len(metadata)).To(Equal(3))
	// 		Expect(metadata[0].Name).To(Equal("String"))
	// 		Expect(metadata[0].Type).To(Equal("TEXT(255)"))
	// 		Expect(metadata[1].Name).To(Equal("Currency"))
	// 		Expect(metadata[1].Type).To(Equal("CURRENCY(16,2)"))
	// 		Expect(metadata[2].Name).To(Equal("Bool"))
	// 		Expect(metadata[2].Type).To(Equal("CHECKBOX"))
	// 	})
	// })

	// Context("Account and Contact To/From User", func() {
	// 	It("Should work", func() {
	// 		// Contact and Account should serialize and then deserialze to the original object
	// 		contact := salesforce.Contact{}
	// 		contact.Read(&user)

	// 		account := salesforce.Account{}
	// 		account.Read(&user)

	// 		u := models.User{}
	// 		contact.Write(&u)
	// 		account.Write(&u)

	// 		u.SalesforceSObject = user.SalesforceSObject

	// 		Expect(reflect.DeepEqual(user, u)).To(Equal(true))
	// 	})

	// 	It("Contact should treat HanzoIdC as ExternalId", func() {
	// 		contact := salesforce.Contact{HanzoIdC: "1234"}
	// 		Expect(contact.HanzoIdC).To(Equal(contact.ExternalId()))

	// 		contact.SetExternalId("4321")
	// 		Expect("4321").To(Equal(contact.HanzoIdC))
	// 	})

	// 	It("Account should treat HanzoIdC as ExternalId", func() {
	// 		account := salesforce.Account{HanzoIdC: "1234"}
	// 		Expect(account.HanzoIdC).To(Equal(account.ExternalId()))

	// 		account.SetExternalId("4321")
	// 		Expect("4321").To(Equal(account.HanzoIdC))
	// 	})
	// })

	// Context("Push/Pull User", func() {
	// 	It("Push Contact", func() {
	// 		params.Bodies = append(params.Bodies, []byte{})

	// 		client := MockSalesforceClient{Params: params}
	// 		contact := salesforce.Contact{}
	// 		contact.Read(&user)
	// 		contact.Push(&client)
	// 		// blank out the HanzoIdC since it is never serialized
	// 		contact.HanzoIdC = ""

	// 		// Verify that the client received the correct inputs
	// 		Expect(params.Verb).To(Equal("PATCH"))

	// 		path := fmt.Sprintf(salesforce.ContactExternalIdPath, strings.Replace(user.Id, ".", "_", -1))
	// 		Expect(params.Path).To(Equal(path))

	// 		data, _ := json.Marshal(contact)
	// 		Expect(params.Data).To(Equal(string(data[:])))

	// 		Expect(params.Headers).To(Equal(map[string]string{"Content-Type": "application/json"}))
	// 	})

	// 	It("Push Account", func() {
	// 		params.Bodies = append(params.Bodies, []byte{})

	// 		client := MockSalesforceClient{Params: params}
	// 		account := salesforce.Account{}
	// 		account.Read(&user)
	// 		account.Push(&client)
	// 		// blank out the HanzoIdC since it is never serialized
	// 		account.HanzoIdC = ""

	// 		// Verify that the client received the correct inputs
	// 		Expect(params.Verb).To(Equal("PATCH"))

	// 		path := fmt.Sprintf(salesforce.AccountExternalIdPath, strings.Replace(user.Id, ".", "_", -1))
	// 		Expect(params.Path).To(Equal(path))

	// 		data, _ := json.Marshal(account)
	// 		Expect(params.Data).To(Equal(string(data[:])))

	// 		Expect(params.Headers).To(Equal(map[string]string{"Content-Type": "application/json"}))
	// 	})

	// 	It("PullExternalId User", func() {
	// 		client := MockSalesforceClient{Params: params}
	// 		account := salesforce.Account{}
	// 		contact := salesforce.Contact{}

	// 		// Create reference objects for testing from user
	// 		refAccount := salesforce.Account{}
	// 		refAccount.Read(&user)

	// 		refContact := salesforce.Contact{}
	// 		refContact.Read(&user)

	// 		// Set the bodies to be decoded
	// 		body1, _ := json.Marshal(refAccount)
	// 		body2, _ := json.Marshal(refContact)

	// 		params.Bodies = append(params.Bodies, body1, body2)

	// 		account.PullExternalId(&client, "Id")
	// 		account.Ref = refAccount.Ref
	// 		contact.PullExternalId(&client, "Id")
	// 		contact.Ref = refContact.Ref

	// 		// Referenced and Decoded values should be equal
	// 		Expect(reflect.DeepEqual(account, refAccount)).To(Equal(true))
	// 		Expect(reflect.DeepEqual(contact, refContact)).To(Equal(true))
	// 	})

	// 	It("PullId User", func() {
	// 		client := MockSalesforceClient{Params: params}
	// 		account := salesforce.Account{}
	// 		contact := salesforce.Contact{}

	// 		// Create reference objects for testing from user
	// 		refAccount := salesforce.Account{}
	// 		refAccount.HanzoIdC = "Id"
	// 		refAccount.Read(&user)

	// 		refContact := salesforce.Contact{}
	// 		refContact.HanzoIdC = "Id"
	// 		refContact.Read(&user)

	// 		// Set the bodies to be decoded
	// 		body1, _ := json.Marshal(refAccount)
	// 		body2, _ := json.Marshal(refContact)

	// 		params.Bodies = append(params.Bodies, body1, body2)

	// 		account.PullId(&client, "Id")
	// 		account.Ref = refAccount.Ref
	// 		contact.PullId(&client, "Id")
	// 		contact.Ref = refContact.Ref

	// 		// Referenced and Decoded values should be equal
	// 		Expect(reflect.DeepEqual(account, refAccount)).To(Equal(true))
	// 		Expect(reflect.DeepEqual(contact, refContact)).To(Equal(true))
	// 	})
	// })

	// Context("Salesforce API", func() {
	// 	It("PullUpdated with nothing in the DB", func() {
	// 		db := datastore.New(ctx)
	// 		key := db.NewKey("user", "NOT IN THE DB", 0, nil)
	// 		id := key.Encode()
	// 		client := MockSalesforceClient{Params: params}

	// 		response := salesforce.UpdatedRecordsResponse{
	// 			Ids: []string{"PrimarySalesforceId"},
	// 		}

	// 		users := make(map[string]salesforce.SObjectCompatible)
	// 		err := salesforce.ProcessUpdatedSObjects(
	// 			&client,
	// 			&response,
	// 			time.Now(),
	// 			users,
	// 			func() salesforce.SObjectLoadable {
	// 				so := new(MockSObjectSerializeable)
	// 				so.ExpectedId = id
	// 				so.ExpectedFirstName = "SOME NAME"
	// 				return so
	// 			})

	// 		Expect(err).ToNot(HaveOccurred())

	// 		so, ok := users[id]
	// 		Expect(ok).To(Equal(true))
	// 		u, ok := so.(*models.User)
	// 		Expect(ok).To(Equal(true))

	// 		// Only the FirstName is updated for the MockSObjectSerializeable
	// 		// FirstName should therefore be the only set field
	// 		refUser := models.User{FirstName: "SOME NAME"}
	// 		u.LastSync_ = refUser.LastSync_
	// 		u.SalesforceSObject = refUser.SalesforceSObject

	// 		log.Warn("%v\n\n%v", refUser, u)
	// 		Expect(reflect.DeepEqual(&refUser, u)).To(Equal(true))
	// 	})

	// 	It("PullUpdated with something in the DB", func() {
	// 		db := datastore.New(ctx)
	// 		key := db.NewKey("user", "Id", 0, nil)
	// 		client := MockSalesforceClient{Params: params}

	// 		// PullUpdated will update a record in db, so add a record to the db that is slightly different than the master user
	// 		someUser := models.User{
	// 			Id:                user.Id,
	// 			FirstName:         "Bad First Name",
	// 			LastName:          user.LastName,
	// 			Phone:             user.Phone,
	// 			Email:             user.Email,
	// 			BillingAddress:    user.BillingAddress,
	// 			ShippingAddress:   user.ShippingAddress,
	// 			SalesforceSObject: user.SalesforceSObject,
	// 		}

	// 		someUser.LastSync_ = time.Now().Add(-1 * time.Hour)

	// 		// Insert into DB
	// 		db.Put(key, &someUser)
	// 		defer db.Delete(key)

	// 		response := salesforce.UpdatedRecordsResponse{
	// 			Ids: []string{"PrimarySalesforceId"},
	// 		}

	// 		users := make(map[string]salesforce.SObjectCompatible)
	// 		err := salesforce.ProcessUpdatedSObjects(
	// 			&client,
	// 			&response,
	// 			time.Now(),
	// 			users,
	// 			func() salesforce.SObjectLoadable {
	// 				so := new(MockSObjectSerializeable)
	// 				so.ExpectedId = user.Id
	// 				so.ExpectedFirstName = user.FirstName
	// 				return so
	// 			})

	// 		Expect(err).ToNot(HaveOccurred())

	// 		// The updated user should look identical to the master user
	// 		so, ok := users[user.Id]
	// 		Expect(ok).To(Equal(true))
	// 		u, ok := so.(*models.User)
	// 		Expect(ok).To(Equal(true))

	// 		// The Datastore initializes these values differently so set them to what they should be
	// 		u.Cart = user.Cart
	// 		u.Campaigns = user.Campaigns
	// 		u.PasswordHash = user.PasswordHash
	// 		u.UpdatedAt = user.UpdatedAt
	// 		u.CreatedAt = user.CreatedAt
	// 		u.Metadata = user.Metadata
	// 		u.LastSync_ = user.LastSync_
	// 		u.SalesforceSObject = user.SalesforceSObject

	// 		Expect(reflect.DeepEqual(&user, u)).To(Equal(true))
	// 	})
	// })
})
