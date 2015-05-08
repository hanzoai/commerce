package permission

import (
	"crowdstart.com/util/bit"
)

const (
	Any       bit.Mask = ^0
	None      bit.Mask = 0
	Live      bit.Mask = 1 << iota // 1 << 0 which is 00000001
	Test                           // 1 << 1 which is 00000010
	Admin                          // 1 << 2 which is 00000100
	Published                      // 1 << 3 which is 00001000
	Authorize                      // ..etc
	Charge
	Capture
	ReadCampaign
	ReadCollection
	ReadCoupon
	ReadOrder
	ReadOrganization
	ReadPlan
	ReadProduct
	ReadToken
	ReadUser
	ReadVariant
	WriteCampaign
	WriteCollection
	WriteCoupon
	WriteOrder
	WriteOrganization
	WritePlan
	WriteProduct
	WriteToken
	WriteUser
	WriteVariant
)
