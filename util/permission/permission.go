package permission

import (
	"crowdstart.com/util/bit"
)

// There are three types of users that permissions commonly accomodate:
// 1. Anonymous users (via published keys)
// 2. Clients using API on their server (via secret keys)
// 3. Crowdstart (Complete access)
const (
	Any  bit.Mask = ^0
	None bit.Mask = 0

	Live      bit.Mask = 1 << iota // 1 << 0 which is 00000001
	Test                           // 1 << 1 which is 00000010
	Admin                          // 1 << 2 which is 00000100
	Published                      // 1 << 3 which is 00001000
	Authorize                      // ..etc
	Charge
	Capture

	// List permissions
	Bundle
	Collection
	Order
	Payment
	Product
	Store
	User
	Variant

	// Read/Write permissions
	ReadBundle
	ReadCampaign
	ReadCollection
	ReadCoupon
	ReadMailingList
	ReadOrder
	ReadOrganization
	ReadPayment
	ReadPlan
	ReadProduct
	ReadStore
	ReadSubscriber
	ReadToken
	ReadUser
	ReadVariant

	WriteBundle
	WriteCampaign
	WriteCollection
	WriteCoupon
	WriteMailingList
	WriteOrder
	WriteOrganization
	WritePayment
	WritePlan
	WriteProduct
	WriteStore
	WriteSubscriber
	WriteToken
	WriteUser
	WriteVariant
)

type Permission struct {
	Kind    string
	Methods []string
}

var Map = map[bit.Mask]Permission{
	Bundle:     Permission{"bundle", []string{"GET", "PUT"}},
	Collection: Permission{"collection", []string{"GET", "PUT"}},
	Order:      Permission{"order", []string{"GET", "PUT"}},
	Payment:    Permission{"payment", []string{"GET", "PUT"}},
	Product:    Permission{"product", []string{"GET", "PUT"}},
	Store:      Permission{"store", []string{"GET", "PUT"}},
	User:       Permission{"user", []string{"GET", "PUT"}},
	Variant:    Permission{"variant", []string{"GET", "PUT"}},

	// Read/Write permissions
	ReadBundle:       Permission{"bundle", []string{"GET"}},
	ReadCampaign:     Permission{"campaign", []string{"GET"}},
	ReadCollection:   Permission{"collection", []string{"GET"}},
	ReadCoupon:       Permission{"coupon", []string{"GET"}},
	ReadMailingList:  Permission{"mailinglist", []string{"GET"}},
	ReadOrder:        Permission{"order", []string{"GET"}},
	ReadOrganization: Permission{"organization", []string{"GET"}},
	ReadPayment:      Permission{"payment", []string{"GET"}},
	ReadPlan:         Permission{"plan", []string{"GET"}},
	ReadProduct:      Permission{"product", []string{"GET"}},
	ReadStore:        Permission{"store", []string{"GET"}},
	ReadSubscriber:   Permission{"subscriber", []string{"GET"}},
	ReadToken:        Permission{"token", []string{"GET"}},
	ReadUser:         Permission{"user", []string{"GET"}},
	ReadVariant:      Permission{"variant", []string{"GET"}},

	WriteBundle:       Permission{"bundle", []string{"PATCH", "POST", "PUT"}},
	WriteCampaign:     Permission{"campaign", []string{"PATCH", "POST", "PUT"}},
	WriteCollection:   Permission{"collection", []string{"PATCH", "POST", "PUT"}},
	WriteCoupon:       Permission{"coupon", []string{"PATCH", "POST", "PUT"}},
	WriteMailingList:  Permission{"mailinglist", []string{"PATCH", "POST", "PUT"}},
	WriteOrder:        Permission{"order", []string{"PATCH", "POST", "PUT"}},
	WriteOrganization: Permission{"organization", []string{"PATCH", "POST", "PUT"}},
	WritePayment:      Permission{"payment", []string{"PATCH", "POST", "PUT"}},
	WritePlan:         Permission{"plan", []string{"PATCH", "POST", "PUT"}},
	WriteProduct:      Permission{"product", []string{"PATCH", "POST", "PUT"}},
	WriteStore:        Permission{"store", []string{"PATCH", "POST", "PUT"}},
	WriteSubscriber:   Permission{"subscriber", []string{"PATCH", "POST", "PUT"}},
	WriteToken:        Permission{"token", []string{"PATCH", "POST", "PUT"}},
	WriteUser:         Permission{"user", []string{"PATCH", "POST", "PUT"}},
	WriteVariant:      Permission{"variant", []string{"PATCH", "POST", "PUT"}},
}
