package lnd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/domain"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

// invoiceClient is the minimal interface LndChainWatcher needs from LND:
// subscribing to a single invoice and settling it with a preimage.
type invoiceClient interface {
	settlement.InvoiceSubscriber
	settlement.InvoiceManager
}

// LndChainWatcher implements settlement.ChainWatcher using LND.
type LndChainWatcher struct {
	client invoiceClient
}

// NewLndChainWatcher creates a new LND-based chain watcher.
func NewLndChainWatcher(client invoiceClient) *LndChainWatcher {
	return &LndChainWatcher{
		client: client,
	}
}

// DetectHTLC waits for the invoice to reach the ACCEPTED state (Held).
func (w *LndChainWatcher) DetectHTLC(ctx context.Context, paymentHash string) (*domain.HTLC, error) {
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

			switch update.State {
			case settlement.InvoiceStateAccepted:
				slog.Info("⚡ [LND] Invoice ACCEPTED (Locked)", "hash", paymentHash, "amt", update.Amt)
				return &domain.HTLC{
					Hash:       paymentHash,
					Amount:     update.Amt,
					Status:     domain.HTLCStatusConfirmed,
					DetectedAt: time.Now(),
				}, nil

			case settlement.InvoiceStateSettled:
				slog.Info("⚡ [LND] Invoice already SETTLED", "hash", paymentHash)
				return &domain.HTLC{
					Hash:   paymentHash,
					Amount: update.Amt,
					Status: domain.HTLCStatusClaimed,
				}, nil

			case settlement.InvoiceStateCanceled:
				return nil, fmt.Errorf("invoice canceled")
			}
			// InvoiceStateOpen: continue waiting
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
