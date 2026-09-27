package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"
)

var ErrGrsaiInsufficientBalance = errors.New("insufficient balance for grsai task")

type GrsaiBalanceHoldRepository interface {
	ReserveGrsaiBalance(context.Context, *sql.Tx, *GrsaiSettlement) error
	CaptureGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiSettlement) error
	ReleaseGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiSettlement) error
}

func GrsaiTaskHoldAmount(record *GrsaiSettlement) (float64, error) {
	if record == nil || record.LocalTaskID == nil || *record.LocalTaskID == "" || record.UserID <= 0 ||
		record.RequestedImageCount <= 0 || !grsaiFiniteNonNegative(record.BillableUnitPrice) {
		return 0, ErrGrsaiSettlementInvalidInput
	}
	amount, _ := decimal.NewFromFloat(record.BillableUnitPrice).Mul(decimal.NewFromInt(int64(record.RequestedImageCount))).Round(8).Float64()
	if !grsaiFiniteNonNegative(amount) {
		return 0, ErrGrsaiSettlementInvalidInput
	}
	return amount, nil
}

func GrsaiTaskUsageCommand(record *GrsaiSettlement) (*UsageBillingCommand, error) {
	cmd, err := grsaiBillingCommand(record)
	if err != nil {
		return nil, err
	}
	if record.LocalTaskID == nil || *record.LocalTaskID == "" {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	amount, err := GrsaiTaskHoldAmount(record)
	if err != nil {
		return nil, err
	}
	cmd.RequestID = "grsai_task:" + *record.LocalTaskID
	cmd.BalanceCost = 0
	cmd.APIKeyQuotaCost = amount
	cmd.APIKeyRateLimitCost = amount
	cmd.RequestFingerprint = ""
	cmd.Normalize()
	return cmd, nil
}
