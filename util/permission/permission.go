package permission

import (
	"crowdstart.io/util/bit"
)

const (
	Admin     bit.Mask = 1 << iota // 1 << 0 which is 00000001
	Published                      // 1 << 1 which is 00000010
	Authorize                      // 1 << 2 which is 00000100
	Charge                         // 1 << 3 which is 00001000
	Capture                        // ..etc
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
