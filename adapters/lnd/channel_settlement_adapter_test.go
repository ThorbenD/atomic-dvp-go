package lnd_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	adapterlnd "github.com/ThorbenD/atomic-dvp-go/adapters/lnd"
	"github.com/ThorbenD/atomic-dvp-go/clients/tapd"
	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	channelSwapID      = "channel-swap-id-abc"
	channelPreimage    = "channel-preimage-hex-string-here-32byte"
	channelPaymentHash = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12"
	testAssetID        = "deadbeefdeadbeefdeadbeefdeadbeef"
	testPeerPubkey     = "021c1075c2e173ea3be32cfaeec528b1e4c70d47d0de0ba88a381cd29cb01e4a19"
)

func channelTestHTLC() *domain.HTLC {
	return &domain.HTLC{
		Hash:       channelPaymentHash,
		Amount:     50_000,
		Status:     domain.HTLCStatusConfirmed,
		DetectedAt: time.Now(),
	}
}

func TestChannelSettlementAdapter_Capabilities(t *testing.T) {
	adapter := adapterlnd.NewChannelSettlementAdapter(nil, nil, nil)
	caps := adapter.Capabilities()
	assert.Equal(t, uint64(16_777_215), caps.MaxTicketSizeSats)
	assert.Equal(t, "HTLC_TAPROOT_CHANNEL", caps.SettlementType)
	assert.False(t, caps.SupportsFreeze)
	assert.False(t, caps.SupportsClawback)
}

func TestChannelSettlementAdapter_PrepareSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	mockWatcher.On("DetectHTLC", mock.Anything, channelPaymentHash).
		Return(channelTestHTLC(), nil)

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	req := settlement.SettlementRequest{
		SwapID:      channelSwapID,
		PaymentHash: channelPaymentHash,
		Preimage:    channelPreimage,
		AssetID:     testAssetID,
		AssetAmount: decimal.NewFromFloat(0.01),
		DestAddr:    testPeerPubkey,
	}

	handle, err := adapter.PrepareSettlement(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, channelSwapID, handle.SwapID)
	assert.Equal(t, channelPreimage, handle.Preimage)
	assert.Equal(t, channelPaymentHash, handle.PaymentHash)
	assert.Equal(t, "HTLC_TAPROOT_CHANNEL", handle.DriverType)
	assert.Equal(t, uint64(50_000), handle.DepositAmtSats)
	assert.Contains(t, handle.ID, channelPaymentHash[:16])
	mockWatcher.AssertExpectations(t)
}

func TestChannelSettlementAdapter_PrepareSettlement_DetectError(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	mockWatcher.On("DetectHTLC", mock.Anything, channelPaymentHash).
		Return(nil, fmt.Errorf("invoice expired"))

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	req := settlement.SettlementRequest{
		SwapID:      channelSwapID,
		PaymentHash: channelPaymentHash,
		Preimage:    channelPreimage,
	}

	_, err := adapter.PrepareSettlement(context.Background(), req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prepare settlement failed")
}

func TestChannelSettlementAdapter_ExecuteSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	// First: prepare (stores request in pending map)
	mockWatcher.On("DetectHTLC", mock.Anything, channelPaymentHash).
		Return(channelTestHTLC(), nil)

	// Then: execute
	expectedSendReq := tapd.ChannelSendRequest{
		AssetID:     testAssetID,
		Amount:      1_000_000, // 0.01 * 10^8
		PeerPubkey:  testPeerPubkey,
		PaymentHash: channelPaymentHash,
	}
	mockChannel.On("SendAssetViaChannel", mock.Anything, expectedSendReq).
		Return(&tapd.ChannelSendResult{Preimage: "abcdef1234567890abcdef12345678"}, nil)

	mockWatcher.On("ClaimHTLC", mock.Anything, channelPreimage).
		Return("off-chain-settled", nil)

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	req := settlement.SettlementRequest{
		SwapID:      channelSwapID,
		PaymentHash: channelPaymentHash,
		Preimage:    channelPreimage,
		AssetID:     testAssetID,
		AssetAmount: decimal.NewFromFloat(0.01),
		DestAddr:    testPeerPubkey,
	}

	handle, err := adapter.PrepareSettlement(context.Background(), req)
	require.NoError(t, err)

	result, err := adapter.ExecuteSettlement(context.Background(), handle)
	require.NoError(t, err)
	assert.Equal(t, "HTLC_TAPROOT_CHANNEL", result.DriverType)
	assert.Equal(t, "CLAIMED", result.FinalState)
	assert.Contains(t, result.TxID, "off-chain-settled")
	mockWatcher.AssertExpectations(t)
	mockChannel.AssertExpectations(t)
}

func TestChannelSettlementAdapter_ExecuteSettlement_NoPendingEntry(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	handle := &settlement.SettlementHandle{
		ID:         "channel_htlc_test",
		SwapID:     "nonexistent-swap",
		DriverType: "HTLC_TAPROOT_CHANNEL",
	}

	_, err := adapter.ExecuteSettlement(context.Background(), handle)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending request")
}

func TestChannelSettlementAdapter_ExecuteSettlement_ZeroAmount(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	mockWatcher.On("DetectHTLC", mock.Anything, channelPaymentHash).
		Return(channelTestHTLC(), nil)

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	req := settlement.SettlementRequest{
		SwapID:      channelSwapID,
		PaymentHash: channelPaymentHash,
		Preimage:    channelPreimage,
		AssetID:     testAssetID,
		AssetAmount: decimal.NewFromFloat(0), // zero → error
		DestAddr:    testPeerPubkey,
	}

	handle, err := adapter.PrepareSettlement(context.Background(), req)
	require.NoError(t, err)

	_, err = adapter.ExecuteSettlement(context.Background(), handle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestChannelSettlementAdapter_AbortSettlement_Success(t *testing.T) {
	mockWatcher := &MockChainWatcher{}
	mockLnd := &MockLightningClient{}
	mockChannel := &MockChannelSender{}

	// Prepare first to populate pending map
	mockWatcher.On("DetectHTLC", mock.Anything, channelPaymentHash).
		Return(channelTestHTLC(), nil)
	mockLnd.On("CancelInvoice", mock.Anything, channelPaymentHash).Return(nil)

	adapter := adapterlnd.NewChannelSettlementAdapter(mockWatcher, mockLnd, mockChannel)
	req := settlement.SettlementRequest{
		SwapID:      channelSwapID,
		PaymentHash: channelPaymentHash,
		Preimage:    channelPreimage,
		AssetAmount: decimal.NewFromFloat(0.01),
		DestAddr:    testPeerPubkey,
	}

	handle, err := adapter.PrepareSettlement(context.Background(), req)
	require.NoError(t, err)

	err = adapter.AbortSettlement(context.Background(), handle)
	require.NoError(t, err)

	// Execute after abort should fail (pending cleared)
	_, err = adapter.ExecuteSettlement(context.Background(), handle)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending request")

	mockLnd.AssertExpectations(t)
}
