package lnd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	swapID      = "swap-id-abc123"
	preimage    = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	paymentHash = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"
)

func testHTLC(amount uint64) *domain.HTLC {
	return &domain.HTLC{
		Hash:       paymentHash,
		Amount:     amount,
		Status:     domain.HTLCStatusConfirmed,
		DetectedAt: time.Now(),
	}
}

func testRequest() settlement.SettlementRequest {
	return settlement.SettlementRequest{
		SwapID:      swapID,
		PaymentHash: paymentHash,
		Preimage:    preimage,
	}
}

func TestLndSettlementAdapter_Capabilities(t *testing.T) {
	adapter := adapterlnd.NewLndSettlementAdapter(nil, nil)
	caps := adapter.Capabilities()
	assert.Equal(t, uint64(1_000_000), caps.MaxTicketSizeSats)
	assert.Equal(t, "HTLC_LIGHTNING", caps.SettlementType)
	assert.False(t, caps.SupportsFreeze)
	assert.False(t, caps.SupportsClawback)
}

func TestLndSettlementAdapter_PrepareSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockWatcher.On("DetectHTLC", mock.Anything, paymentHash).
		Return(testHTLC(75_000), nil)

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)

	handle, err := adapter.PrepareSettlement(context.Background(), testRequest())

	require.NoError(t, err)
	assert.Equal(t, swapID, handle.SwapID)
	assert.Equal(t, preimage, handle.Preimage)
	assert.Equal(t, paymentHash, handle.PaymentHash)
	assert.Equal(t, "HTLC_LIGHTNING", handle.DriverType)
	assert.Equal(t, uint64(75_000), handle.DepositAmtSats)
	assert.Contains(t, handle.ID, paymentHash[:16])
	mockWatcher.AssertExpectations(t)
}

func TestLndSettlementAdapter_PrepareSettlement_DetectError(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockWatcher.On("DetectHTLC", mock.Anything, paymentHash).
		Return(nil, fmt.Errorf("invoice not found"))

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)

	_, err := adapter.PrepareSettlement(context.Background(), testRequest())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prepare settlement failed")
}

func TestLndSettlementAdapter_ExecuteSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockWatcher.On("ClaimHTLC", mock.Anything, preimage).
		Return("off-chain-settled", nil)

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)
	handle := &settlement.SettlementHandle{
		ID:          "htlc_" + paymentHash[:16],
		SwapID:      swapID,
		DriverType:  "HTLC_LIGHTNING",
		Preimage:    preimage,
		PaymentHash: paymentHash,
	}

	result, err := adapter.ExecuteSettlement(context.Background(), handle)

	require.NoError(t, err)
	assert.Equal(t, "off-chain-settled", result.TxID)
	assert.Equal(t, "HTLC_LIGHTNING", result.DriverType)
	assert.Equal(t, "CLAIMED", result.FinalState)
	mockWatcher.AssertExpectations(t)
}

func TestLndSettlementAdapter_ExecuteSettlement_ClaimError(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockWatcher.On("ClaimHTLC", mock.Anything, preimage).
		Return("", fmt.Errorf("preimage mismatch"))

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)
	handle := &settlement.SettlementHandle{
		SwapID:      swapID,
		DriverType:  "HTLC_LIGHTNING",
		Preimage:    preimage,
		PaymentHash: paymentHash,
	}

	_, err := adapter.ExecuteSettlement(context.Background(), handle)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execute settlement failed")
}

func TestLndSettlementAdapter_AbortSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockLnd.On("CancelInvoice", mock.Anything, paymentHash).Return(nil)

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)
	handle := &settlement.SettlementHandle{
		SwapID:      swapID,
		Preimage:    preimage,
		PaymentHash: paymentHash,
	}

	err := adapter.AbortSettlement(context.Background(), handle)

	require.NoError(t, err)
	mockLnd.AssertExpectations(t)
}

func TestLndSettlementAdapter_AbortSettlement_Error(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}

	mockLnd.On("CancelInvoice", mock.Anything, paymentHash).
		Return(fmt.Errorf("invoice not found"))

	adapter := adapterlnd.NewLndSettlementAdapter(mockWatcher, mockLnd)
	handle := &settlement.SettlementHandle{SwapID: swapID, PaymentHash: paymentHash}

	err := adapter.AbortSettlement(context.Background(), handle)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "abort settlement failed")
}
