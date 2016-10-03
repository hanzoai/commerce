package campaign

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
	"crowdstart.com/util/slug"
)

func Fake(db *datastore.Datastore, organizationId string) *Campaign {
	c := New(db)
	c.Title = fake.Words(3)
	c.Slug = slug.Slugify(c.Title)
	c.Approved = true
	c.Enabled = true
	c.OrganizationId = organizationId
	c.Tagline = fake.Sentence()
	c.Description = fake.Sentences(4)
	return c
}
