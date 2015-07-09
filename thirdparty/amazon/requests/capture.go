package requests

import "crowdstart.io/thirdparty/amazon/types"

type CaptureRequest struct {
	AmazonAuthorizationId string      //Amazon-generated auth id for a transaction
	CaptureReferenceId    string      //Crowdstart-generated auth.  Max length: 32 chars
	CaptureAmount         types.Price //Amount being authed
	SellerCaptureNote     string      // Description for the capture transaction displayed in emails to the buyer.  Max 255 chars
	SoftDescriptor        string      //Payment description if CaptureNow is true.  max 16 chars
}
