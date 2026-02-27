package domain

import "time"

type HTLCStatus string

const (
	HTLCStatusPending   HTLCStatus = "PENDING"
	HTLCStatusConfirmed HTLCStatus = "CONFIRMED"
	HTLCStatusClaimed   HTLCStatus = "CLAIMED"
)

// HTLC represents an incoming Hash Time-Locked Contract deposit.
// This is the core primitive used by the SettlementDriver to detect locked funds.
type HTLC struct {
	Hash       string // Payment Hash (hex)
	Amount     uint64 // Amount in Satoshis (or smallest unit)
	Expiry     int64  // CLTV Expiry (Block Height)
	Status     HTLCStatus
	DetectedAt time.Time
}
