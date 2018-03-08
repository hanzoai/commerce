package test

import (
	"testing"

	"golang.org/x/net/context"

	"hanzo.io/datastore"
	"hanzo.io/models/fixtures"
	"hanzo.io/models/organization"
	"hanzo.io/models/segment"
	"hanzo.io/models/subscriber"
	"hanzo.io/models/trigger"
	"hanzo.io/models/user"
	"hanzo.io/util/gincontext"
	"hanzo.io/util/test/ae"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("events", t)
}

var (
	ctx  context.Context
	inst ae.Instance
	db   *datastore.Datastore
	org  *organization.Organization
	seg  *segment.Segment
	trg  *trigger.Trigger
)

// Setup appengine context and datastore before tests
var _ = BeforeSuite(func() {
	var err error
	ctx, inst, err = ae.NewContext(ae.Options{
		Modules:    []string{"default"},
		TaskQueues: []string{"default"},
		LogChild:   true,
	})
	Expect(err).NotTo(HaveOccurred())

	// Mock gin context that we can use with fixtures
	c := gincontext.New(ctx)
	org = fixtures.Organization(c).(*organization.Organization)

	seg = fixtures.Segment(c).(*segment.Segment)
	trg = fixtures.Trigger(c).(*trigger.Trigger)

	nsCtx := org.Namespaced(ctx)
	db = datastore.New(nsCtx)
})

// Tear-down appengine context
var _ = AfterSuite(func() {
	inst.Close()
})

var _ = Describe("Event", func() {
	It("User Should Be Added As Subscriber To Segment", func() {
		s := subscriber.New(db)

		usr := user.New(db)
		usr.FirstName = "Test"
		usr.MustCreate()

		err := Retry(10, func() error {
			_, err := s.Query().Filter("UserId=", usr.Id()).Filter("SegmentId=", seg.Id()).First()
			return err
		})
		Expect(err).ToNot(HaveOccurred())
	})
})
