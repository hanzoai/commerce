package requests

import (
	"crowdstart.com/thirdparty/amazon/types"
)

type AuthorizeRequest struct {
	AmazonOrderReferenceId   string      //Amazon-generated auth id for a transaction
	AuthorizationReferenceId string      //Crowdstart-generated auth.  Max length: 32 chars
	AuthorizationAmount      types.Price //Amount being authed
	TransactionTimeout       uint        //Maximum number of minutes for the Authorize to be processed, before it's invalid. default 1440
	CaptureNow               bool        //Capture right now y/n
	SoftDescriptor           string      //Payment description if CaptureNow is true.  max 16 chars
}
