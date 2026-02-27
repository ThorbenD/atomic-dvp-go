package lnd

import (
	"context"
	"fmt"
	"log"

	"github.com/liquidityos/atomic-dvp-go/settlement"
)

// LndChainMonitor implements the ChainMonitor interface using LND.
type LndChainMonitor struct {
	lndClient settlement.LightningClient
}

// NewLndChainMonitor creates a new instance of LndChainMonitor.
func NewLndChainMonitor(lndClient settlement.LightningClient) *LndChainMonitor {
	return &LndChainMonitor{
		lndClient: lndClient,
	}
}

// WaitForConfirmations blocks until the transaction has at least minConfs confirmations.
func (m *LndChainMonitor) WaitForConfirmations(ctx context.Context, txid string, minConfs int) error {
	log.Printf("⛓️  ChainMonitor: Waiting for %d confirmations for TXID %s...", minConfs, txid)

	// Delegate to LND client's WaitForConfirmations (which uses ChainNotifier)
	if err := m.lndClient.WaitForConfirmations(ctx, txid, uint32(minConfs)); err != nil {
		return fmt.Errorf("wait for confirmations failed: %w", err)
	}

	log.Printf("✅ ChainMonitor: TXID %s confirmed!", txid)
	return nil
}
