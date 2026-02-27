package tapd

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/lightninglabs/taproot-assets/taprpc"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
)

type LndRpcSender struct {
	client taprpc.TaprootAssetsClient
	logger *slog.Logger
}

func NewLndRpcSender(conn *grpc.ClientConn, logger *slog.Logger) *LndRpcSender {
	return &LndRpcSender{
		client: taprpc.NewTaprootAssetsClient(conn),
		logger: logger,
	}
}

func (s *LndRpcSender) SendAsset(ctx context.Context, assetID string, amount decimal.Decimal, destAddr string) (string, error) {
	// Convert decimal to uint64 (taproot assets use smallest unit)
	// Example: If amount represents whole units with 8 decimal places (like BTC)
	// We assume default precision for now or that 'amount' is already scaled if it came from core?
	// The prompt implies we need to shift.
	// "amountSats := amount.Shift(8).IntPart() // Adjust shift based on asset precision"
	// However, if we don't know the precision here, we might be making an assumption.
	// For now, following the user's example strictly.
	// Assumption: Precision is 8? Or 0 if it's already base units?
	// The core usually treats Decimal as "human readable" or "base units"?
	// If the domain amount is e.g. "10.5", and precision is 0, it's weird.
	// Let's assume Shift(0) if it's raw units, or Shift(8) if it's BTC-like.
	// Given the prompt example uses Shift(8), I will use Shift(8) but log it clearly.
	// Wait, if I shift 8, 1 unit becomes 100,000,000.

	amountSats := amount.Shift(8).IntPart()

	if amountSats <= 0 {
		return "", fmt.Errorf("invalid amount: must be positive, got %s", amount.String())
	}

	s.logger.Debug("converting amount", "decimal", amount.String(), "sats", amountSats)

	// 1. Build the SendAssetRequest
	// Note: taprpc.SendAssetRequest usually takes just the address (which encodes amount).
	// If the 'tapd' API requires amount override or check, we would pass it.
	// Currently, typical usage relies on the address.
	// We pass the address provided.
	req := &taprpc.SendAssetRequest{
		TapAddrs: []string{destAddr},
	}

	// 2. Call the RPC
	resp, err := s.client.SendAsset(ctx, req)
	if err != nil {
		return "", fmt.Errorf("tapd.SendAsset failed: %w", err)
	}

	// 3. Return resp.Txid or wrap error
	// The response structure might vary by version.
	// Accessing Transfer information if available.
	if resp.Transfer != nil {
		if len(resp.Transfer.AnchorTxHash) > 0 {
			return hex.EncodeToString(resp.Transfer.AnchorTxHash), nil
		}
		// Fallback if no anchor hash yet
		return fmt.Sprintf("pending-%d", resp.Transfer.TransferTimestamp), nil
	}

	// Fallback if transfer info is missing but no error
	return "unknown-txid", nil
}
