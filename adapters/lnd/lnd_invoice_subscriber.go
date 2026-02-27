package lnd

import (
	"context"
	"log"
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
// extracted from the SwapOrchestrator in PI 28 to achieve Inversion of Control.
//
// In a future chain-agnostic world, there would be equivalent subscribers for
// EVM events (via WebSocket), Liquid block notifications, etc. — all calling
// the same DepositHandler.OnDepositDetected().
type LndInvoiceSubscriber struct {
	lnd     settlement.LightningClient
	handler DepositHandler
}

// NewLndInvoiceSubscriber creates a subscriber that bridges LND events to the orchestrator.
func NewLndInvoiceSubscriber(lnd settlement.LightningClient, handler DepositHandler) *LndInvoiceSubscriber {
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
	log.Println("🔌 [LndInvoiceSubscriber] Connecting to LND Invoice Stream...")

	for {
		// Check context before reconnecting
		select {
		case <-ctx.Done():
			log.Println("🔌 [LndInvoiceSubscriber] Context cancelled, stopping.")
			return
		default:
		}

		updates, errors, err := s.lnd.SubscribeInvoices(ctx)
		if err != nil {
			log.Printf("❌ [LndInvoiceSubscriber] Failed to subscribe: %v. Retrying in 5s...", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		log.Println("✅ [LndInvoiceSubscriber] Listening for Invoices...")

	streamLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errors:
				if !ok {
					log.Println("⚠️ [LndInvoiceSubscriber] Error stream closed.")
					break streamLoop
				}
				log.Printf("❌ [LndInvoiceSubscriber] Stream error: %v. Reconnecting...", err)
				break streamLoop
			case update, ok := <-updates:
				if !ok {
					log.Println("⚠️ [LndInvoiceSubscriber] Update stream closed. Reconnecting...")
					break streamLoop
				}

				// We care about ACCEPTED (Hold Invoice Paid)
				if update.State == "ACCEPTED" {
					go func(hash string) {
						if err := s.handler.OnDepositDetected(context.Background(), hash); err != nil {
							log.Printf("❌ [LndInvoiceSubscriber] OnDepositDetected failed for %s: %v", hash, err)
						}
					}(update.Hash)
				}
			}
		}

		// Backoff before reconnecting
		time.Sleep(1 * time.Second)
	}
}
