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

// LightningClient defines the interface for interacting with the LND node.
type LightningClient interface {
	GetInfo(ctx context.Context) (*NodeInfo, error)
	AddHoldInvoice(ctx context.Context, memo string, hash string, val uint64) (string, uint64, error)
	SettleInvoice(ctx context.Context, preimage string) error
	CancelInvoice(ctx context.Context, hash string) error
	StartInterceptor(ctx context.Context, handler func(HtlcPacket) HtlcResolution) error
	SubscribeSingleInvoice(ctx context.Context, hash string) (<-chan *InvoiceUpdate, <-chan error, error)
	SubscribeInvoices(ctx context.Context) (<-chan *InvoiceUpdate, <-chan error, error)
	FundPsbt(ctx context.Context, outputs map[string]uint64) (string, int32, error)
	SignPsbt(ctx context.Context, packet string) (string, bool, error)
	PublishTransaction(ctx context.Context, txHex string) error
	WaitForConfirmations(ctx context.Context, txid string, numConfs uint32) error
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

type InvoiceUpdate struct {
	Hash  string
	State string // OPEN, SETTLED, CANCELED, ACCEPTED
	Amt   uint64
}

const (
	ResolutionResume HtlcResolution = iota
	ResolutionFail
)
