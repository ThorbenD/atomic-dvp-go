package mock

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/liquidityos/atomic-dvp-go/settlement"
)

// MockSettlementAdapter implements settlement.SettlementDriver for testing and demos.
// It wraps a MockChainWatcher and provides controllable settlement behavior.
type MockSettlementAdapter struct {
	mu           sync.Mutex
	chainWatcher *MockChainWatcher
	capabilities settlement.DriverCapabilities
}

// NewMockSettlementAdapter creates a test SettlementDriver.
func NewMockSettlementAdapter(chainWatcher *MockChainWatcher) *MockSettlementAdapter {
	return &MockSettlementAdapter{
		chainWatcher: chainWatcher,
		capabilities: settlement.DriverCapabilities{
			MaxTicketSizeSats: 1_000_000,
			SupportsFreeze:    false,
			SupportsClawback:  false,
			SettlementType:    "HTLC_LIGHTNING",
		},
	}
}

// PrepareSettlement blocks until the MockChainWatcher detects an HTLC.
func (m *MockSettlementAdapter) PrepareSettlement(ctx context.Context, req settlement.SettlementRequest) (*settlement.SettlementHandle, error) {
	slog.Info("🧪 [MockSettlement] Preparing settlement...", "swap_id", req.SwapID)

	htlc, err := m.chainWatcher.DetectHTLC(ctx, req.PaymentHash)
	if err != nil {
		return nil, fmt.Errorf("mock prepare failed: %w", err)
	}

	return &settlement.SettlementHandle{
		ID:             fmt.Sprintf("mock_htlc_%s", req.PaymentHash[:8]),
		SwapID:         req.SwapID,
		DriverType:     "HTLC_LIGHTNING",
		PreparedAt:     time.Now(),
		DepositAmtSats: htlc.Amount,
	}, nil
}

// ExecuteSettlement claims the HTLC via the mock chain watcher.
func (m *MockSettlementAdapter) ExecuteSettlement(ctx context.Context, handle *settlement.SettlementHandle) (*settlement.SettlementResult, error) {
	slog.Info("🧪 [MockSettlement] Executing settlement...", "swap_id", handle.SwapID)

	// Use SwapID as preimage carrier (same pattern as LND adapter)
	txID, err := m.chainWatcher.ClaimHTLC(ctx, handle.SwapID)
	if err != nil {
		return nil, fmt.Errorf("mock execute failed: %w", err)
	}

	return &settlement.SettlementResult{
		TxID:       txID,
		DriverType: "HTLC_LIGHTNING",
		FinalState: "CLAIMED",
	}, nil
}

// AbortSettlement is a no-op in the mock (no real invoice to cancel).
func (m *MockSettlementAdapter) AbortSettlement(ctx context.Context, handle *settlement.SettlementHandle) error {
	slog.Info("🧪 [MockSettlement] Aborting settlement...", "swap_id", handle.SwapID)
	return nil
}

// Capabilities returns the mock driver's capabilities.
func (m *MockSettlementAdapter) Capabilities() settlement.DriverCapabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.capabilities
}

// SetCapabilities allows tests to modify capabilities (e.g., to test ticket-size rejection).
func (m *MockSettlementAdapter) SetCapabilities(caps settlement.DriverCapabilities) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capabilities = caps
}

// SimulateDeposit is a test helper that injects a simulated HTLC into the
// underlying MockChainWatcher.
func (m *MockSettlementAdapter) SimulateDeposit(paymentHash string, amountSats uint64) {
	m.chainWatcher.SimulateIncomingHTLC(paymentHash, amountSats)
}
