// Demonstrates atomic DvP via Lightning Channel:
// Both BTC payment and Asset transfer as off-chain HTLCs.
// Settlement time: milliseconds.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/adapters/mock"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
	"github.com/shopspring/decimal"
)

func main() {
	ctx := context.Background()

	// Initialize mock chain watcher and driver
	chainWatcher := mock.NewMockChainWatcher()
	driver := mock.NewMockSettlementAdapter(chainWatcher)
	driver.SetCapabilities(settlement.DriverCapabilities{
		MaxTicketSizeSats: 16_777_215,
		SupportsFreeze:    false,
		SupportsClawback:  false,
		SettlementType:    "HTLC_TAPROOT_CHANNEL",
	})

	// Generate matched preimage and hash (must be valid hex for mock chain watcher)
	preimage := "0000000000000000000000000000000000000000000000000000000000000001"
	preimageBytes, _ := hex.DecodeString(preimage)
	hashBytes := sha256.Sum256(preimageBytes)
	paymentHash := hex.EncodeToString(hashBytes[:])

	// SwapID is used as the preimage carrier in the adapter execution
	swapID := preimage

	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulated channel HTLC arrival (very fast!)
		driver.SimulateDeposit(paymentHash, 50_000)
	}()

	fmt.Println("Waiting for Channel HTLC settlement preparation...")
	handle, err := driver.PrepareSettlement(ctx, settlement.SettlementRequest{
		SwapID:      swapID,
		PaymentHash: paymentHash,
		AssetID:     "deadbeef",
		AssetAmount: decimal.NewFromFloat(10.5),
		DestAddr:    "021c1075c2e173ea3be32cfaeec528b1e4c70d47d0de0ba88a381cd29cb01e4a19",
	})
	if err != nil {
		panic(err)
	}
	prepTime := time.Since(start)
	fmt.Printf("Prepared HTLC handle: %s with deposit %d sats in %v\n", handle.ID, handle.DepositAmtSats, prepTime)

	fmt.Println("Executing channel settlement (Routing Asset + Claiming HTLC)...")
	result, err := driver.ExecuteSettlement(ctx, handle)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Settlement complete! TxID: %s\n", result.TxID)

	// Zeigt den Unterschied zu On-Chain:
	// OnChain: ~10 Minuten
	// Channel: ~500ms
	totalTime := time.Since(start)
	fmt.Printf("\n--- Timing Analysis ---\n")
	fmt.Printf("Total settlement time: %v\n", totalTime)
	fmt.Println("Notice how both legs completed in milliseconds via off-chain HTLCs.")
}
