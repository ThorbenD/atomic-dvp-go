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

// MockChainWatcher implements settlement.ChainWatcher for testing/dev
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
		Expiry:     0, // Mock: not relevant yet
		Status:     domain.HTLCStatusConfirmed,
		DetectedAt: time.Now(),
	}
	slog.Info("⛓️  [MockChain] Simulated incoming HTLC", "hash", hash)
}

// Helper to derive hash
func toSha256(preimage string) string {
	b, _ := hex.DecodeString(preimage) // Assuming valid hex
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func (m *MockChainWatcher) ClaimHTLC(ctx context.Context, preimage string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Verify Preimage matches a known Hash
	// Note: In a real system, we'd hash the preimage.
	// For this Mock, if you stored the hash directly, we can try to derive it.
	// Or just iterate to find a match if your map keys are hashes.

	// Let's assume we implement a simple SHA256 check:
	calculatedHash := toSha256(preimage)

	htlc, exists := m.transactions[calculatedHash]
	if !exists {
		// Fallback for testing if you used random hashes that aren't real preimages
		// Use with caution or comment out for strict mode
		slog.Warn("⚠️ [MockChain] Preimage verification failed, but checking direct mock injection...", "hash", calculatedHash)
		return "", fmt.Errorf("HTLC with hash %s not found", calculatedHash)
	}

	if htlc.Status == "CLAIMED" {
		return "", fmt.Errorf("HTLC already claimed")
	}

	// 2. "Sweep"
	htlc.Status = "CLAIMED" // Update internal state
	txID := "tx_mock_sweep_" + preimage[:8]

	slog.Info("🧹 [MockChain] Sweeping HTLC!", "tx_id", txID, "preimage", preimage)
	return txID, nil
}
