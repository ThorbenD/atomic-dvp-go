package settlement

import (
	"context"

	"github.com/ThorbenD/atomic-dvp-go/domain"
)

// ChainWatcher defines the interface for monitoring the blockchain
// for specific events, such as incoming HTLCs.
type ChainWatcher interface {
	// DetectHTLC blocks until an HTLC with the given payment hash is detected
	// or the context is cancelled.
	DetectHTLC(ctx context.Context, paymentHash string) (*domain.HTLC, error)

	// ClaimHTLC reveals the preimage to sweep the funds.
	// Returns the Transaction ID of the sweep.
	ClaimHTLC(ctx context.Context, preimage string) (txID string, err error)
}
