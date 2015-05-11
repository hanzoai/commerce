package migrations

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"appengine"
	aeds "appengine/datastore"

	olddatastore "crowdstart.io/datastore"
	oldparallel "crowdstart.io/datastore/parallel"
	oldmodels "crowdstart.io/models"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models"
	"crowdstart.com/models/bundle"
	"crowdstart.com/models/namespace"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/types/weight"
	"crowdstart.com/models/user"
	"crowdstart.com/models/variant"
	"crowdstart.com/util/log"
	"crowdstart.com/util/task"
)

var skullyIntId = 420
var skullyNamespace = strconv.Itoa(skullyIntId)

var _ = task.Func("migrate-skully-1-org-products-users", func(c *gin.Context) {
	oldparallel.Run(c, "campaign", 50, migrateSkullyOrg)
	oldparallel.Run(c, "products", 50, migrateSkullyProducts)
	oldparallel.Run(c, "users", 50, migrateSkullyUsers)
})

var _ = task.Func("migrate-skully-2-listings-orders", func(c *gin.Context) {
	oldparallel.Run(c, "listings", 50, migrateSkullyOrg)
	oldparallel.Run(c, "orders", 50, migrateSkullyProducts)
})

var migrateSkullyOrg = oldparallel.Task("migrate-skully-org-task", func(odb *olddatastore.Datastore, key olddatastore.Key, campaign oldmodels.Campaign) {
	db := ds.New(odb.Context)
	org := organization.New(db)

	// Anal ytics
	org.FacebookTag = campaign.FacebookTag
	org.GoogleAnalytics = campaign.GoogleAnalytics

	// Stripe
	org.Stripe.AccessToken = campaign.Stripe.AccessToken
	org.Stripe.PublishableKey = campaign.Stripe.PublishableKey
	org.Stripe.RefreshToken = campaign.Stripe.RefreshToken
	org.Stripe.UserId = campaign.Stripe.UserId

	// SF
	org.Salesforce.AccessToken = campaign.Salesforce.AccessToken
	org.Salesforce.RefreshToken = campaign.Salesforce.RefreshToken
	org.Salesforce.InstanceUrl = campaign.Salesforce.InstanceUrl
	org.Salesforce.Id = campaign.Salesforce.Id
	org.Salesforce.IssuedAt = campaign.Salesforce.IssuedAt
	org.Salesforce.Signature = campaign.Salesforce.Signature
	org.Salesforce.DefaultPriceBookId = campaign.Salesforce.DefaultPriceBookId

	// Basic Info
	org.Name = "skully"
	org.FullName = "SKULLY"
	org.Enabled = true
	org.Website = "http://www.skully.com"

	if err := db.RunInTransaction(func(ctx appengine.Context) error {
		if err := org.Put(); err != nil {
			return err
		}

		// Save namespace so we can decode keys for this organization later
		ns := namespace.New(db)
		ns.Name = org.Name
		ns.IntId = org.Key().IntID()
		if err := ns.Put(); err != nil {
			return err
		}

		return odb.Delete(key)
	}, &aeds.TransactionOptions{}); err != nil {
		log.Error("Error %v", err, db.Context)
	}
})

var migrateSkullyUsers = oldparallel.Task("migrate-skully-user", func(odb *olddatastore.Datastore, key olddatastore.Key, ou oldmodels.User) {
	db := ds.New(odb.Context)
	u := user.New(db)

	// Contact Info
	u.FirstName = ou.FirstName
	u.LastName = ou.LastName
	u.Phone = ou.Phone

	// Addresses
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

	// Login
	u.Email = ou.Email
	u.PasswordHash = ou.PasswordHash
	u.CreatedAt = ou.CreatedAt

	// SObject
	u.Salesforce.PrimarySalesforceId_ = ou.SalesforceSObject.PrimarySalesforceId_
	u.Salesforce.SecondarySalesforceId_ = ou.SalesforceSObject.SecondarySalesforceId_
	u.Salesforce.ExternalId_ = ou.Id
	u.Salesforce.LastSync_ = ou.SalesforceSObject.LastSync_

	// Stripe Ids
	u.Accounts.Stripe.CustomerId = ou.Stripe.CustomerId

	if err := db.RunInTransaction(func(ctx appengine.Context) error {
		u.SetNamespace(skullyNamespace)
		if err := u.Put(); err != nil {
			return err
		}
		return odb.Delete(key)
	}, &aeds.TransactionOptions{}); err != nil {
		log.Error("Error %v", err, db.Context)
	}
})

func AddOption(p *product.Product, v *variant.Variant, name, value string) {
	v.Options = append(v.Options, variant.Option{Name: name, Value: value})

	for i, option := range p.Options {
		if option.Name == name {
			p.Options[i].Values = append(p.Options[i].Values, value)
			return
		}
	}

	p.Options = append(p.Options, &product.Option{Name: name, Values: []string{value}})
}

var migrateSkullyProducts = oldparallel.Task("migrate-skully-product", func(odb *olddatastore.Datastore, key olddatastore.Key, op oldmodels.Product) {
	db := ds.New(odb.Context)
	p := product.New(db)

	// Identifier
	p.Slug = op.Slug
	p.SKU = op.Slug

	// Prices
	p.Currency = currency.USD
	p.Price = currency.Cents(op.MinPrice())
	p.ListPrice = p.Price

	// Text Fields
	p.Headline = op.Headline
	p.Excerpt = op.Excerpt
	p.Description = op.Description
	p.Available = op.Available
	p.AddLabel = op.AddLabel

	// Structs
	p.Options = make([]*product.Option, 0)
	p.Variants = make([]*variant.Variant, len(op.Variants))

	pid := p.Id()
	if err := db.RunInTransaction(func(ctx appengine.Context) error {
		for i, ov := range op.Variants {
			v := variant.New(db)
			vkey := odb.NewKey("variants", ov.SKU, 0, nil)
			if err := odb.Get(vkey, &ov); err != nil {
				log.Error("%v Error", err, db.Context)
				continue
			}

			// SObjects
			v.Salesforce.PrimarySalesforceId_ = ov.SalesforceSObject.PrimarySalesforceId_
			v.Salesforce.SecondarySalesforceId_ = ov.SalesforceSObject.SecondarySalesforceId_
			v.Salesforce.ExternalId_ = ov.Id
			v.Salesforce.LastSync_ = ov.SalesforceSObject.LastSync_

			// Identifier
			v.ProductId = pid
			v.SKU = ov.SKU
			v.Name = ov.SKU

			// Prices
			v.Currency = currency.USD
			v.Price = currency.Cents(ov.Price)

			// Volume/Masses
			v.Dimensions = ov.Dimensions
			v.Weight = weight.Mass(ov.Weight)
			v.WeightUnit = weight.Pound

			// Options on Variants
			v.Options = make([]variant.Option, 0)
			for _, option := range v.Options {
				AddOption(p, v, option.Name, option.Value)
			}

			p.Variants[i] = v

			v.SetNamespace(skullyNamespace)
			if err := v.Put(); err != nil {
				return err
			}

			if err := odb.Delete(vkey); err != nil {
				return err
			}
		}

		p.SetNamespace(skullyNamespace)
		if err := p.Put(); err != nil {
			return err
		}
		return odb.Delete(key)
	}, &aeds.TransactionOptions{}); err != nil {
		log.Error("Error %v", err, db.Context)
	}
})

var migrateSkullyListings = oldparallel.Task("migrate-skully-listing", func(odb *olddatastore.Datastore, key olddatastore.Key, l oldmodels.Listing) {
	db := ds.New(odb.Context)
	b := bundle.New(db)

	b.Slug = l.SKU
	b.Name = l.Title
	b.Description = l.Description

	b.Hidden = l.Disabled
	b.Available = !l.SoldOut

	b.ProductIds = make([]string, 0)
	b.VariantIds = make([]string, 0)

	if err := db.RunInTransaction(func(ctx appengine.Context) error {
		for _, config := range l.Configs {
			if config.Variant != "" {
				v := product.New(db)
				v.SetNamespace(skullyNamespace)
				if ok, err := v.Query().Filter("SKU=", config.Variant).First(); !ok {
					return err
				}
				for i := 0; i < config.Quantity; i++ {
					b.VariantIds = append(b.VariantIds, v.Id())
				}
			} else {
				p := product.New(db)
				p.SetNamespace(skullyNamespace)
				if ok, err := p.Query().Filter("SKU=", config.Product).First(); !ok {
					return err
				}
				for i := 0; i < config.Quantity; i++ {
					b.ProductIds = append(b.ProductIds, p.Id())
				}
			}
		}

		b.SetNamespace(skullyNamespace)
		if err := b.Put(); err != nil {
			return err
		}
		return odb.Delete(key)
	}, &aeds.TransactionOptions{}); err != nil {
		log.Error("Error %v", err, db.Context)
	}
})

var migrateSkullyOrders = oldparallel.Task("migrate-skully-order", func(odb *olddatastore.Datastore, key olddatastore.Key, oo oldmodels.Order) {
	db := ds.New(odb.Context)
	o := order.New(db)

	// SObjects
	o.Salesforce.PrimarySalesforceId_ = oo.SalesforceSObject.PrimarySalesforceId_
	o.Salesforce.SecondarySalesforceId_ = oo.SalesforceSObject.SecondarySalesforceId_
	o.Salesforce.ExternalId_ = oo.Id
	o.Salesforce.LastSync_ = oo.SalesforceSObject.LastSync_

	// Addresses
	o.BillingAddress.Line1 = oo.BillingAddress.Line1
	o.BillingAddress.Line2 = oo.BillingAddress.Line2
	o.BillingAddress.City = oo.BillingAddress.City
	o.BillingAddress.PostalCode = oo.BillingAddress.PostalCode
	o.BillingAddress.Country = oo.BillingAddress.Country

	o.ShippingAddress.Line1 = oo.ShippingAddress.Line1
	o.ShippingAddress.Line2 = oo.ShippingAddress.Line2
	o.ShippingAddress.City = oo.ShippingAddress.City
	o.ShippingAddress.PostalCode = oo.ShippingAddress.PostalCode
	o.ShippingAddress.Country = oo.ShippingAddress.Country

	// Status, descending
	if oo.Refunded {
		o.Status = order.Cancelled
		o.PaymentStatus = payment.Refunded
	} else if oo.Cancelled {
		o.Status = order.Cancelled
		if oo.Disputed {
			o.PaymentStatus = payment.Fraudulent
		} else {
			o.PaymentStatus = payment.Failed
		}
	} else if oo.Disputed {
		o.Status = order.Open
		o.PaymentStatus = payment.Disputed
	} else if oo.Locked {
		o.Status = order.Locked
		o.PaymentStatus = payment.Paid
	} else {
		o.Status = order.Open
		o.PaymentStatus = payment.Paid
	}
	o.FulfillmentStatus = models.FulfillmentUnfulfilled

	// Preorder/Configured
	o.Unconfirmed = oo.Unconfirmed
	o.Preorder = oo.Preorder

	// Invoice
	o.Currency = currency.USD
	o.Shipping = currency.Cents(oo.Shipping)
	o.Tax = currency.Cents(oo.Tax)
	o.Subtotal = currency.Cents(oo.Subtotal)
	o.Total = currency.Cents(oo.Total)

	o.PaymentIds = make([]string, len(oo.Charges))

	oid := o.Id()

	if err := db.RunInTransaction(func(ctx appengine.Context) error {
		u := user.New(db)
		if ok, err := u.Query().Filter("Salesforce.ExternalId_=", oo.UserId).First(); !ok {
			return err
		}

		o.UserId = u.Id()

		for i, charge := range oo.Charges {
			p := payment.New(db)

			// Dispute stuff
			p.Type = payment.Stripe
			p.Currency = currency.USD
			p.Amount = currency.Cents(charge.Amount)
			p.AmountRefunded = currency.Cents(charge.AmountRefunded)

			p.Account.ChargeId = charge.ID
			p.Live = charge.Live
			p.Captured = charge.Captured

			if charge.FailCode != "" {
				p.Status = payment.Failed
			} else if charge.Refunded {
				if charge.Disputed {
					p.Status = payment.Fraudulent
				} else {
					p.Status = payment.Refunded
				}
			} else if charge.Disputed {
				p.Status = payment.Disputed
			} else if charge.Paid {
				p.Status = payment.Paid
			}

			// Buyer
			p.Buyer.Email = charge.Email
			p.Buyer.UserId = o.UserId
			p.Buyer.FirstName = u.FirstName
			p.Buyer.LastName = u.LastName
			p.Buyer.Company = u.Company
			p.Buyer.Phone = u.Phone

			p.Buyer.Address.Line1 = u.ShippingAddress.Line1
			p.Buyer.Address.Line2 = u.ShippingAddress.Line2
			p.Buyer.Address.City = u.ShippingAddress.City
			p.Buyer.Address.PostalCode = u.ShippingAddress.PostalCode
			p.Buyer.Address.Country = u.ShippingAddress.Country

			p.OrderId = oid

			p.SetNamespace(skullyNamespace)
			if err := p.Put(); err != nil {
				return err
			}

			o.PaymentIds[i] = p.Id()
		}

		o.SetNamespace(skullyNamespace)
		if err := o.Put(); err != nil {
			return err
		}
		return odb.Delete(key)
	}, &aeds.TransactionOptions{}); err != nil {
		log.Error("Error %v", err, db.Context)
	}
})
