package lnd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

// LndChainMonitor implements the ChainMonitor interface using LND.
type LndChainMonitor struct {
	lndClient settlement.ChainConfirmer
}

// NewLndChainMonitor creates a new instance of LndChainMonitor.
func NewLndChainMonitor(lndClient settlement.ChainConfirmer) *LndChainMonitor {
	return &LndChainMonitor{
		lndClient: lndClient,
	}
}

// WaitForConfirmations blocks until the transaction has at least minConfs confirmations.
func (m *LndChainMonitor) WaitForConfirmations(ctx context.Context, txid string, minConfs int) error {
	slog.Info("⛓️  [ChainMonitor] Waiting for confirmations", "txid", txid, "min_confs", minConfs)

	if err := m.lndClient.WaitForConfirmations(ctx, txid, uint32(minConfs)); err != nil {
		return fmt.Errorf("wait for confirmations failed: %w", err)
	}

	slog.Info("✅ [ChainMonitor] Transaction confirmed", "txid", txid)
	return nil
}
