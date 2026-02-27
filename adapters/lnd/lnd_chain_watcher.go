package lnd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/liquidityos/atomic-dvp-go/domain"
	"github.com/liquidityos/atomic-dvp-go/settlement"
)

// LndChainWatcher implements settlement.ChainWatcher using LND.
type LndChainWatcher struct {
	client settlement.LightningClient
}

// NewLndChainWatcher creates a new LND-based chain watcher.
func NewLndChainWatcher(client settlement.LightningClient) *LndChainWatcher {
	return &LndChainWatcher{
		client: client,
	}
}

// DetectHTLC waits for the invoice to reach the ACCEPTED state (Held).
func (w *LndChainWatcher) DetectHTLC(ctx context.Context, paymentHash string) (*domain.HTLC, error) {
	// Subscribe to invoice updates
	updateChan, errChan, err := w.client.SubscribeSingleInvoice(ctx, paymentHash)
	if err != nil {
		return nil, fmt.Errorf("subscribe invoice failed: %w", err)
	}

	slog.Info("⚡ [LND] Watching invoice...", "hash", paymentHash)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-errChan:
			return nil, fmt.Errorf("stream error: %w", err)
		case update, ok := <-updateChan:
			if !ok {
				return nil, fmt.Errorf("stream closed unexpectedly")
			}

			// Map LND State to Domain State
			switch update.State {
			case "ACCEPTED":
				slog.Info("⚡ [LND] Invoice ACCEPTED (Locked)", "hash", paymentHash, "amt", update.Amt)
				return &domain.HTLC{
					Hash:       paymentHash,
					Amount:     update.Amt, // Satoshis
					Status:     domain.HTLCStatusConfirmed,
					DetectedAt: time.Now(), // Assuming we want current time, or we could add it to InvoiceUpdate if needed
				}, nil

			case "SETTLED":
				slog.Info("⚡ [LND] Invoice already SETTLED", "hash", paymentHash)
				return &domain.HTLC{
					Hash:   paymentHash,
					Amount: update.Amt,
					Status: domain.HTLCStatusClaimed,
				}, nil

			case "CANCELED":
				return nil, fmt.Errorf("invoice canceled")
			}
			// If OPEN, continue loop
		}
	}
}

// ClaimHTLC settles the invoice using the preimage.
func (w *LndChainWatcher) ClaimHTLC(ctx context.Context, preimage string) (string, error) {
	slog.Info("⚡ [LND] Settling Invoice...", "preimage_len", len(preimage))

	if err := w.client.SettleInvoice(ctx, preimage); err != nil {
		return "", fmt.Errorf("lnd settle failed: %w", err)
	}

	return "off-chain-settled", nil
}
