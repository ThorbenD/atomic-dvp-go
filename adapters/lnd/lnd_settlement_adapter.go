package lnd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

// LndSettlementAdapter implements settlement.SettlementDriver using the existing
// LND (ChainWatcher) and Lightning (LightningClient) infrastructure.
// This is the "Lightning/HTLC" driver — the first concrete implementation
// of the chain-agnostic settlement interface.
type LndSettlementAdapter struct {
	chainWatcher settlement.ChainWatcher
	lnd          settlement.InvoiceManager // only CancelInvoice is needed here
}

// NewLndSettlementAdapter composes existing LND components into the generic
// SettlementDriver interface. The AssetService/AssetSender is NOT included here
// because asset sending is handled separately in the RFQ/resumeSwap flow.
func NewLndSettlementAdapter(
	chainWatcher settlement.ChainWatcher,
	lnd settlement.InvoiceManager,
) *LndSettlementAdapter {
	return &LndSettlementAdapter{
		chainWatcher: chainWatcher,
		lnd:          lnd,
	}
}

// PrepareSettlement waits for an incoming HTLC (Lightning payment) matching
// the given payment hash. This is a BLOCKING call — it returns when the
// deposit is detected or the context expires.
func (a *LndSettlementAdapter) PrepareSettlement(ctx context.Context, req settlement.SettlementRequest) (*settlement.SettlementHandle, error) {
	slog.Info("⚡ [LndSettlement] Preparing settlement (waiting for HTLC)...",
		"swap_id", req.SwapID,
		"payment_hash", req.PaymentHash,
	)

	htlc, err := a.chainWatcher.DetectHTLC(ctx, req.PaymentHash)
	if err != nil {
		return nil, fmt.Errorf("prepare settlement failed: %w", err)
	}

	slog.Info("⚡ [LndSettlement] HTLC detected, settlement prepared",
		"swap_id", req.SwapID,
		"amount_sats", htlc.Amount,
	)

	return &settlement.SettlementHandle{
		ID:             fmt.Sprintf("htlc_%s", req.PaymentHash[:16]),
		SwapID:         req.SwapID,
		DriverType:     "HTLC_LIGHTNING",
		PreparedAt:     time.Now(),
		DepositAmtSats: htlc.Amount,
		Preimage:       req.Preimage,
		PaymentHash:    req.PaymentHash,
	}, nil
}

// ExecuteSettlement claims the HTLC by revealing the preimage.
// This is the "sweep" step that finalizes the DvP.
func (a *LndSettlementAdapter) ExecuteSettlement(ctx context.Context, handle *settlement.SettlementHandle) (*settlement.SettlementResult, error) {
	slog.Info("⚡ [LndSettlement] Executing settlement (claiming HTLC)...",
		"swap_id", handle.SwapID,
		"handle_id", handle.ID,
	)

	txID, err := a.chainWatcher.ClaimHTLC(ctx, handle.Preimage)
	if err != nil {
		return nil, fmt.Errorf("execute settlement failed: %w", err)
	}

	return &settlement.SettlementResult{
		TxID:       txID,
		DriverType: "HTLC_LIGHTNING",
		FinalState: "CLAIMED",
	}, nil
}

// AbortSettlement cancels the hold invoice, releasing the locked funds back
// to the payer.
func (a *LndSettlementAdapter) AbortSettlement(ctx context.Context, handle *settlement.SettlementHandle) error {
	slog.Info("⚡ [LndSettlement] Aborting settlement (canceling invoice)...",
		"swap_id", handle.SwapID,
		"handle_id", handle.ID,
	)

	if err := a.lnd.CancelInvoice(ctx, handle.PaymentHash); err != nil {
		return fmt.Errorf("abort settlement failed: %w", err)
	}

	return nil
}

// Capabilities returns the operational constraints of the Lightning/HTLC driver.
func (a *LndSettlementAdapter) Capabilities() settlement.DriverCapabilities {
	return settlement.DriverCapabilities{
		MaxTicketSizeSats: 1_000_000, // ~0.01 BTC — Lightning routing limit for reliable delivery
		SupportsFreeze:    false,     // Lightning has no native freeze mechanism
		SupportsClawback:  false,     // Lightning has no native clawback
		SettlementType:    "HTLC_LIGHTNING",
	}
}
