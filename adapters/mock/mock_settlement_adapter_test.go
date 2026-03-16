package mock_test

import (
	"context"
	"testing"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/adapters/mock"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAdapter() (*mock.MockSettlementAdapter, string, string) {
	watcher := mock.NewMockChainWatcher()
	adapter := mock.NewMockSettlementAdapter(watcher)
	hash := validHash()
	preimage := validPreimage
	return adapter, hash, preimage
}

func TestMockSettlementAdapter_Capabilities_Defaults(t *testing.T) {
	adapter, _, _ := newTestAdapter()
	caps := adapter.Capabilities()
	assert.Equal(t, uint64(1_000_000), caps.MaxTicketSizeSats)
	assert.Equal(t, "HTLC_LIGHTNING", caps.SettlementType)
	assert.False(t, caps.SupportsFreeze)
	assert.False(t, caps.SupportsClawback)
}

func TestMockSettlementAdapter_SetCapabilities(t *testing.T) {
	adapter, _, _ := newTestAdapter()
	adapter.SetCapabilities(settlement.DriverCapabilities{
		MaxTicketSizeSats: 16_777_215,
		SettlementType:    "HTLC_TAPROOT_CHANNEL",
	})
	caps := adapter.Capabilities()
	assert.Equal(t, uint64(16_777_215), caps.MaxTicketSizeSats)
	assert.Equal(t, "HTLC_TAPROOT_CHANNEL", caps.SettlementType)
}

func TestMockSettlementAdapter_PrepareSettlement(t *testing.T) {
	adapter, hash, preimage := newTestAdapter()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		adapter.SimulateDeposit(hash, 80_000)
	}()

	req := settlement.SettlementRequest{
		SwapID:      preimage,
		PaymentHash: hash,
		AssetID:     "asset-1",
		AssetAmount: decimal.NewFromFloat(1.0),
	}

	handle, err := adapter.PrepareSettlement(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, preimage, handle.SwapID)
	assert.Equal(t, "HTLC_LIGHTNING", handle.DriverType)
	assert.Equal(t, uint64(80_000), handle.DepositAmtSats)
	assert.Contains(t, handle.ID, hash[:8])
}

func TestMockSettlementAdapter_ExecuteSettlement(t *testing.T) {
	adapter, hash, preimage := newTestAdapter()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		adapter.SimulateDeposit(hash, 80_000)
	}()

	req := settlement.SettlementRequest{
		SwapID:      preimage,
		PaymentHash: hash,
	}

	handle, err := adapter.PrepareSettlement(ctx, req)
	require.NoError(t, err)

	result, err := adapter.ExecuteSettlement(ctx, handle)
	require.NoError(t, err)
	assert.Equal(t, "HTLC_LIGHTNING", result.DriverType)
	assert.Equal(t, "CLAIMED", result.FinalState)
	assert.Contains(t, result.TxID, "tx_mock_sweep_")
}

func TestMockSettlementAdapter_AbortSettlement_NoOp(t *testing.T) {
	adapter, _, preimage := newTestAdapter()
	handle := &settlement.SettlementHandle{
		SwapID:     preimage,
		DriverType: "HTLC_LIGHTNING",
	}
	err := adapter.AbortSettlement(context.Background(), handle)
	assert.NoError(t, err)
}
