package settlement

import "context"

// ChainMonitor defines the interface for monitoring blockchain events.
type ChainMonitor interface {
	// WaitForConfirmations blocks until the transaction has at least minConfs confirmations.
	// It returns nil if confirmed, or error if the context is canceled or monitoring fails.
	WaitForConfirmations(ctx context.Context, txid string, minConfs int) error
}
