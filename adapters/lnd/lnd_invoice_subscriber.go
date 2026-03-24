package lnd

import (
	"context"
	"log/slog"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

// DepositHandler is the interface that the subscriber calls when a deposit is detected.
// This decouples the subscriber from the orchestrator — any struct with OnDepositDetected works.
type DepositHandler interface {
	OnDepositDetected(ctx context.Context, paymentHash string) error
}

// LndInvoiceSubscriber listens to LND invoice updates and triggers the orchestrator
// when an invoice is accepted (held). This is the adapter-layer component that was
// extracted from the SwapOrchestrator to achieve Inversion of Control.
//
// In a future chain-agnostic world, there would be equivalent subscribers for
// EVM events (via WebSocket), Liquid block notifications, etc. — all calling
// the same DepositHandler.OnDepositDetected().
type LndInvoiceSubscriber struct {
	lnd     settlement.InvoiceSubscriber
	handler DepositHandler
}

// NewLndInvoiceSubscriber creates a subscriber that bridges LND events to the orchestrator.
func NewLndInvoiceSubscriber(lnd settlement.InvoiceSubscriber, handler DepositHandler) *LndInvoiceSubscriber {
	return &LndInvoiceSubscriber{
		lnd:     lnd,
		handler: handler,
	}
}

// Start begins listening for LND invoice updates in a background goroutine.
// When an invoice reaches the ACCEPTED state (hold invoice paid), it calls
// handler.OnDepositDetected(). This method is non-blocking.
func (s *LndInvoiceSubscriber) Start(ctx context.Context) {
	go s.subscribeLoop(ctx)
}

func (s *LndInvoiceSubscriber) subscribeLoop(ctx context.Context) {
	slog.Info("🔌 [LndInvoiceSubscriber] Connecting to LND Invoice Stream...")

	for {
		select {
		case <-ctx.Done():
			slog.Info("🔌 [LndInvoiceSubscriber] Context cancelled, stopping.")
			return
		default:
		}

		updates, errors, err := s.lnd.SubscribeInvoices(ctx)
		if err != nil {
			slog.Error("❌ [LndInvoiceSubscriber] Failed to subscribe, retrying in 5s...", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		slog.Info("✅ [LndInvoiceSubscriber] Listening for Invoices...")

	streamLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errors:
				if !ok {
					slog.Warn("⚠️ [LndInvoiceSubscriber] Error stream closed.")
					break streamLoop
				}
				slog.Error("❌ [LndInvoiceSubscriber] Stream error, reconnecting...", "err", err)
				break streamLoop
			case update, ok := <-updates:
				if !ok {
					slog.Warn("⚠️ [LndInvoiceSubscriber] Update stream closed. Reconnecting...")
					break streamLoop
				}

				if update.State == settlement.InvoiceStateAccepted {
					go func(hash string) {
// Use a timeout so a slow or stuck handler cannot leak this goroutine.
						callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
						defer cancel()
						if err := s.handler.OnDepositDetected(callCtx, hash); err != nil {
							slog.Error("❌ [LndInvoiceSubscriber] OnDepositDetected failed",
								"hash", hash, "err", err)
						}
					}(update.Hash)
				}
			}
		}

		// Backoff before reconnecting
		time.Sleep(1 * time.Second)
	}
}
