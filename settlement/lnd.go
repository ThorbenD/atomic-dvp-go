package settlement

import (
	"context"
)

// NodeInfo contains basic information about a Lightning Node.
type NodeInfo struct {
	Pubkey  string
	Alias   string
	Network string
	Synced  bool
}

// InvoiceState represents the lifecycle state of a Lightning invoice.
// Using a typed constant avoids magic string comparisons across the codebase.
type InvoiceState string

const (
	InvoiceStateOpen     InvoiceState = "OPEN"
	InvoiceStateSettled  InvoiceState = "SETTLED"
	InvoiceStateCanceled InvoiceState = "CANCELED"
	InvoiceStateAccepted InvoiceState = "ACCEPTED"
)

// InvoiceUpdate carries a snapshot of an invoice's current state.
type InvoiceUpdate struct {
	Hash  string
	State InvoiceState
	Amt   uint64
}

// NodeInfoProvider returns basic node metadata.
type NodeInfoProvider interface {
	GetInfo(ctx context.Context) (*NodeInfo, error)
}

// InvoiceManager handles the lifecycle of hold invoices.
type InvoiceManager interface {
	AddHoldInvoice(ctx context.Context, memo string, hash string, val uint64) (string, uint64, error)
	SettleInvoice(ctx context.Context, preimage string) error
	CancelInvoice(ctx context.Context, hash string) error
}

// InvoiceSubscriber subscribes to invoice state change streams.
type InvoiceSubscriber interface {
	SubscribeSingleInvoice(ctx context.Context, hash string) (<-chan *InvoiceUpdate, <-chan error, error)
	SubscribeInvoices(ctx context.Context) (<-chan *InvoiceUpdate, <-chan error, error)
}

// HTLCInterceptor intercepts and resolves forwarded HTLCs.
type HTLCInterceptor interface {
	StartInterceptor(ctx context.Context, handler func(HtlcPacket) HtlcResolution) error
}

// PSBTOperator handles PSBT-based on-chain Bitcoin operations.
type PSBTOperator interface {
	FundPsbt(ctx context.Context, outputs map[string]uint64) (string, int32, error)
	SignPsbt(ctx context.Context, packet string) (string, bool, error)
	PublishTransaction(ctx context.Context, txHex string) error
}

// ChainConfirmer waits for on-chain transaction confirmations.
type ChainConfirmer interface {
	WaitForConfirmations(ctx context.Context, txid string, numConfs uint32) error
}

// LightningClient composes all LND sub-interfaces into a single convenience interface.
// Prefer the focused sub-interfaces (InvoiceManager, InvoiceSubscriber, etc.) when a
// component only needs a subset of operations — this enforces Interface Segregation.
type LightningClient interface {
	NodeInfoProvider
	InvoiceManager
	InvoiceSubscriber
	HTLCInterceptor
	PSBTOperator
	ChainConfirmer
}

type HtlcPacket struct {
	IncomingCircuitKey CircuitKey
	PaymentHash        string
	IncomingAmount     uint64
	OutgoingAmount     uint64
}

type CircuitKey struct {
	ChanID uint64
	HtlcID uint64
}

type HtlcResolution int

const (
	ResolutionResume HtlcResolution = iota
	ResolutionFail
)
