package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keep the contract test close to the requested public command shape. The
// concrete *sql.Tx parameter is asserted by the production interface.
func TestGrsaiBalanceHoldCommandHasSettlementIdentity(t *testing.T) {
	cmd := GrsaiBalanceHoldCommand{SettlementID: 7, UserID: 8, APIKeyID: 9, Amount: 1.25, IdempotencyKey: "grsai_hold:7"}
	require.Equal(t, int64(7), cmd.SettlementID)
	require.Equal(t, "grsai_hold:7", cmd.IdempotencyKey)
}

func TestGrsaiHoldOperationKeysAreStable(t *testing.T) {
	require.Equal(t, "grsai_hold:7", GrsaiHoldReserveRequestID(7))
	require.Equal(t, "grsai_capture:7", GrsaiHoldCaptureRequestID(7))
	require.Equal(t, "grsai_release:7", GrsaiHoldReleaseRequestID(7))
}

func TestGrsaiHoldErrorsRemainDistinguishable(t *testing.T) {
	require.ErrorIs(t, ErrGrsaiInsufficientBalance, ErrInsufficientBalance)
	require.NotEqual(t, ErrGrsaiInsufficientBalance, errors.New("x"))
}
