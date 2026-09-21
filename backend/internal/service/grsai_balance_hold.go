package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrGrsaiInsufficientBalance = ErrInsufficientBalance

type GrsaiBalanceHoldCommand struct {
	SettlementID   int64
	UserID         int64
	APIKeyID       int64
	Amount         float64
	IdempotencyKey string
}

type GrsaiBalanceHoldResult struct {
	Applied    bool
	NewBalance *float64
	Frozen     *float64
}

type GrsaiHoldBillingRepository interface {
	ReserveGrsaiBalance(context.Context, *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error)
	CaptureGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error)
	ReleaseGrsaiBalanceTx(context.Context, *sql.Tx, *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error)
}

type grsaiHoldReleaseRepository interface {
	ReleaseGrsaiBalance(context.Context, *GrsaiBalanceHoldCommand) (*GrsaiBalanceHoldResult, error)
}

type GrsaiHoldStateRepository interface {
	MarkHoldHeld(context.Context, int64, int64) error
	CloseNoChargeWithRelease(context.Context, int64, int64, string, GrsaiSettlementTxFunc) error
	MarkManualReviewWithRelease(context.Context, int64, int64, string, GrsaiSettlementTxFunc) error
}

func GrsaiHoldReserveRequestID(id int64) string { return fmt.Sprintf("grsai_hold:%d", id) }
func GrsaiHoldCaptureRequestID(id int64) string { return fmt.Sprintf("grsai_capture:%d", id) }
func GrsaiHoldReleaseRequestID(id int64) string { return fmt.Sprintf("grsai_release:%d", id) }

func (s *GrsaiSettlementService) holdBilling() GrsaiHoldBillingRepository {
	if s == nil {
		return nil
	}
	if s.HoldBilling != nil {
		return s.HoldBilling
	}
	h, _ := s.Billing.(GrsaiHoldBillingRepository)
	return h
}

func grsaiHoldCommand(record *GrsaiSettlement, operation string) (*GrsaiBalanceHoldCommand, error) {
	if record == nil || record.ID <= 0 || record.UserID <= 0 || record.APIKeyID <= 0 || !grsaiFiniteNonNegative(record.HoldAmount) {
		return nil, ErrGrsaiSettlementInvalidInput
	}
	return &GrsaiBalanceHoldCommand{SettlementID: record.ID, UserID: record.UserID, APIKeyID: record.APIKeyID,
		Amount: record.HoldAmount, IdempotencyKey: operation}, nil
}

func (s *GrsaiSettlementService) reserveGrsaiBalance(ctx context.Context, record *GrsaiSettlement) error {
	h := s.holdBilling()
	if h == nil || record.HoldAmount <= 0 {
		return nil
	}
	cmd, err := grsaiHoldCommand(record, GrsaiHoldReserveRequestID(record.ID))
	if err != nil {
		return err
	}
	_, err = h.ReserveGrsaiBalance(ctx, cmd)
	if errors.Is(err, ErrBatchImageInsufficientBalance) || errors.Is(err, ErrInsufficientBalance) {
		return ErrGrsaiInsufficientBalance
	}
	return err
}

func (s *GrsaiSettlementService) captureGrsaiBalanceTx(ctx context.Context, tx *sql.Tx, record *GrsaiSettlement) error {
	h := s.holdBilling()
	if h == nil || record.HoldAmount <= 0 {
		return nil
	}
	cmd, err := grsaiHoldCommand(record, GrsaiHoldCaptureRequestID(record.ID))
	if err != nil {
		return err
	}
	_, err = h.CaptureGrsaiBalanceTx(ctx, tx, cmd)
	return err
}

func (s *GrsaiSettlementService) releaseGrsaiBalanceTx(ctx context.Context, tx *sql.Tx, record *GrsaiSettlement) error {
	h := s.holdBilling()
	if h == nil || record.HoldAmount <= 0 {
		return nil
	}
	cmd, err := grsaiHoldCommand(record, GrsaiHoldReleaseRequestID(record.ID))
	if err != nil {
		return err
	}
	_, err = h.ReleaseGrsaiBalanceTx(ctx, tx, cmd)
	return err
}

func (s *GrsaiSettlementService) releaseGrsaiBalance(ctx context.Context, record *GrsaiSettlement) error {
	h := s.holdBilling()
	if h == nil || record == nil || record.HoldAmount <= 0 {
		return nil
	}
	cmd, err := grsaiHoldCommand(record, GrsaiHoldReleaseRequestID(record.ID))
	if err != nil {
		return err
	}
	if direct, ok := h.(grsaiHoldReleaseRepository); ok {
		_, err = direct.ReleaseGrsaiBalance(ctx, cmd)
		return err
	}
	return errors.New("grsai hold repository cannot release outside transaction")
}
