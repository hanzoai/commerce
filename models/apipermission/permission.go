package apipermission

import (
	"github.com/hanzoai/commerce/models/mixin"
)

type ApiPermission struct {
	mixin.Model

	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"` // "read", "write", "delete", "manage"
}
