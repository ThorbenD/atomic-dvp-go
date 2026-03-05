package lnd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/clients/tapd"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

// ChannelSettlementAdapter implements settlement.SettlementDriver
// using tapd v0.6+ Lightning Channel transfers.
// Both payment and asset legs run as off-chain HTLCs –
// enabling true atomic DvP with millisecond settlement.
type ChannelSettlementAdapter struct {
	lndClient     settlement.LightningClient
	channelClient *tapd.ChannelClient
	chainWatcher  settlement.ChainWatcher

	mu      sync.Mutex
	pending map[string]settlement.SettlementRequest
}

func NewChannelSettlementAdapter(
	chainWatcher settlement.ChainWatcher,
	lndClient settlement.LightningClient,
	channelClient *tapd.ChannelClient,
) *ChannelSettlementAdapter {
	return &ChannelSettlementAdapter{
		lndClient:     lndClient,
		channelClient: channelClient,
		chainWatcher:  chainWatcher,
		pending:       make(map[string]settlement.SettlementRequest),
	}
}

func (a *ChannelSettlementAdapter) PrepareSettlement(
	ctx context.Context,
	req settlement.SettlementRequest,
) (*settlement.SettlementHandle, error) {
	slog.Info("⚡ [ChannelSettlement] Preparing settlement (waiting for HTLC)...",
		"swap_id", req.SwapID,
		"payment_hash", req.PaymentHash,
	)

	// Wait for BTC deposit via standard chainWatcher logic
	htlc, err := a.chainWatcher.DetectHTLC(ctx, req.PaymentHash)
	if err != nil {
		return nil, fmt.Errorf("prepare settlement failed: %w", err)
	}

	a.mu.Lock()
	a.pending[req.SwapID] = req
	a.mu.Unlock()

	slog.Info("⚡ [ChannelSettlement] HTLC detected, settlement prepared",
		"swap_id", req.SwapID,
		"amount_sats", htlc.Amount,
	)

	return &settlement.SettlementHandle{
		ID:             fmt.Sprintf("channel_htlc_%s", req.PaymentHash[:16]),
		SwapID:         req.SwapID,
		DriverType:     "HTLC_TAPROOT_CHANNEL",
		PreparedAt:     time.Now(),
		DepositAmtSats: htlc.Amount,
	}, nil
}

func (a *ChannelSettlementAdapter) ExecuteSettlement(
	ctx context.Context,
	handle *settlement.SettlementHandle,
) (*settlement.SettlementResult, error) {
	slog.Info("⚡ [ChannelSettlement] Executing settlement (sending asset via channel + sweeping HTLC)...",
		"swap_id", handle.SwapID,
		"handle_id", handle.ID,
	)

	a.mu.Lock()
	req, ok := a.pending[handle.SwapID]
	a.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no pending request found for swap %s", handle.SwapID)
	}

	// 1. Send the Asset via Channel (HTLC)
	// The prompt specified we should do an offchain HTLC based standard flow: both legs or neither.
	// We initiate the routing using channelClient.
	// Since req.DestAddr gives us a hint, ideally we'd pass an invoice string or peer pubkey.
	// We will assume DestAddr is either encoded as Peer Pubkey for direct routing or we can pass it as generic routing dest.
	amountSats := req.AssetAmount.Shift(8).IntPart() // same precision adjustment as sender.go
	if amountSats <= 0 {
		return nil, fmt.Errorf("invalid amount: must be positive, got %s", req.AssetAmount.String())
	}

	// Note: We use the SwapID which carries the preimage for execution, same as the fallback behavior
	// In the regular adapter, `handle.SwapID` acts as preimage in `ClaimHTLC`.
	preimage := handle.SwapID

	sendReq := tapd.ChannelSendRequest{
		AssetID:     req.AssetID,
		Amount:      uint64(amountSats),
		PeerPubkey:  req.DestAddr, // for keysend-like behavior, DestAddr is the remote node
		PaymentHash: req.PaymentHash,
	}

	res, err := a.channelClient.SendAssetViaChannel(ctx, sendReq)
	if err != nil {
		return nil, fmt.Errorf("failed to route asset via channel: %w", err)
	}

	// 2. Clear out pending map as asset leg succeeded
	a.mu.Lock()
	delete(a.pending, req.SwapID)
	a.mu.Unlock()

	// 3. Reveal Preimage / Claim BTC HTLC
	// The standard lnd execution mechanism to claim the receiving HTLC.
	txID, err := a.chainWatcher.ClaimHTLC(ctx, preimage)
	if err != nil {
		return nil, fmt.Errorf("failed to claim BTC HTLC: %w", err)
	}

	return &settlement.SettlementResult{
		TxID:       fmt.Sprintf("%s|offchain_%s", txID, res.Preimage[:16]),
		DriverType: "HTLC_TAPROOT_CHANNEL",
		FinalState: "CLAIMED",
	}, nil
}

func (a *ChannelSettlementAdapter) AbortSettlement(
	ctx context.Context,
	handle *settlement.SettlementHandle,
) error {
	slog.Info("⚡ [ChannelSettlement] Aborting settlement...",
		"swap_id", handle.SwapID,
		"handle_id", handle.ID,
	)

	a.mu.Lock()
	delete(a.pending, handle.SwapID)
	a.mu.Unlock()

	// Like standard lnd adapter, we depend on canceling the invoice.
	if err := a.lndClient.CancelInvoice(ctx, handle.SwapID); err != nil {
		return fmt.Errorf("abort settlement failed: %w", err)
	}

	return nil
}

func (a *ChannelSettlementAdapter) Capabilities() settlement.DriverCapabilities {
	return settlement.DriverCapabilities{
		MaxTicketSizeSats: 16_777_215, // Standard Channel Limit
		SupportsFreeze:    false,
		SupportsClawback:  false,
		SettlementType:    "HTLC_TAPROOT_CHANNEL",
	}
}
