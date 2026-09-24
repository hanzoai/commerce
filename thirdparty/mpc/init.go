package mpc

import (
	"github.com/hanzoai/commerce/payment/processor"
)

// The rail registers itself from the environment, through the one function that
// reads it. Nothing is assembled here: a second place that built a Config would
// be a second place a setting could be forgotten, which is how MPC_ZAP_ADDR
// could have been added to the config type and still done nothing.
func init() {
	processor.Register(NewProcessor(DefaultConfig()))
}
