package responses

import "crowdstart.com/thirdparty/amazon/types"

type AuthorizeResponse struct {
	AuthorizationDetails types.AuthorizationDetails // Details about the Authorization object
}
