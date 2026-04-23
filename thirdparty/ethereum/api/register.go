package api

import (
	commerceapi "github.com/hanzoai/commerce/api/api"
)

func init() {
	commerceapi.RegisterRoute(Route)
}
