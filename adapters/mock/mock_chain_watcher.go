package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/domain"
)

// MockChainWatcher implements settlement.ChainWatcher for testing/dev.
type MockChainWatcher struct {
	mu           sync.RWMutex
	transactions map[string]*domain.HTLC
}

func NewMockChainWatcher() *MockChainWatcher {
	return &MockChainWatcher{
		transactions: make(map[string]*domain.HTLC),
	}
}

// DetectHTLC polls the internal map for the presence of a specific hash
func (m *MockChainWatcher) DetectHTLC(ctx context.Context, paymentHash string) (*domain.HTLC, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	slog.Info("⛓️  [MockChain] Watching for HTLC", "hash", paymentHash)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			m.mu.RLock()
			htlc, exists := m.transactions[paymentHash]
			m.mu.RUnlock()

			if exists {
				slog.Info("⛓️  [MockChain] HTLC Detected!", "hash", paymentHash, "amount", htlc.Amount)
				return htlc, nil
			}
		}
	}
}

// SimulateIncomingHTLC is a helper to manually trigger a "blockchain event"
func (m *MockChainWatcher) SimulateIncomingHTLC(hash string, amount uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transactions[hash] = &domain.HTLC{
		Hash:       hash,
		Amount:     amount,
		Expiry:     0,
		Status:     domain.HTLCStatusConfirmed,
		DetectedAt: time.Now(),
	}
	slog.Info("⛓️  [MockChain] Simulated incoming HTLC", "hash", hash)
}

// toSha256 derives a SHA256 hash from a hex-encoded preimage.
func toSha256(preimage string) (string, error) {
	b, err := hex.DecodeString(preimage)
	if err != nil {
		return "", fmt.Errorf("invalid preimage hex: %w", err)
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

func (m *MockChainWatcher) ClaimHTLC(ctx context.Context, preimage string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	calculatedHash, err := toSha256(preimage)
	if err != nil {
		return "", fmt.Errorf("claim HTLC: %w", err)
	}

	htlc, exists := m.transactions[calculatedHash]
	if !exists {
		return "", fmt.Errorf("HTLC with hash %s not found", calculatedHash)
	}

	if htlc.Status == domain.HTLCStatusClaimed {
		return "", fmt.Errorf("HTLC already claimed")
	}

	htlc.Status = domain.HTLCStatusClaimed
	txID := "tx_mock_sweep_" + preimage[:8]

	slog.Info("🧹 [MockChain] Sweeping HTLC!", "tx_id", txID)
	return txID, nil
}
