package settlement

import (
	"context"

	"github.com/shopspring/decimal"
)

// AssetSender defines the interface for interacting with an asset daemon
// (like tapd) to transfer assets on-chain.
type AssetSender interface {
	SendAsset(ctx context.Context, assetID string, amount decimal.Decimal, destAddr string) (txID string, err error)
}
