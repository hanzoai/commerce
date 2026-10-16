package square

import (
	"errors"
	"net/http"
	"testing"

	"github.com/square/square-go-sdk/v3/core"
)

// Square's own no is an answer: nothing was done. A server error, a timeout or a
// dropped connection may have done it, so it is not.
func TestSaidMarksOnlySquaresOwnRefusals(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{core.NewAPIError(http.StatusPaymentRequired, nil, errors.New("CARD_DECLINED")), true},
		{core.NewAPIError(http.StatusBadRequest, nil, errors.New("payment is CANCELED")), true},
		{core.NewAPIError(http.StatusTooManyRequests, nil, errors.New("RATE_LIMITED")), true},
		{core.NewAPIError(http.StatusRequestTimeout, nil, errors.New("timeout")), false},
		{core.NewAPIError(http.StatusInternalServerError, nil, errors.New("boom")), false},
		{core.NewAPIError(http.StatusBadGateway, nil, errors.New("bad gateway")), false},
		{errors.New("connection reset"), false},
	} {
		var a interface{ Answered() bool }
		got := errors.As(said(tc.err), &a) && a.Answered()
		if got != tc.want {
			t.Errorf("said(%v) answered=%v, want %v", tc.err, got, tc.want)
		}
	}
}
