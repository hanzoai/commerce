package responses

import "crowdstart.com/thirdparty/amazon/types"

type CaptureResponse struct {
	CaptureDetails types.CaptureDetails // Encapsulates details about the Capture object and its status
}
