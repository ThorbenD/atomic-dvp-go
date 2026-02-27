package settlement

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// SettlementDriver is the chain-agnostic port for executing Delivery vs. Payment (DvP).
// Implementations include LND/Lightning (HTLC), future EVM (ERC-3643), or Liquid/L-BTC.
// The SwapOrchestrator talks ONLY to this interface — never to LND, tapd, or any chain directly.
type SettlementDriver interface {
	// PrepareSettlement waits for or verifies that the counterparty has locked funds.
	// For Lightning: this means detecting an incoming HTLC.
	// For EVM: this could mean verifying an escrow contract deposit.
	// This call MAY block until the deposit is detected or the context expires.
	PrepareSettlement(ctx context.Context, req SettlementRequest) (*SettlementHandle, error)

	// ExecuteSettlement claims/sweeps the locked funds, finalizing the DvP.
	// For Lightning: this reveals the preimage to claim the HTLC.
	// For EVM: this could call a smart contract's release function.
	ExecuteSettlement(ctx context.Context, handle *SettlementHandle) (*SettlementResult, error)

	// AbortSettlement rolls back a prepared (but not yet executed) settlement.
	// For Lightning: this cancels the hold invoice.
	// For EVM: this could trigger a refund from escrow.
	AbortSettlement(ctx context.Context, handle *SettlementHandle) error

	// Capabilities returns the operational constraints of this driver.
	// The orchestrator uses this for ticket-size validation and feature detection.
	Capabilities() DriverCapabilities
}

// SettlementRequest contains the parameters needed to prepare a settlement.
type SettlementRequest struct {
	SwapID       string
	PaymentHash  string
	Preimage     string
	AssetID      string
	AssetAmount  decimal.Decimal
	DestAddr     string
	FiatAmount   decimal.Decimal
	FiatCurrency string
}

// SettlementHandle is an opaque reference returned by PrepareSettlement.
// It carries driver-internal state needed to execute or abort the settlement.
type SettlementHandle struct {
	ID             string    // Opaque driver-internal handle ID
	SwapID         string    // Reference back to the swap
	DriverType     string    // e.g. "HTLC_LIGHTNING", "ERC3643", "LIQUID"
	PreparedAt     time.Time // When the deposit was detected/locked
	DepositAmtSats uint64    // Amount detected (for Lightning: satoshis)
}

// SettlementResult is returned by ExecuteSettlement upon successful claim.
type SettlementResult struct {
	TxID       string // Transaction ID of the claim/sweep
	DriverType string // Driver that executed the settlement
	FinalState string // e.g. "CLAIMED", "SETTLED"
}

// DriverCapabilities describes the operational constraints of a SettlementDriver.
// The orchestrator uses these for routing decisions and ticket-size validation.
type DriverCapabilities struct {
	MaxTicketSizeSats uint64 // Maximum transaction size this driver can handle (in sats)
	SupportsFreeze    bool   // Can this driver freeze an asset? (eWpG requirement)
	SupportsClawback  bool   // Can this driver claw back an asset? (eWpG requirement)
	SettlementType    string // Human-readable type: "HTLC_LIGHTNING", "ERC3643", "LIQUID"
}
