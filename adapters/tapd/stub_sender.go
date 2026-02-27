package tapd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shopspring/decimal"
)

// StubSender simulates Taproot asset transfers.
type StubSender struct{}

// NewStubSender creates a new StubSender instance.
func NewStubSender() *StubSender {
	return &StubSender{}
}

// SendAsset simulates sending an asset by logging, waiting, and returning a fake TXID.
func (s *StubSender) SendAsset(ctx context.Context, assetID string, amount decimal.Decimal, destAddr string) (string, error) {
	log.Printf("[TaprootStub] Sending %s of asset %s to %s...", amount.String(), assetID, destAddr)

	// Simulate network latency
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	txID := fmt.Sprintf("tx_sim_%d", time.Now().UnixNano())
	log.Printf("[TaprootStub] Asset sent! Fake TXID: %s", txID)

	return txID, nil
}
