package migrations

// import (
// 	"github.com/gin-gonic/gin"

// 	"appengine"
// 	aeds "appengine/datastore"

// 	olddatastore "crowdstart.io/datastore"
// 	oldparallel "crowdstart.io/datastore/parallel"
// 	oldmodels "crowdstart.io/models"

// 	"crowdstart.com/auth/password"
// 	ds "crowdstart.com/datastore"
// 	"crowdstart.com/models"
// 	"crowdstart.com/models/bundle"
// 	"crowdstart.com/models/lineitem"
// 	"crowdstart.com/models/mixin"
// 	"crowdstart.com/models/namespace"
// 	"crowdstart.com/models/order"
// 	"crowdstart.com/models/organization"
// 	"crowdstart.com/models/payment"
// 	"crowdstart.com/models/product"
// 	"crowdstart.com/models/types/currency"
// 	"crowdstart.com/models/types/weight"
// 	"crowdstart.com/models/user"
// 	"crowdstart.com/models/variant"
// 	"crowdstart.com/util/log"
// 	"crowdstart.com/util/task"
// )

// var skullyIntId = 420
// var skullyNamespace = "skully"

// var _ = task.Func("migrate-skully-1-org-products-users", func(c *gin.Context) {
// 	oldparallel.Run(c, "campaign", 50, migrateSkullyOrg)
// 	oldparallel.Run(c, "product", 50, migrateSkullyProducts)
// 	oldparallel.Run(c, "user", 50, migrateSkullyUsers)
// })

// var _ = task.Func("migrate-skully-2-listings-orders", func(c *gin.Context) {
// 	oldparallel.Run(c, "listing", 50, migrateSkullyListings)
// 	oldparallel.Run(c, "order", 50, migrateSkullyOrders)
// })

// var migrateSkullyOrg = oldparallel.Task("migrate-skully-org-task", func(odb *olddatastore.Datastore, key olddatastore.Key, campaign oldmodels.Campaign) {
// 	db := ds.New(odb.Context)
// 	org := organization.New(db)

// 	// Anal ytics
// 	org.FacebookTag = campaign.FacebookTag
// 	org.GoogleAnalytics = campaign.GoogleAnalytics

// 	// Stripe
// 	org.Stripe.AccessToken = campaign.Stripe.AccessToken
// 	org.Stripe.PublishableKey = campaign.Stripe.PublishableKey
// 	org.Stripe.RefreshToken = campaign.Stripe.RefreshToken
// 	org.Stripe.UserId = campaign.Stripe.UserId

// 	// SF
// 	org.Salesforce.AccessToken = campaign.Salesforce.AccessToken
// 	org.Salesforce.RefreshToken = campaign.Salesforce.RefreshToken
// 	org.Salesforce.InstanceUrl = campaign.Salesforce.InstanceUrl
// 	org.Salesforce.Id = campaign.Salesforce.Id
// 	org.Salesforce.IssuedAt = campaign.Salesforce.IssuedAt
// 	org.Salesforce.Signature = campaign.Salesforce.Signature
// 	org.Salesforce.DefaultPriceBookId = campaign.Salesforce.DefaultPriceBookId

// 	// Basic Info
// 	org.Name = "skully"
// 	org.FullName = "SKULLY"
// 	org.Enabled = true
// 	org.Website = "http://www.skully.com"
// 	org.SecretKey = []byte("9ul9k12F8gGp0r5sIM4x34hDqR7tJK5f")

// 	if err := db.RunInTransaction(func(ctx appengine.Context) error {
// 		org.SetKey(skullyIntId)

// 		u := user.New(db)
// 		u.Email = "dev@hanzo.ai"
// 		u.GetOrCreate("Email=", u.Email)
// 		u.FirstName = "Mitchell"
// 		u.LastName = "Weller"
// 		u.Organizations = []string{org.Id()}
// 		u.PasswordHash, _ = password.Hash("Ducati1!")
// 		u.Put()

// 		org.Owners = []string{u.Id()}
// 		org.AddDefaultTokens()

// 		if err := org.Put(); err != nil {
// 			return err
// 		}

// 		// Save namespace so we can decode keys for this organization later
// 		ns := namespace.New(db)
// 		ns.Name = org.Name
// 		ns.IntId = org.Key().IntID()
// 		if err := ns.Put(); err != nil {
// 			return err
// 		}

// 		return nil
// 		//return odb.Delete(key)
// 	}, &aeds.TransactionOptions{}); err != nil {
// 		log.Error("Error %v", err, db.Context)
// 	}
// })

// var migrateSkullyUsers = oldparallel.Task("migrate-skully-user", func(odb *olddatastore.Datastore, key olddatastore.Key, ou oldmodels.User) {
// 	if ou.Email == "dev@hanzo.ai" {
// 		return
// 	}

// 	db := ds.New(odb.Context)
// 	u := user.New(db)
// 	u.SetNamespace(skullyNamespace)

// 	// Contact Info
// 	u.FirstName = ou.FirstName
// 	u.LastName = ou.LastName
// 	u.Phone = ou.Phone

// 	// Addresses
// 	u.BillingAddress.Line1 = ou.BillingAddress.Line1
// 	u.BillingAddress.Line2 = ou.BillingAddress.Line2
// 	u.BillingAddress.City = ou.BillingAddress.City
// 	u.BillingAddress.PostalCode = ou.BillingAddress.PostalCode
// 	u.BillingAddress.Country = ou.BillingAddress.Country

// 	u.ShippingAddress.Line1 = ou.ShippingAddress.Line1
// 	u.ShippingAddress.Line2 = ou.ShippingAddress.Line2
// 	u.ShippingAddress.City = ou.ShippingAddress.City
// 	u.ShippingAddress.PostalCode = ou.ShippingAddress.PostalCode
// 	u.ShippingAddress.Country = ou.ShippingAddress.Country

// 	// Login
// 	u.Email = ou.Email
// 	u.PasswordHash = ou.PasswordHash
// 	u.CreatedAt = ou.CreatedAt

// 	// SObject
// 	u.Salesforce.PrimarySalesforceId_ = ou.SalesforceSObject.PrimarySalesforceId_
// 	u.Salesforce.SecondarySalesforceId_ = ou.SalesforceSObject.SecondarySalesforceId_
// 	u.Salesforce.ExternalId_ = ou.Id
// 	u.Salesforce.LastSync_ = ou.SalesforceSObject.LastSync_

// 	// Stripe Ids
// 	u.Accounts.Stripe.CustomerId = ou.Stripe.CustomerId

// 	if err := db.RunInTransaction(func(ctx appengine.Context) error {
// 		if err := u.Put(); err != nil {
// 			return err
// 		}
// 		return nil
// 		//return odb.Delete(key)
// 	}, &aeds.TransactionOptions{}); err != nil {
// 		log.Error("Error %v", err, db.Context)
// 	}
// })

// func AddOption(p *product.Product, v *variant.Variant, name, value string) {
// 	v.Options = append(v.Options, variant.Option{Name: name, Value: value})

// 	for i, option := range p.Options {
// 		if option.Name == name {
// 			p.Options[i].Values = append(p.Options[i].Values, value)
// 			return
// 		}
// 	}

// 	p.Options = append(p.Options, &product.Option{Name: name, Values: []string{value}})
// }

// var migrateSkullyProducts = oldparallel.Task("migrate-skully-product", func(odb *olddatastore.Datastore, key olddatastore.Key, op oldmodels.Product) {
// 	db := ds.New(odb.Context)
// 	p := product.New(db)
// 	p.SetNamespace(skullyNamespace)

// 	// Identifier
// 	p.Name = op.Slug
// 	p.Slug = op.Slug
// 	p.SKU = op.Slug

// 	// Prices
// 	p.Currency = currency.USD
// 	p.Price = currency.Cents(op.MinPrice() / 100)
// 	p.ListPrice = p.Price

// 	// Text Fields
// 	p.Headline = op.Headline
// 	p.Excerpt = op.Excerpt
// 	p.Description = op.Description
// 	p.Available = true
// 	p.AddLabel = op.AddLabel

// 	// Structs
// 	p.Options = make([]*product.Option, 0)
// 	p.Variants = make([]*variant.Variant, len(op.Variants))

// 	pid := p.Id()
// 	if err := db.RunInTransaction(func(ctx appengine.Context) error {
// 		for i, ov := range op.Variants {
// 			v := variant.New(db)
// 			v.SetNamespace(skullyNamespace)

// 			vkey := odb.NewKey("variant", ov.SKU, 0, nil)
// 			if err := odb.Get(vkey, &ov); err != nil {
// 				log.Error("%v Error", err, db.Context)
// 				continue
// 			}

// 			// SObjects
// 			v.Salesforce.PrimarySalesforceId_ = ov.SalesforceSObject.PrimarySalesforceId_
// 			v.Salesforce.SecondarySalesforceId_ = ov.SalesforceSObject.SecondarySalesforceId_
// 			v.Salesforce.ExternalId_ = ov.Id
// 			v.Salesforce.LastSync_ = ov.SalesforceSObject.LastSync_

// 			// Identifier
// 			v.ProductId = pid
// 			v.SKU = ov.SKU
// 			v.Name = ov.SKU

// 			// Prices
// 			v.Currency = currency.USD
// 			v.Price = currency.Cents(ov.Price / 100)

// 			// Volume/Masses
// 			v.Dimensions = ov.Dimensions
// 			v.Weight = weight.Mass(ov.Weight)
// 			v.WeightUnit = weight.Pound

// 			// Options on Variants
// 			v.Options = make([]variant.Option, 0)
// 			for _, option := range v.Options {
// 				AddOption(p, v, option.Name, option.Value)
// 			}

// 			p.Variants[i] = v

// 			if err := v.Put(); err != nil {
// 				return err
// 			}

// 			// if err := odb.Delete(vkey); err != nil {
// 			// 	return err
// 			// }
// 		}

// 		if err := p.Put(); err != nil {
// 			return err
// 		}
// 		return nil
// 		//return odb.Delete(key)
// 	}, &aeds.TransactionOptions{}); err != nil {
// 		log.Error("Error %v", err, db.Context)
// 	}
// })

// var migrateSkullyListings = oldparallel.Task("migrate-skully-listing", func(odb *olddatastore.Datastore, key olddatastore.Key, l oldmodels.Listing) {
// 	db := ds.New(odb.Context)
// 	b := bundle.New(db)

// 	b.Slug = l.SKU
// 	b.Name = l.Title
// 	b.Description = l.Description

// 	b.Hidden = l.Disabled
// 	b.Available = !l.SoldOut

// 	b.ProductIds = make([]string, 0)
// 	b.VariantIds = make([]string, 0)

// 	if err := db.RunInTransaction(func(ctx appengine.Context) error {
// 		for _, config := range l.Configs {
// 			if config.Variant != "" {
// 				v := variant.New(db)
// 				v.SetNamespace(skullyNamespace)
// 				if ok, err := v.Query().Filter("SKU=", config.Variant).First(); !ok {
// 					log.Warn("!ok or error %v for %v", err, config.Variant, db.Context)
// 					return err
// 				}
// 				for i := 0; i < config.Quantity; i++ {
// 					b.VariantIds = append(b.VariantIds, v.Id())
// 				}
// 			} else {
// 				p := product.New(db)
// 				p.SetNamespace(skullyNamespace)
// 				if ok, err := p.Query().Filter("SKU=", config.Product).First(); !ok {
// 					log.Warn("!ok or error %v for %v", err, config.Product, db.Context)
// 					return err
// 				}
// 				for i := 0; i < config.Quantity; i++ {
// 					b.ProductIds = append(b.ProductIds, p.Id())
// 				}
// 			}
// 		}

// 		b.SetNamespace(skullyNamespace)
// 		if err := b.Put(); err != nil {
// 			return err
// 		}
// 		return nil
// 		//return odb.Delete(key)
// 	}, &aeds.TransactionOptions{}); err != nil {
// 		log.Error("Error %v", err, db.Context)
// 	}
// })

// var migrateSkullyOrders = oldparallel.Task("migrate-skully-order", func(odb *olddatastore.Datastore, key olddatastore.Key, oo oldmodels.Order) {
// 	db := ds.New(odb.Context)
// 	o := order.New(db)
// 	o.SetNamespace(skullyNamespace)

// 	// SObjects
// 	o.Salesforce.PrimarySalesforceId_ = oo.SalesforceSObject.PrimarySalesforceId_
// 	o.Salesforce.SecondarySalesforceId_ = oo.SalesforceSObject.SecondarySalesforceId_
// 	o.Salesforce.ExternalId_ = oo.Id
// 	o.Salesforce.LastSync_ = oo.SalesforceSObject.LastSync_

// 	// Addresses
// 	o.BillingAddress.Line1 = oo.BillingAddress.Line1
// 	o.BillingAddress.Line2 = oo.BillingAddress.Line2
// 	o.BillingAddress.City = oo.BillingAddress.City
// 	o.BillingAddress.PostalCode = oo.BillingAddress.PostalCode
// 	o.BillingAddress.Country = oo.BillingAddress.Country

// 	o.ShippingAddress.Line1 = oo.ShippingAddress.Line1
// 	o.ShippingAddress.Line2 = oo.ShippingAddress.Line2
// 	o.ShippingAddress.City = oo.ShippingAddress.City
// 	o.ShippingAddress.PostalCode = oo.ShippingAddress.PostalCode
// 	o.ShippingAddress.Country = oo.ShippingAddress.Country

// 	// Status, descending
// 	if oo.Refunded {
// 		o.Status = order.Cancelled
// 		o.PaymentStatus = payment.Refunded
// 	} else if oo.Cancelled {
// 		o.Status = order.Cancelled
// 		if oo.Disputed {
// 			o.PaymentStatus = payment.Fraudulent
// 		} else {
// 			o.PaymentStatus = payment.Failed
// 		}
// 	} else if oo.Disputed {
// 		o.Status = order.Open
// 		o.PaymentStatus = payment.Disputed
// 	} else if oo.Locked {
// 		o.Status = order.Locked
// 		o.PaymentStatus = payment.Paid
// 	} else {
// 		o.Status = order.Open
// 		o.PaymentStatus = payment.Paid
// 	}
// 	o.FulfillmentStatus = models.FulfillmentUnfulfilled

// 	// Preorder/Configured
// 	o.Unconfirmed = oo.Unconfirmed
// 	o.Preorder = oo.Preorder

// 	// Invoice
// 	o.Currency = currency.USD
// 	o.Shipping = currency.Cents(oo.Shipping / 100)
// 	o.Tax = currency.Cents(oo.Tax / 100)
// 	o.Subtotal = currency.Cents(oo.Subtotal / 100)
// 	o.Total = currency.Cents(oo.Total / 100)

// 	o.Items = make([]lineitem.LineItem, len(oo.Items))
// 	o.PaymentIds = make([]string, len(oo.Charges))

// 	oid := o.Id()

// 	if err := db.RunInTransaction(func(ctx appengine.Context) error {
// 		u := user.New(db)
// 		if ok, err := u.Query().Filter("ExternalId_=", oo.UserId).First(); !ok {
// 			log.Warn("!ok or error %v for %v", err, oo.UserId, db.Context)
// 			if ok, err := u.Query().Filter("Email=", oo.Email).First(); !ok {
// 				log.Warn("!ok or error %v for %v", err, oo.UserId, db.Context)
// 				return err
// 			}
// 		}

// 		o.UserId = u.Id()

// 		for i, item := range oo.Items {
// 			o.Items[i] = lineitem.LineItem{
// 				ProductSlug: item.Slug_,
// 				VariantSKU:  item.SKU_,
// 				Quantity:    item.Quantity,
// 				// SObjects
// 				Salesforce: mixin.Salesforce{
// 					PrimarySalesforceId_:   oo.SalesforceSObject.PrimarySalesforceId_,
// 					SecondarySalesforceId_: oo.SalesforceSObject.SecondarySalesforceId_,
// 					LastSync_:              oo.SalesforceSObject.LastSync_,
// 				},
// 			}

// 			if item.Slug_ != "" {
// 				p := product.New(db)
// 				p.SetNamespace(skullyNamespace)
// 				if ok, err := p.Query().Filter("SKU=", item.Slug_).First(); !ok {
// 					log.Warn("!ok or error %v for %v", err, item.Slug_, db.Context)
// 					return err
// 				}

// 				o.Items[i].VariantName = p.Name
// 				o.Items[i].VariantId = p.Id()
// 				o.Items[i].Taxable = p.Taxable
// 				o.Items[i].Price = p.Price
// 				o.Items[i].Weight = p.Weight
// 				o.Items[i].WeightUnit = p.WeightUnit
// 				o.Items[i].Taxable = p.Taxable
// 			}

// 			if item.SKU_ != "" {
// 				v := variant.New(db)
// 				v.SetNamespace(skullyNamespace)
// 				if ok, err := v.Query().Filter("SKU=", item.SKU_).First(); !ok {
// 					log.Warn("!ok or error %v for %v", err, item.SKU_, db.Context)
// 					return err
// 				}

// 				o.Items[i].ProductName = v.Name
// 				o.Items[i].ProductId = v.Id()
// 				o.Items[i].Price = v.Price
// 				o.Items[i].Weight = v.Weight
// 				o.Items[i].WeightUnit = v.WeightUnit
// 				o.Items[i].Taxable = v.Taxable
// 			}
// 		}

// 		for i, charge := range oo.Charges {
// 			p := payment.New(db)
// 			p.SetNamespace(skullyNamespace)

// 			// Dispute stuff
// 			p.Type = payment.Stripe
// 			p.Currency = currency.USD
// 			p.Amount = currency.Cents(charge.Amount / 100)
// 			p.AmountRefunded = currency.Cents(charge.AmountRefunded / 100)

// 			p.Account.ChargeId = charge.ID
// 			p.Live = charge.Live
// 			p.Captured = charge.Captured

// 			if charge.FailCode != "" {
// 				p.Status = payment.Failed
// 			} else if charge.Refunded {
// 				if charge.Disputed {
// 					p.Status = payment.Fraudulent
// 				} else {
// 					p.Status = payment.Refunded
// 				}
// 			} else if charge.Disputed {
// 				p.Status = payment.Disputed
// 			} else if charge.Paid {
// 				p.Status = payment.Paid
// 			}

// 			// Buyer
// 			p.Buyer.Email = charge.Email
// 			p.Buyer.UserId = o.UserId
// 			p.Buyer.FirstName = u.FirstName
// 			p.Buyer.LastName = u.LastName
// 			p.Buyer.Company = u.Company
// 			p.Buyer.Phone = u.Phone

// 			p.Buyer.Address.Line1 = u.ShippingAddress.Line1
// 			p.Buyer.Address.Line2 = u.ShippingAddress.Line2
// 			p.Buyer.Address.City = u.ShippingAddress.City
// 			p.Buyer.Address.PostalCode = u.ShippingAddress.PostalCode
// 			p.Buyer.Address.Country = u.ShippingAddress.Country

// 			p.OrderId = oid

// 			if err := p.Put(); err != nil {
// 				return err
// 			}

// 			o.PaymentIds[i] = p.Id()
// 		}

// 		if err := o.Put(); err != nil {
// 			return err
// 		}
// 		return nil
// 		//return odb.Delete(key)
// 	}, &aeds.TransactionOptions{}); err != nil {
// 		log.Error("Error %v", err, db.Context)
// 	}
// })
