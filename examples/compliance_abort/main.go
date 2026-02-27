// Demonstrates atomic abort on a compliance event.
// No orphaned trades. Funds automatically return to sender.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ThorbenD/atomic-dvp-go/adapters/mock"
	"github.com/ThorbenD/atomic-dvp-go/settlement"
)

func main() {
	ctx := context.Background()

	// Initialize mock chain watcher and driver
	chainWatcher := mock.NewMockChainWatcher()
	driver := mock.NewMockSettlementAdapter(chainWatcher)

	// Simulate an incoming HTLC on the network so PrepareSettlement can detect it
	paymentHash := "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f"
	swapID := "test-swap-abort-id"
	go func() {
		time.Sleep(1 * time.Second)
		driver.SimulateDeposit(paymentHash, 50_000)
	}()

	fmt.Println("Waiting for HTLC settlement preparation...")
	handle, err := driver.PrepareSettlement(ctx, settlement.SettlementRequest{
		SwapID:      swapID,
		PaymentHash: paymentHash,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Prepared HTLC handle: %s with deposit %d sats\n", handle.ID, handle.DepositAmtSats)

	// Simulate compliance flag (e.g. AML hit)
	fmt.Println("Compliance event triggered – aborting settlement atomically.")
	if err := driver.AbortSettlement(ctx, handle); err != nil {
		panic(err)
	}

	fmt.Println("Abort successful. No orphaned trade. Funds returned.")
}
