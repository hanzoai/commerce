package campaign

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
	"github.com/hanzoai/commerce/util/slug"
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
