package migrations

import (
	"strconv"

	olddatastore "crowdstart.io/datastore"
	oldparallel "crowdstart.io/datastore/parallel"
	oldmodels "crowdstart.io/models"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
)

var skullyIntId = 420
var skullyNamespace = strconv.Itoa(skullyIntId)

var _ = oldparallel.Task("migrate-skully-org-task", func(odb *olddatastore.Datastore, key olddatastore.Key, campaign oldmodels.Campaign) {
	db := ds.New(odb.Context)
	org := organization.New(db)

	org.FacebookTag = campaign.FacebookTag
	org.GoogleAnalytics = campaign.GoogleAnalytics

	org.Stripe.AccessToken = campaign.Stripe.AccessToken
	org.Stripe.PublishableKey = campaign.Stripe.PublishableKey
	org.Stripe.RefreshToken = campaign.Stripe.RefreshToken
	org.Stripe.UserId = campaign.Stripe.UserId

	org.Salesforce.AccessToken = campaign.Salesforce.AccessToken
	org.Salesforce.RefreshToken = campaign.Salesforce.RefreshToken
	org.Salesforce.InstanceUrl = campaign.Salesforce.InstanceUrl
	org.Salesforce.Id = campaign.Salesforce.Id
	org.Salesforce.IssuedAt = campaign.Salesforce.IssuedAt
	org.Salesforce.Signature = campaign.Salesforce.Signature
	org.Salesforce.DefaultPriceBookId = campaign.Salesforce.DefaultPriceBookId

	org.Name = "skully"
	org.FullName = "SKULLY"
	org.Enabled = true
	org.Website = "http://www.skully.com"

	org.Put()
})

var _ = oldparallel.Task("migrate-skully-user", func(odb *olddatastore.Datastore, key olddatastore.Key, ou oldmodels.User) {
	db := ds.New(odb.Context)
	u := user.New(db)

	u.FirstName = ou.FirstName
	u.LastName = ou.LastName
	u.Phone = ou.Phone

	u.BillingAddress.Line1 = ou.BillingAddress.Line1
	u.BillingAddress.Line2 = ou.BillingAddress.Line2
	u.BillingAddress.City = ou.BillingAddress.City
	u.BillingAddress.PostalCode = ou.BillingAddress.PostalCode
	u.BillingAddress.Country = ou.BillingAddress.Country

	u.ShippingAddress.Line1 = ou.ShippingAddress.Line1
	u.ShippingAddress.Line2 = ou.ShippingAddress.Line2
	u.ShippingAddress.City = ou.ShippingAddress.City
	u.ShippingAddress.PostalCode = ou.ShippingAddress.PostalCode
	u.ShippingAddress.Country = ou.ShippingAddress.Country

	u.Email = ou.Email
	u.PasswordHash = ou.PasswordHash
	u.CreatedAt = ou.CreatedAt

	u.Salesforce.PrimarySalesforceId_ = ou.SalesforceSObject.PrimarySalesforceId_
	u.Salesforce.SecondarySalesforceId_ = ou.SalesforceSObject.SecondarySalesforceId_
	u.Salesforce.ExternalId_ = ou.Id
	u.Salesforce.LastSync_ = ou.SalesforceSObject.LastSync_

	u.Accounts.Stripe.CustomerId = ou.Stripe.CustomerId

	//left off here

	u.SetNamespace(skullyNamespace)
	u.Put()
})
